package provider

import (
	"context"

	"github.com/kndpio/kndp/internal/loader"
	"github.com/kndpio/kndp/internal/packages"
	"github.com/kndpio/kndp/internal/registry"
	"k8s.io/client-go/dynamic"
	"go.uber.org/zap"
	"k8s.io/client-go/rest"
)

// Load Provider package from TAR archive path
func (p *Provider) LoadProvider(ctx context.Context, path string, config *rest.Config, dc *dynamic.DynamicClient, logger *zap.SugaredLogger) error {
	logger.Debugf("Loading image to: %s", p.Name)

	p.Image, _ = loader.LoadPathArchive(path)
	providers := ListProviders(ctx, dc, logger)
	var pkgs []packages.Package
	for _, prvd := range providers {
		pkg := packages.Package{
			Name: prvd.Name,
			Url:  prvd.Spec.Package,
		}
		pkgs = append(pkgs, pkg)
	}
	if p.Upgrade {
		logger.Debug("Upgrading provider")
		p.Name = p.UpgradeVersion(ctx, dc, p.Name, pkgs)
	}
	logger.Debug("Pushing to local registry")
	err := registry.PushLocalRegistry(ctx, p.Name, p.Image, config, logger)
	var names []string
	names = append(names, p.Name)
	if p.Apply {
		logger.Debug("Apply provider")
		return p.ApplyProvider(ctx, names, config, logger)
	}
	if err != nil {
		return err
	}
	logger.Infof("Image archive %s loaded to local registry.", p.Name)
	return err
}
