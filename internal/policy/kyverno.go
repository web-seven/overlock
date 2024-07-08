package policy

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/kndpio/kndp/internal/install/helm"
	"github.com/kndpio/kndp/internal/namespace"
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

// Add registry policies to sync and apply image pull secrets
func addKyvernoRegistryPolicies(ctx context.Context, config *rest.Config, registry *RegistryPolicy) error {

	scplc := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "kyverno.io/v1",
			"kind":       "ClusterPolicy",
			"metadata": map[string]interface{}{
				"name": "kndp-sync-registry-secrets-" + registry.Name,
			},
			"spec": map[string]interface{}{
				"generateExisting": true,
				"rules": []map[string]interface{}{
					{
						"name": "kndp-sync-registry-secrets",
						"match": map[string]interface{}{
							"resources": map[string]interface{}{
								"kinds": []string{
									"Namespace",
								},
							},
						},
						"generate": map[string]interface{}{
							"apiVersion":  "v1",
							"kind":        "Secret",
							"name":        registry.Name,
							"namespace":   "{{request.object.metadata.name}}",
							"synchronize": true,
							"clone": map[string]interface{}{
								"namespace": namespace.Namespace,
								"name":      registry.Name,
							},
						},
					},
				},
			},
		},
	}

	images := []map[string]interface{}{}
	for _, url := range registry.Urls {
		images = append(
			images,
			map[string]interface{}{
				"<(image)": strings.Replace(url, "https://", "", -1) + "/*",
			},
		)
	}

	imsplc := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "kyverno.io/v1",
			"kind":       "ClusterPolicy",
			"metadata": map[string]interface{}{
				"name": "kndp-add-imagepullsecrets-" + registry.Name,
			},
			"spec": map[string]interface{}{
				"generateExisting": true,
				"rules": []map[string]interface{}{
					{
						"name": "kndp-add-imagepullsecret",
						"match": map[string]interface{}{
							"any": []map[string]interface{}{
								{
									"resources": map[string]interface{}{
										"kinds": []string{
											"Pod",
										},
									},
								},
							},
						},
						"skipBackgroundRequests": false,
						"mutate": map[string]interface{}{
							"patchStrategicMerge": map[string]interface{}{
								"spec": map[string]interface{}{
									"containers": images,
									"imagePullSecrets": []map[string]interface{}{
										{
											"name": registry.Name,
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

	_, err = dynamicClient.Resource(gvr).Create(ctx, scplc, metav1.CreateOptions{})
	if err != nil {
		return err
	}
	_, err = dynamicClient.Resource(gvr).Create(ctx, imsplc, metav1.CreateOptions{})
	if err != nil {
		return err
	}

	return nil
}

// Add local registry policies
func addKyvernoLocalRegistryPolicies(ctx context.Context, config *rest.Config, registry *RegistryPolicy) error {

	regplc := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "kyverno.io/v1",
			"kind":       "ClusterPolicy",
			"metadata": map[string]interface{}{
				"name": "kndp-patch-" + registry.Name,
			},
			"spec": map[string]interface{}{
				"generateExisting": true,
				"rules": []interface{}{
					map[string]interface{}{
						"name": "kndp-patch-" + registry.Name,
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
													"(image)": "*registry.kndp-system.svc.cluster.local*",
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
