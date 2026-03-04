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

func (c CertManagerChart) Install(ctx context.Context, restConfig *rest.Config, logger *zap.SugaredLogger) error {
	logger.Debug("Installing cert-manager")
	err := certmanager.InstallCertManager(ctx, restConfig)
	if err != nil {
		return err
	}
	logger.Debug("Done")
	return nil
}

func (c CertManagerChart) Apply(restConfig *rest.Config, nodeSelector map[string]interface{}, tolerations []interface{}, logger *zap.SugaredLogger) error {
	scope := map[string]any{
		"nodeSelector": nodeSelector,
		"tolerations":  tolerations,
	}
	params := map[string]any{
		"nodeSelector": nodeSelector,
		"tolerations":  tolerations,
		"webhook":      scope,
		"cainjector":   scope,
	}
	return c.def().applyValues(restConfig, params, logger)
}

func (c CertManagerChart) Remove(restConfig *rest.Config, logger *zap.SugaredLogger) error {
	return c.def().removeValues(restConfig, []string{"nodeSelector", "tolerations", "webhook", "cainjector"}, logger)
}
