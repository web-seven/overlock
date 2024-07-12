package provider

import (
	"context"

	"go.uber.org/zap"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"

	"github.com/kndpio/kndp/internal/provider"
)

type applyCmd struct {
	Name string   `arg:"" help:"Name of provider."`
	Link []string `arg:"" required:"" help:"Link URL (or multiple comma separated) to Crossplane provider to be applied to Environment."`
}

func (c *applyCmd) Run(ctx context.Context, dc *dynamic.DynamicClient, config *rest.Config, logger *zap.SugaredLogger) error {
	return provider.New(c.Name).ApplyProvider(ctx, c.Link, config, logger)
}
