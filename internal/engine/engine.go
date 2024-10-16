package engine

import (
	"context"
	b64 "encoding/base64"
	"fmt"
	"net/url"
	"strings"

	v1 "github.com/crossplane/crossplane/apis/pkg/v1"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/pkg/errors"
	"github.com/web-seven/overlock/internal/install"
	"github.com/web-seven/overlock/internal/install/helm"
	"github.com/web-seven/overlock/internal/namespace"
	"gopkg.in/yaml.v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type SecretReconciler struct {
	serverIP string
	client.Client
	context.CancelFunc
}

const (
	RepoUrl                = "https://charts.crossplane.io/stable"
	ChartName              = "crossplane"
	ReleaseName            = "overlock-crossplane"
	Version                = "1.15.2"
	kindClusterRole        = "ClusterRole"
	ProviderConfigName     = "overlock-kubernetes-provider-config"
	helmProviderConfigName = "overlock-helm-provider-config"
	aggregateToAdmin       = "rbac.crossplane.io/aggregate-to-admin"
	trueVal                = "true"
	errParsePackageName    = "package name is not valid"
)

var (
	managedLabels = map[string]string{
		"app.kubernetes.io/managed-by": "overlock",
	}
	initParameters = map[string]any{
		"provider": map[string]any{
			"packages": []string{},
		},
		"configuration": map[string]any{
			"packages": []string{},
		},
		"args": []string{},
	}
)

// Get engine Helm manager
func GetEngine(configClient *rest.Config) (install.Manager, error) {
	repoURL, err := url.Parse(RepoUrl)
	if err != nil {
		return nil, fmt.Errorf("error parsing repository URL: %v", err)
	}
	setWait := helm.InstallerModifierFn(helm.Wait())
	setNamespace := helm.InstallerModifierFn(helm.WithNamespace(namespace.Namespace))
	setUpInstall := helm.InstallerModifierFn(helm.WithUpgradeInstall(true))
	setCreateNs := helm.InstallerModifierFn(helm.WithCreateNamespace(true))
	setReuseValues := helm.InstallerModifierFn(helm.WithReuseValues(true))

	installer, err := helm.NewManager(
		configClient,
		ChartName,
		repoURL,
		ReleaseName,
		setWait,
		setNamespace,
		setUpInstall,
		setCreateNs,
		setReuseValues,
	)

	if err != nil {
		return nil, fmt.Errorf("error creating Helm manager: %v", err)
	}

	return installer, nil
}

// Install engine Helm release
func InstallEngine(ctx context.Context, configClient *rest.Config, params map[string]any, logger *zap.SugaredLogger) error {
	engine, err := GetEngine(configClient)
	if err != nil {
		return err
	}

	if params == nil {
		params = initParameters
	}
	logger.Debug("Upgrade Crossplane release")
	return engine.Upgrade(Version, params)
}

// Check if engine release exists
func IsHelmReleaseFound(configClient *rest.Config) bool {

	installer, err := GetEngine(configClient)
	if err != nil {
		return false
	}
	_, err = installer.GetRelease()
	return err == nil

}

// Lables for engine system resources, mixed with provided
func ManagedLabels(m map[string]string) map[string]string {
	merged := make(map[string]string)
	for k, v := range managedLabels {
		merged[k] = v
	}
	for key, value := range m {
		merged[key] = value
	}
	return merged
}

// Selector for engine system resources, mixed with provided
func ManagedSelector(m map[string]string) string {
	selectors := []string{}
	for k, v := range managedLabels {
		selectors = append(selectors, k+"="+v)
	}
	for key, value := range m {
		selectors = append(selectors, key+"="+value)
	}
	return strings.Join(selectors, ",")
}

