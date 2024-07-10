package provider

import (
	"context"

	"github.com/charmbracelet/log"
	"github.com/kndpio/kndp/internal/loader"
	"github.com/kndpio/kndp/internal/registry"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
)

// Load Provider package from TAR archive path
func (p *Provider) LoadProvider(ctx context.Context, path string, name string, config *rest.Config, dc *dynamic.DynamicClient, logger *log.Logger) error {
	logger.Debugf("Loading image to: %s", name)
	p.Image, _ = loader.LoadPathArchive(path)
	if p.Upgrade {
		logger.Debug("Upgrading to provider")
		p.UpgradeVersion(ctx, dc, logger)
	}
	logger.Debug("Pushing to local registry")
	err := registry.PushLocalRegistry(ctx, p.Name, p.Image, config, logger)
	var names []string
	names = append(names, name)
	if p.Apply {
		logger.Debug("Apply provider")
		return p.ApplyProvider(ctx, names, config, logger)
	}
	if err != nil {
		return err
	}
	logger.Infof("Image archive %s loaded to local registry.", name)
	return err
}
