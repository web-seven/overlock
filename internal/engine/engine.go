package engine

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/kndpio/kndp/internal/install"
	"github.com/kndpio/kndp/internal/install/helm"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime/pkg/client"
)

const RepoUrl = "https://charts.crossplane.io/stable"

const ChartName = "crossplane"

const ReleaseName = "kndp-crossplane"

const Namespace = "kndp-system"

var managedLabels = map[string]string{
	"app.kubernetes.io/managed-by": "kndp",
}

var initialResources = []ctrl.Object{
	&rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: "TEST",
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      rbacv1.ServiceAccountKind,
				Namespace: Namespace,
				Name:      "provider-kubernetes",
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "ClusterRole",
			Name:     "cluster-admin",
		},
	},
}

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

	err = engine.Upgrade("", nil)
	if err != nil {
		return err
	}

	return injectKubernetesProvider(ctx, configClient)
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

func injectKubernetesProvider(ctx context.Context, configClient *rest.Config) error {
	scheme := runtime.NewScheme()
	rbacv1.AddToScheme(scheme)
	client, _ := ctrl.New(configClient, ctrl.Options{Scheme: scheme})
	for _, res := range initialResources {
		err := client.Create(ctx, res)
		if err != nil {
			return err
		}
	}
	return nil
}
