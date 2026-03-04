package chart

import (
	"context"
	"strings"

	"go.uber.org/zap"
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

func (c CrossplaneChart) Install(ctx context.Context, restConfig *rest.Config, logger *zap.SugaredLogger) error {
	installer, err := engine.GetEngine(restConfig)
	if err != nil {
		return err
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

	logger.Debug("Installing engine")
	err = engine.InstallEngine(ctx, restConfig, params, logger)
	if err != nil {
		if strings.Contains(err.Error(), "chart already installed") {
			logger.Info("Engine already installed, skipping installation")
			return nil
		}
		return err
	}
	logger.Debug("Done")
	return nil
}

func (c CrossplaneChart) Apply(restConfig *rest.Config, nodeSelector map[string]interface{}, tolerations []interface{}, logger *zap.SugaredLogger) error {
	params := map[string]any{
		"nodeSelector": nodeSelector,
		"tolerations":  tolerations,
		"rbacManager": map[string]any{
			"nodeSelector": nodeSelector,
			"tolerations":  tolerations,
		},
	}
	return c.def().applyValues(restConfig, params, logger)
}

func (c CrossplaneChart) Remove(restConfig *rest.Config, logger *zap.SugaredLogger) error {
	return c.def().removeValues(restConfig, []string{"nodeSelector", "tolerations", "rbacManager"}, logger)
}
