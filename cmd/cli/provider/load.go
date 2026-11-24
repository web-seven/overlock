package provider

import (
	"context"

	"go.uber.org/zap"

	"github.com/web-seven/overlock/internal/provider"

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
)

type loadCmd struct {
	Name    string `arg:"" help:"Name of provider."`
	Path    string `help:"Path to provider package archive."`
	Apply   bool   `help:"Apply provider after load."`
	Upgrade bool   `help:"Upgrade existing provider."`
}

func (p *loadCmd) Run(ctx context.Context, config *rest.Config, dc *dynamic.DynamicClient, logger *zap.SugaredLogger) error {
	return provider.New(p.Name).WithApply(p.Apply).WithUpgrade(p.Upgrade).LoadProvider(ctx, p.Path, config, dc, logger)
}
