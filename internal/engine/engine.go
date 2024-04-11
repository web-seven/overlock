package engine

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/kndpio/kndp/internal/install"
	"github.com/kndpio/kndp/internal/install/helm"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrl "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type SecretReconciler struct {
	client.Client
	context.CancelFunc
}

const (
	RepoUrl             = "https://charts.crossplane.io/stable"
	ChartName           = "crossplane"
	ReleaseName         = "kndp-crossplane"
	Namespace           = "kndp-system"
	kindClusterRole     = "ClusterRole"
	clusterAdminRole    = "cluster-admin"
	providerConfigName  = "kndp-kubernetes-provider-config"
	providerConfigSec   = providerConfigName + "-secret"
	svcAccSecretLabel   = providerConfigName + "/account"
	providerSecretLabel = providerConfigName + "/provider"
	aggregateToAdmin    = "rbac.crossplane.io/aggregate-to-admin"
	trueVal             = "true"
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

// Setup Kubernetes provider which has cluster-admin role assigned
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
			Labels: map[string]string{
				svcAccSecretLabel: sa.Name,
			},
		},
		Type: corev1.SecretTypeServiceAccountToken,
	}

	pcSec := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      providerConfigSec,
			Namespace: Namespace,
			Labels: map[string]string{
				providerSecretLabel: sa.Name,
			},
		},
		Type: corev1.SecretTypeOpaque,
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

	pc := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "kubernetes.crossplane.io/v1alpha1",
			"kind":       "ProviderConfig",
			"metadata": map[string]interface{}{
				"name":      pcn,
				"namespace": Namespace,
			},
			"spec": map[string]interface{}{
				"credentials": map[string]interface{}{
					"secretRef": map[string]interface{}{
						"key":       "token",
						"name":      pcSec.Name,
						"namespace": Namespace,
					},
					"source": "Secret",
				},
			},
		},
	}

	kpr := []ctrl.Object{pc, sa, saSec, pcSec, cr, crb}

	scheme := runtime.NewScheme()
	rbacv1.AddToScheme(scheme)
	corev1.AddToScheme(scheme)
	client, _ := ctrl.New(configClient, ctrl.Options{Scheme: scheme})
	for _, res := range kpr {
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
		For(pcSec).
		Owns(saSec).
		Complete(&SecretReconciler{
			Client:     mgr.GetClient(),
			CancelFunc: cancel,
		}); err != nil {
		return err
	}
	mgr.Start(mgrContext)
	return nil
}

func (a *SecretReconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	fmt.Println("reconcile", time.Now())

	pcSec := &corev1.Secret{}
	err := a.Get(ctx, req.NamespacedName, pcSec)
	if err != nil {
		return reconcile.Result{}, err
	}

	saSecList := &corev1.SecretList{}
	matchSaLabels := map[string]string{
		svcAccSecretLabel: pcSec.Labels[providerSecretLabel],
	}
	err = a.List(ctx, saSecList, client.InNamespace(req.Namespace), client.MatchingLabels(matchSaLabels))
	if err != nil {
		return reconcile.Result{}, err
	}

	for _, saSec := range saSecList.Items {
		pcSec.Data = saSec.Data
		a.Update(ctx, pcSec, &ctrl.UpdateOptions{})
		a.CancelFunc()
	}

	return reconcile.Result{}, nil
}
