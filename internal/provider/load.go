package provider

import (
	"context"

	"github.com/kndpio/kndp/internal/loader"
	"github.com/kndpio/kndp/internal/registry"
	"go.uber.org/zap"
	"k8s.io/client-go/rest"
)

// Load Provider package from TAR archive path
func (p *Provider) LoadProvider(ctx context.Context, path string, name string, config *rest.Config, logger *zap.SugaredLogger) error {
	logger.Debugf("Loading image to: %s", name)
	p.Image, _ = loader.LoadPathArchive(path)
	logger.Debug("Pushing to local registry")
	err := registry.PushLocalRegistry(ctx, p.Name, p.Image, config, logger)
	if err != nil {
		return err
	}
	logger.Infof("Image archive %s loaded to local registry.", name)
	return nil
}
