package configuration

import (
	"context"

	"go.uber.org/zap"

	"github.com/web-seven/overlock/internal/configuration"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
)

type serveCmd struct {
	Path string `default:"./" arg:"" help:"Path to package directory"`
}

func (c *serveCmd) Run(ctx context.Context, dc *dynamic.DynamicClient, config *rest.Config, logger *zap.SugaredLogger) error {
	return configuration.Serve(ctx, dc, config, logger, c.Path)
}
