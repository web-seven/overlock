package policy

import (
	"context"
	"fmt"
	"net/url"

	"github.com/web-seven/overlock/internal/install/helm"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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
		"cleanupController": map[string]interface{}{
			"enabled": false,
		},
		"reportsController": map[string]interface{}{
			"enabled": false,
		},
		"backgroundController": map[string]interface{}{
			"enabled": false,
		},
		"features": map[string]interface{}{
			"admissionReports": map[string]interface{}{
				"enabled": "false",
			},
			"aggregateReports": map[string]interface{}{
				"enabled": "false",
			},
			"policyReports": map[string]interface{}{
				"enabled": "false",
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

	err = manager.Upgrade(kyvernoChartVersion, chartValues)
	if err != nil {
		return err
	}

	return nil
}

// Add default policies (currently empty)
func addKyvernoDefaultPolicies(ctx context.Context, config *rest.Config) error {
	return nil
}

// Add registry policies to sync and apply image pull secrets
func addKyvernoRegistryPolicies(ctx context.Context, config *rest.Config, registry *RegistryPolicy) error {

	regplc := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "kyverno.io/v1",
			"kind":       "ClusterPolicy",
			"metadata": map[string]interface{}{
				"name": "overlock-local-reg-" + registry.Name,
			},
			"spec": map[string]interface{}{
				"generateExisting": true,
				"rules": []interface{}{
					map[string]interface{}{
						"name": "overlock-local-reg-" + registry.Name,
						"match": map[string]interface{}{
							"any": []interface{}{
								map[string]interface{}{
									"resources": map[string]interface{}{
										"kinds": []interface{}{
											"Pod",
										},
									},
								},
							},
						},
						"skipBackgroundRequests": false,
						"mutate": map[string]interface{}{
							"foreach": []interface{}{
								map[string]interface{}{
									"list": "request.object.spec.containers",
									"patchStrategicMerge": map[string]interface{}{
										"spec": map[string]interface{}{
											"containers": []interface{}{
												map[string]interface{}{
													"(image)": fmt.Sprintf("*%s*", registry.Name),
													"image":   fmt.Sprintf("{{ regex_replace_all_literal('^[^/]+', '{{element.image}}', 'localhost:%s' )}}", nodePort),
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return err
	}

	gvr := schema.GroupVersionResource{
		Group:    "kyverno.io",
		Version:  "v1",
		Resource: "clusterpolicies",
	}

	deleteKyvernoRegistryPolicies(ctx, config, registry)

	_, err = dynamicClient.Resource(gvr).Create(ctx, regplc, metav1.CreateOptions{})
	if err != nil {
		return err
	}
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
	scplcName := "overlock-local-reg-" + registry.Name
	err = dynamicClient.Resource(gvr).Delete(ctx, scplcName, metav1.DeleteOptions{})
	if err != nil {
		return err
	}
	return nil
}
