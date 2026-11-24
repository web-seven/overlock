package provider

import (
	"context"

	"go.uber.org/zap"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"

	"github.com/web-seven/overlock/internal/provider"
)

type deleteCmd struct {
	ProviderUrl string `arg:"" required:"" help:"Crossplane provider package URL to be removed from the environment."`
}

func (c *deleteCmd) Run(ctx context.Context, config *rest.Config, dynamicClient *dynamic.DynamicClient, logger *zap.SugaredLogger) error {
	return provider.DeleteProvider(ctx, config, c.ProviderUrl, logger)
}
