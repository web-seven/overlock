package provider

import (
	"context"

	"go.uber.org/zap"

	"github.com/web-seven/overlock/internal/provider"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
)

type serveCmd struct {
	Path     string `default:"./" arg:"" help:"Path to package directory"`
	MainPath string `default:"cmd/provider" arg:"" help:"Path to main module"`
}

func (c *serveCmd) Run(ctx context.Context, dc *dynamic.DynamicClient, config *rest.Config, logger *zap.SugaredLogger) error {
	return provider.Serve(ctx, dc, config, logger, c.Path, c.MainPath)
}
