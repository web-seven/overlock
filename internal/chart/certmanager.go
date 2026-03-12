package chart

import (
	"context"

	"go.uber.org/zap"
	"k8s.io/client-go/rest"

	"github.com/web-seven/overlock/internal/certmanager"
)

type CertManagerChart struct{}

func (CertManagerChart) def() chartDef {
	return chartDef{"cert-manager", "https://charts.jetstack.io", "cert-manager", "cert-manager"}
}

func (c CertManagerChart) Install(ctx context.Context, restConfig *rest.Config, params map[string]any, logger *zap.SugaredLogger) error {
	logger.Debug("Installing cert-manager")
	err := certmanager.InstallCertManager(ctx, restConfig, params)
	if err != nil {
		return err
	}
	logger.Debug("Done")
	return nil
}

func (c CertManagerChart) ScopeParams(nodeSelector map[string]interface{}, tolerations []interface{}) map[string]any {
	scope := map[string]any{
		"nodeSelector": nodeSelector,
		"tolerations":  tolerations,
	}
	return map[string]any{
		"nodeSelector": nodeSelector,
		"tolerations":  tolerations,
		"webhook":      scope,
		"cainjector":   scope,
	}
}

func (c CertManagerChart) Apply(restConfig *rest.Config, nodeSelector map[string]interface{}, tolerations []interface{}, logger *zap.SugaredLogger) error {
	return c.def().applyValues(restConfig, c.ScopeParams(nodeSelector, tolerations), logger)
}

func (c CertManagerChart) Remove(restConfig *rest.Config, logger *zap.SugaredLogger) error {
	return c.def().removeValues(restConfig, []string{"nodeSelector", "tolerations", "webhook", "cainjector"}, logger)
}
