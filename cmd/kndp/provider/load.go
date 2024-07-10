package provider

import (
	"context"

	"github.com/kndpio/kndp/internal/provider"
	"go.uber.org/zap"

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
)

type loadCmd struct {
	Name string `arg:"" help:"Name of provider."`
	Path string `help:"Path to provider package archive."`
}

func (c *loadCmd) Run(ctx context.Context, config *rest.Config, dc *dynamic.DynamicClient, logger *zap.Logger) error {
	return provider.New(c.Name).LoadProvider(ctx, c.Path, c.Name, config, logger)
}
