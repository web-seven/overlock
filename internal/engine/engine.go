package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/kndpio/kndp/internal/install"
	"github.com/kndpio/kndp/internal/install/helm"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrl "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type SecretReconciler struct {
	client.Client
	context.CancelFunc
}

const (
	RepoUrl            = "https://charts.crossplane.io/stable"
	ChartName          = "crossplane"
	ReleaseName        = "kndp-crossplane"
	Namespace          = "kndp-system"
	kindClusterRole    = "ClusterRole"
	clusterAdminRole   = "cluster-admin"
	providerConfigName = "kndp-kubernetes-provider-config"
	aggregateToAdmin   = "rbac.crossplane.io/aggregate-to-admin"
	trueVal            = "true"
)

var (
	managedLabels = map[string]string{
		"app.kubernetes.io/managed-by": "kndp",
	}
	initParameters = map[string]any{
		"provider": map[string]any{
			"packages": []string{
				"xpkg.upbound.io/crossplane-contrib/provider-kubernetes:v0.13.0",
			},
		},
	}
)

// Get engine Helm manager
func GetEngine(configClient *rest.Config) (install.Manager, error) {
	repoURL, err := url.Parse(RepoUrl)
	if err != nil {
		return nil, fmt.Errorf("error parsing repository URL: %v", err)
	}
	setWait := helm.InstallerModifierFn(helm.Wait())
	setNamespace := helm.InstallerModifierFn(helm.WithNamespace(Namespace))
	setUpInstall := helm.InstallerModifierFn(helm.WithUpgradeInstall(true))
	setCreateNs := helm.InstallerModifierFn(helm.WithCreateNamespace(true))

	installer, err := helm.NewManager(
		configClient,
		ChartName,
		repoURL,
		ReleaseName,
		setWait,
		setNamespace,
		setUpInstall,
		setCreateNs,
	)

	if err != nil {
		return nil, fmt.Errorf("error creating Helm manager: %v", err)
	}

	return installer, nil
}

// Install engine Helm release
func InstallEngine(ctx context.Context, configClient *rest.Config) error {
	engine, err := GetEngine(configClient)
	if err != nil {
		return err
	}

	err = engine.Upgrade("", initParameters)
	if err != nil {
		return err
	}

	return SetupPrivilegedKubernetesProvider(ctx, configClient)
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
func SetupPrivilegedKubernetesProvider(ctx context.Context, configClient *rest.Config) error {

	pcn := providerConfigName

	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pcn,
			Namespace: Namespace,
		},
	}

	saSec := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pcn,
			Namespace: Namespace,
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
		AggregationRule: &rbacv1.AggregationRule{
			ClusterRoleSelectors: []metav1.LabelSelector{
				{
					MatchLabels: map[string]string{
						aggregateToAdmin: trueVal,
					},
				},
			},
		},
	}

	crb := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pcn,
			Namespace: Namespace,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      rbacv1.ServiceAccountKind,
				Name:      sa.Name,
				Namespace: Namespace,
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
	client, _ := ctrl.New(configClient, ctrl.Options{Scheme: scheme})
	for _, res := range []ctrl.Object{sa, saSec, cr, crb} {
		controllerutil.CreateOrUpdate(ctx, client, res, func() error {
			return nil
		})
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
				fmt.Println(e.ObjectNew.GetName())
				return e.ObjectNew.GetName() == providerConfigName
			},
			DeleteFunc: func(e event.DeleteEvent) bool {
				return e.Object.GetName() == providerConfigName
			},
			CreateFunc: func(e event.CreateEvent) bool {
				return e.Object.GetName() == providerConfigName
			},
			GenericFunc: func(e event.GenericEvent) bool {
				return e.Object.GetName() == providerConfigName
			},
		},
		).
		Complete(&SecretReconciler{
			Client:     client,
			CancelFunc: cancel,
		}); err != nil {
		return err
	}
	mgr.Start(mgrContext)
	return nil
}

// Reconcile SvcAcc secret for make kubeconfig
func (a *SecretReconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {

	sa := &corev1.ServiceAccount{}
	err := a.Get(ctx, req.NamespacedName, sa)
	if err != nil {
		return reconcile.Result{}, err
	} else if sa.GetName() != providerConfigName {
		return reconcile.Result{Requeue: true}, nil
	}

	sec := &corev1.Secret{}
	err = a.Get(ctx, types.NamespacedName{Namespace: req.Namespace, Name: providerConfigName}, sec)
	if err != nil {
		return reconcile.Result{}, err
	}

	svc := &corev1.Service{}
	err = a.Get(ctx, types.NamespacedName{Namespace: "default", Name: "kubernetes"}, svc)
	if err != nil {
		return reconcile.Result{}, err
	}

	fmt.Println(svc.Spec.ClusterIP)

	pc := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "kubernetes.crossplane.io/v1alpha1",
			"kind":       "ProviderConfig",
			"metadata": map[string]interface{}{
				"name":      providerConfigName,
				"namespace": Namespace,
			},
			"spec": map[string]interface{}{
				"credentials": map[string]interface{}{
					"secretRef": map[string]interface{}{
						"key":       "kubeconfig",
						"name":      providerConfigName,
						"namespace": Namespace,
					},
					"source": "Secret",
				},
			},
		},
	}

	sec.Data["kubeconfig"] = []byte("sdfsd")

	// apiVersion: v1
	// kind: Config
	// current-context: "kind-source"
	// clusters:
	// - cluster:
	//     certificate-authority-data: ""
	// 		server: https://35.234.64.110
	//   	name: gke_break-packages_europe-west3-a_break-packages
	// contexts:
	// - context:
	//     cluster: kind-source
	//     user: kind-source
	//   name: kind-source
	// users:
	// - name: kind-source
	//   user:
	//     client-certificate-data: ""

	for _, res := range []ctrl.Object{pc, sec} {
		_, err := controllerutil.CreateOrUpdate(ctx, a.Client, res, func() error {
			return nil
		})
		if err != nil {

			res2print, _ := json.MarshalIndent(sec, "", "  ")
			fmt.Println(string(res2print))

			fmt.Println(err)

		}
	}
	a.CancelFunc()

	return reconcile.Result{}, nil
}