// Setup Kubernetes provider which has crossplane admin aggregation role assigned
func SetupPrivilegedKubernetesProvider(ctx context.Context, configClient *rest.Config, logger *zap.SugaredLogger) error {

	pcn := ProviderConfigName

	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pcn,
			Namespace: namespace.Namespace,
		},
	}

	saSec := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pcn,
			Namespace: namespace.Namespace,
			Annotations: map[string]string{
				"kubernetes.io/service-account.name": sa.Name,
			},
		},
		Type: corev1.SecretTypeServiceAccountToken,
	}

	cr := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: pcn,
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{"*", ""},
				Verbs:     []string{"*"},
				Resources: []string{"*"},
			},
		},
	}

	crb := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: pcn,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      rbacv1.ServiceAccountKind,
				Name:      sa.Name,
				Namespace: namespace.Namespace,
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     kindClusterRole,
			Name:     cr.Name,
		},
	}

	scheme := runtime.NewScheme()
	rbacv1.AddToScheme(scheme)
	corev1.AddToScheme(scheme)
	extv1.AddToScheme(scheme)
	ctrl, _ := client.New(configClient, client.Options{Scheme: scheme})
	for _, res := range []client.Object{sa, saSec, cr, crb} {
		_, err := controllerutil.CreateOrUpdate(ctx, ctrl, res, func() error {
			return nil
		})
		if err != nil {
			return err
		}
	}

	svc := &corev1.Service{}
	err := ctrl.Get(ctx, types.NamespacedName{Namespace: "default", Name: "kubernetes"}, svc)
	if err != nil {
		return err
	}

	mgr, err := manager.New(configClient, manager.Options{})
	if err != nil {
		return err
	}
	mgrContext, cancel := context.WithCancel(context.Background())
	if err = builder.
		ControllerManagedBy(mgr).
		For(&corev1.ServiceAccount{}).
		WithEventFilter(predicate.Funcs{
			UpdateFunc: func(e event.UpdateEvent) bool {
				return e.ObjectNew.GetName() == ProviderConfigName
			},
			DeleteFunc: func(e event.DeleteEvent) bool {
				return e.Object.GetName() == ProviderConfigName
			},
			CreateFunc: func(e event.CreateEvent) bool {
				return e.Object.GetName() == ProviderConfigName
			},
			GenericFunc: func(e event.GenericEvent) bool {
				return e.Object.GetName() == ProviderConfigName
			},
		},
		).
		Complete(&SecretReconciler{
			Client:     ctrl,
			CancelFunc: cancel,
			serverIP:   "https://" + svc.Spec.ClusterIP + ":443",
		}); err != nil {
		return err
	}
	logger.Debug("Starting reconciliation of Kubernetes Provider")
	mgr.Start(mgrContext)
	return nil
}

