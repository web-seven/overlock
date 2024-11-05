package provider

import (
	"context"

	"github.com/web-seven/overlock/internal/kube"
	"github.com/web-seven/overlock/internal/loader"
	"github.com/web-seven/overlock/internal/packages"
	"github.com/web-seven/overlock/internal/registry"
	"go.uber.org/zap"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
)

// Load Provider package from TAR archive path
func (p *Provider) LoadProvider(ctx context.Context, path string, config *rest.Config, dc *dynamic.DynamicClient, logger *zap.SugaredLogger) error {
	logger.Debugf("Loading image to: %s", p.Name)

	client, err := kube.Client(config)
	if err != nil {
		return err
	}

	isLocal, err := registry.IsLocalRegistry(ctx, client)
	if !isLocal || err != nil {
		if err != nil {
			logger.Debug(err)
		}
		reg := registry.NewLocal()
		reg.SetDefault(true)
		err := reg.Create(ctx, config, logger)
		if err != nil {
			return err
		}
	}

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
		p.Name, err = p.UpgradeVersion(ctx, dc, p.Name, pkgs)
		if err != nil {
			return err
		}
	}
	logger.Debug("Pushing to local registry")
	err = registry.PushLocalRegistry(ctx, p.Name, p.Image, config, logger)
	if err != nil {
		return err
	}
	logger.Infof("Image archive %s loaded to local registry.", p.Name)
	if p.Apply {
		logger.Debug("Apply provider")
		return p.ApplyProvider(ctx, []string{p.Name}, config, logger)
	}
	return nil
}
