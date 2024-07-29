package policy

import (
	"context"
	"net/url"

	"github.com/kndpio/kndp/internal/install/helm"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
)

const (
	kyvernoChartName    = "kyverno"
	kyvernoChartVersion = "3.2.5"
	kyvernoReleaseName  = "kyverno"
	kyvernoRepoUrl      = "https://kyverno.github.io/kyverno/"
	kyvernoNamespace    = "kyverno"
	nodePort            = "30100"
)

var (
	chartValues = map[string]interface{}{
		"features": map[string]interface{}{
			"admissionReports": map[string]interface{}{
				"enabled": "true",
			},

			"aggregateReports": map[string]interface{}{
				"enabled": "true",
			},
			"policyReports": map[string]interface{}{
				"enabled": "true",
			},
		},
	}
)

func addKyvernoPolicyConroller(ctx context.Context, config *rest.Config) error {
	repoURL, err := url.Parse(kyvernoRepoUrl)
	if err != nil {
		return err
	}

	manager, err := helm.NewManager(config, kyvernoChartName, repoURL, kyvernoReleaseName,
		helm.InstallerModifierFn(helm.WithNamespace(kyvernoNamespace)),
		helm.InstallerModifierFn(helm.WithUpgradeInstall(true)),
		helm.InstallerModifierFn(helm.WithCreateNamespace(true)),
	)
	if err != nil {
		return err
	}

	manager.Upgrade(kyvernoChartVersion, chartValues)
	if err != nil {
		return err
	}

	return nil
}

// Add default policies (currently empty)
func addKyvernoDefaultPolicies(ctx context.Context, config *rest.Config) error {
	return nil
}

// Delete policies of removed registry
func deleteKyvernoRegistryPolicies(ctx context.Context, config *rest.Config, registry *RegistryPolicy) error {
	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return err
	}

	gvr := schema.GroupVersionResource{
		Group:    "kyverno.io",
		Version:  "v1",
		Resource: "clusterpolicies",
	}
	scplcName := "kndp-sync-registry-secrets-" + registry.Name
	err = dynamicClient.Resource(gvr).Delete(ctx, scplcName, metav1.DeleteOptions{})
	if err != nil {
		return err
	}
	imsplc := "kndp-add-imagepullsecrets-" + registry.Name
	err = dynamicClient.Resource(gvr).Delete(ctx, imsplc, metav1.DeleteOptions{})
	if err != nil {
		return err
	}
	return nil
}