// Reconcile SvcAcc secret for make kubeconfig
func (a *SecretReconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	sec := &corev1.Secret{}
	err := a.Get(ctx, req.NamespacedName, sec)
	if err != nil {
		return reconcile.Result{}, err
	} else if sec.GetName() != ProviderConfigName {
		return reconcile.Result{Requeue: true}, nil
	}

	if _, err = controllerutil.CreateOrUpdate(ctx, a.Client, sec, func() error {
		kubeconfig, _ := yaml.Marshal(&map[string]interface{}{
			"apiVersion":      "v1",
			"kind":            "Config",
			"current-context": "in-cluster",
			"clusters": []map[string]interface{}{
				{
					"cluster": map[string]interface{}{
						"certificate-authority-data": b64.StdEncoding.EncodeToString(sec.Data["ca.crt"]),
						"server":                     a.serverIP,
					},
					"name": "in-cluster",
				},
			},
			"contexts": []map[string]interface{}{
				{
					"context": map[string]interface{}{
						"cluster":   "in-cluster",
						"user":      "in-cluster",
						"namespace": "overlock-system",
					},
					"name": "in-cluster",
				},
			},
			"preferences": map[string]interface{}{},
			"users": []map[string]interface{}{
				{
					"name": "in-cluster",
					"user": map[string]interface{}{
						"token": string(sec.Data["token"]),
					},
				},
			},
		})

		sec.Data["kubeconfig"] = []byte(kubeconfig)
		return nil
	}); err != nil {
		return reconcile.Result{}, err
	}

	crd := &extv1.CustomResourceDefinition{}
	err = a.Get(ctx, types.NamespacedName{Name: "providerconfigs.kubernetes.crossplane.io"}, crd)
	if err != nil {
		return reconcile.Result{Requeue: true}, err
	}

	pc := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "kubernetes.crossplane.io/v1alpha1",
			"kind":       "ProviderConfig",
			"metadata": map[string]interface{}{
				"name": ProviderConfigName,
			},
		},
	}

	hpc := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "helm.crossplane.io/v1beta1",
			"kind":       "ProviderConfig",
			"metadata": map[string]interface{}{
				"name": helmProviderConfigName,
			},
		},
	}

	envObj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "overlock.io/v1alpha1",
			"kind":       "Environment",
			"metadata": map[string]interface{}{
				"name": "environment",
			},
			"spec": map[string]interface{}{
				"crossplane:": map[string]interface{}{},
				"kyverno:":    map[string]interface{}{},
				"name":        ReleaseName,
				"namespace":   namespace.Namespace,
				"configuration": map[string]interface{}{
					"packages": []interface{}{},
				},
				"provider": map[string]interface{}{
					"packages": []interface{}{},
				},
				"helmProviderCfgRef":       helmProviderConfigName,
				"kubernetesProviderCfgRef": ProviderConfigName,
			},
		},
	}

	if _, err = controllerutil.CreateOrUpdate(ctx, a.Client, pc, func() error {
		pc.Object["spec"] = map[string]interface{}{
			"credentials": map[string]interface{}{
				"secretRef": map[string]interface{}{
					"key":       "kubeconfig",
					"name":      ProviderConfigName,
					"namespace": namespace.Namespace,
				},
				"source": "Secret",
			},
		}
		return nil
	}); err != nil {
		return reconcile.Result{}, err
	}

	if _, err = controllerutil.CreateOrUpdate(ctx, a.Client, hpc, func() error {
		hpc.Object["spec"] = map[string]interface{}{
			"credentials": map[string]interface{}{
				"secretRef": map[string]interface{}{
					"key":       "kubeconfig",
					"name":      ProviderConfigName,
					"namespace": namespace.Namespace,
				},
				"source": "Secret",
			},
		}
		return nil
	}); err != nil {
		return reconcile.Result{}, err
	}

	if _, err = controllerutil.CreateOrUpdate(ctx, a.Client, envObj, func() error { return nil }); err != nil {
		return reconcile.Result{}, err
	}

	a.CancelFunc()

	return reconcile.Result{}, nil
}

func BuildPack(pack v1.Package, img string, pkgMap map[string]string) error {
	ref, err := name.ParseReference(img, name.WithDefaultRegistry(""))
	if err != nil {
		return errors.Wrap(err, errParsePackageName)
	}
	objName := ToDNSLabel(ref.Context().RepositoryStr())
	if existing, ok := pkgMap[ref.Context().RepositoryStr()]; ok {
		objName = existing
	}
	pack.SetName(objName)
	pack.SetSource(ref.String())
	return nil
}

// ToDNSLabel converts the string to a valid DNS label.
func ToDNSLabel(s string) string {
	var cut strings.Builder
	for i := range s {
		b := s[i]
		if ('a' <= b && b <= 'z') || ('0' <= b && b <= '9') {
			cut.WriteByte(b)
		}
		if (b == '.' || b == '/' || b == ':' || b == '-') && (i != 0 && i != 62 && i != len(s)-1) {
			cut.WriteByte('-')
		}
		if i == 62 {
			break
		}
	}
	return strings.Trim(cut.String(), "-")
}
