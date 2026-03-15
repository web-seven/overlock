package chart

import (
	"context"
	"fmt"

	"go.uber.org/zap"
	"k8s.io/client-go/rest"

	"github.com/web-seven/overlock/internal/policy"
)

type KyvernoChart struct{}

func (KyvernoChart) def() chartDef {
	return chartDef{"kyverno", "https://kyverno.github.io/kyverno/", "kyverno", "kyverno"}
}

func (c KyvernoChart) Install(ctx context.Context, restConfig *rest.Config, params map[string]any, logger *zap.SugaredLogger) error {
	logger.Debug("Installing policy controller")
	err := policy.AddPolicyConroller(ctx, restConfig, "kyverno", params)
	if err != nil {
		return fmt.Errorf("failed to install kyverno: %w", err)
	}
	logger.Debug("Done")
	return nil
}

func (c KyvernoChart) ScopeParams(nodeSelector map[string]interface{}, tolerations []interface{}) map[string]any {
	return map[string]any{
		"admissionController": map[string]any{
			"nodeSelector": nodeSelector,
			"tolerations":  tolerations,
		},
	}
}

func (c KyvernoChart) Apply(restConfig *rest.Config, nodeSelector map[string]interface{}, tolerations []interface{}, logger *zap.SugaredLogger) error {
	return c.def().applyValues(restConfig, c.ScopeParams(nodeSelector, tolerations), logger)
}

func (c KyvernoChart) Remove(restConfig *rest.Config, logger *zap.SugaredLogger) error {
	return c.def().removeValues(restConfig, []string{"admissionController"}, logger)
}
