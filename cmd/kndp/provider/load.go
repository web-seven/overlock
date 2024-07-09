package provider

import (
	"context"

	"github.com/charmbracelet/log"
	"github.com/kndpio/kndp/internal/provider"

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
)

type loadCmd struct {
	Name    string `arg:"" help:"Name of provider."`
	Path    string `help:"Path to provider package archive."`
	Apply   bool   `help:"Apply provider after load."`
	Upgrade bool   `help:"Upgrade existing provider."`
}

func (p *loadCmd) Run(ctx context.Context, config *rest.Config, dc *dynamic.DynamicClient, logger *log.Logger) error {
	provider := provider.New(p.Name)
	if p.Upgrade {
		provider.UpgradeVersion(ctx, dc, logger)
	}
	provider.LoadProvider(ctx, p.Path, provider.Name, config, logger)
	if p.Apply {
		return provider.ApplyProvider(ctx, provider.Name, config, logger)
	}
	return nil
}
