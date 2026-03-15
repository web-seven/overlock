package chart

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"

	"github.com/web-seven/overlock/internal/engine"
	"github.com/web-seven/overlock/internal/namespace"
)

type CrossplaneChart struct {
	Configurations []string
	Providers      []string
	Functions      []string
}

func (CrossplaneChart) def() chartDef {
	return chartDef{engine.ChartName, engine.RepoUrl, engine.ReleaseName, namespace.Namespace}
}

func (c CrossplaneChart) Install(ctx context.Context, restConfig *rest.Config, extraParams map[string]any, logger *zap.SugaredLogger) error {
	installer, err := engine.GetEngine(restConfig)
	if err != nil {
		return fmt.Errorf("failed to get engine: %w", err)
	}

	var params map[string]any
	release, err := installer.GetRelease()
	if err == nil {
		params = release.Config
	}
	if configMap, ok := params["configuration"].(map[string]interface{}); ok {
		configMap["packages"] = c.Configurations
	}
	if providersMap, ok := params["providers"].(map[string]interface{}); ok {
		providersMap["packages"] = c.Providers
	}
	if functionsMap, ok := params["functions"].(map[string]interface{}); ok {
		functionsMap["packages"] = c.Functions
	}

	for k, v := range extraParams {
		if params == nil {
			params = make(map[string]any)
		}
		params[k] = v
	}

	logger.Debug("Installing engine")
	err = engine.InstallEngine(ctx, restConfig, params, logger)
	if err != nil {
		if strings.Contains(err.Error(), "chart already installed") {
			logger.Info("Engine already installed, skipping installation")
			return nil
		}
		return fmt.Errorf("failed to install engine: %w", err)
	}
	logger.Debug("Done")
	return nil
}

func (c CrossplaneChart) ScopeParams(nodeSelector map[string]interface{}, tolerations []interface{}) map[string]any {
	return map[string]any{
		"nodeSelector": nodeSelector,
		"tolerations":  tolerations,
		"rbacManager": map[string]any{
			"nodeSelector": nodeSelector,
			"tolerations":  tolerations,
		},
	}
}

func (c CrossplaneChart) Apply(restConfig *rest.Config, nodeSelector map[string]interface{}, tolerations []interface{}, logger *zap.SugaredLogger) error {
	if err := c.def().applyValues(restConfig, c.ScopeParams(nodeSelector, tolerations), logger); err != nil {
		return err
	}
	return PatchDefaultRuntimeConfig(restConfig, nodeSelector, tolerations, logger)
}

func (c CrossplaneChart) Remove(restConfig *rest.Config, logger *zap.SugaredLogger) error {
	return c.def().removeValues(restConfig, []string{"nodeSelector", "tolerations", "rbacManager"}, logger)
}

var runtimeConfigGVR = schema.GroupVersionResource{
	Group:    "pkg.crossplane.io",
	Version:  "v1beta1",
	Resource: "deploymentruntimeconfigs",
}

// patchDefaultRuntimeConfig patches the default DeploymentRuntimeConfig with
// nodeSelector and tolerations so Crossplane providers schedule on the scoped node.
func PatchDefaultRuntimeConfig(restConfig *rest.Config, nodeSelector map[string]interface{}, tolerations []interface{}, logger *zap.SugaredLogger) error {
	dynClient, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		return fmt.Errorf("failed to create dynamic client: %w", err)
	}

	patch := map[string]interface{}{
		"apiVersion": "pkg.crossplane.io/v1beta1",
		"kind":       "DeploymentRuntimeConfig",
		"metadata": map[string]interface{}{
			"name": "default",
		},
		"spec": map[string]interface{}{
			"deploymentTemplate": map[string]interface{}{
				"spec": map[string]interface{}{
					"selector": map[string]interface{}{},
					"template": map[string]interface{}{
						"spec": map[string]interface{}{
							"containers":   []interface{}{map[string]interface{}{"name": "package-runtime"}},
							"nodeSelector": nodeSelector,
							"tolerations":  tolerations,
						},
					},
				},
			},
		},
	}

	patchData, err := json.Marshal(patch)
	if err != nil {
		return fmt.Errorf("failed to marshal patch: %w", err)
	}

	_, err = dynClient.Resource(runtimeConfigGVR).Patch(context.Background(), "default", types.ApplyPatchType, patchData, metav1.PatchOptions{
		FieldManager: "overlock",
	})
	if err != nil {
		return fmt.Errorf("failed to patch default DeploymentRuntimeConfig: %w", err)
	}
	logger.Debug("Patched default DeploymentRuntimeConfig with engine scope.")
	return nil
}
