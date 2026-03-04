package chart

import (
	"context"

	"go.uber.org/zap"
	"k8s.io/client-go/rest"

	"github.com/web-seven/overlock/internal/policy"
)

type KyvernoChart struct{}

func (KyvernoChart) def() chartDef {
	return chartDef{"kyverno", "https://kyverno.github.io/kyverno/", "kyverno", "kyverno"}
}

func (c KyvernoChart) Install(ctx context.Context, restConfig *rest.Config, logger *zap.SugaredLogger) error {
	logger.Debug("Installing policy controller")
	err := policy.AddPolicyConroller(ctx, restConfig, "kyverno")
	if err != nil {
		return err
	}
	logger.Debug("Done")
	return nil
}

func (c KyvernoChart) Apply(restConfig *rest.Config, nodeSelector map[string]interface{}, tolerations []interface{}, logger *zap.SugaredLogger) error {
	params := map[string]any{
		"nodeSelector": nodeSelector,
		"tolerations":  tolerations,
	}
	return c.def().applyValues(restConfig, params, logger)
}

func (c KyvernoChart) Remove(restConfig *rest.Config, logger *zap.SugaredLogger) error {
	return c.def().removeValues(restConfig, []string{"nodeSelector", "tolerations"}, logger)
}
