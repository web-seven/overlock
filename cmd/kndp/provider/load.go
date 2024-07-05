package provider

import (
	"context"

	"github.com/charmbracelet/log"
	"github.com/kndpio/kndp/internal/provider"
	"github.com/kndpio/kndp/internal/registry"

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
)

type loadCmd struct {
	Name string `arg:"" help:"Name of provider."`
	Path string `help:"Path to provider package archive."`
}

func (c *loadCmd) Run(ctx context.Context, config *rest.Config, dc *dynamic.DynamicClient, logger *log.Logger) error {
	provider := provider.Provider{}
	provider.Name = c.Name
	logger.Debugf("Loading image to: %s", provider.Name)
	err := provider.LoadPathArchive(c.Path)
	if err != nil {
		return err
	}
	logger.Debug("Pushing to local registry")
	err = registry.PushLocalRegistry(ctx, provider.Name, provider.Image, config, logger)
	if err != nil {
		return err
	}
	logger.Infof("Image archive %s loaded to local registry.", provider.Name)
	return nil
}
