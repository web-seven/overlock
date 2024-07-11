package configuration

import (
	"bufio"
	"context"
	"os"

	"github.com/kndpio/kndp/internal/configuration"
	"github.com/kndpio/kndp/internal/kube"
	"github.com/kndpio/kndp/internal/loader"
	"github.com/kndpio/kndp/internal/registry"
	"go.uber.org/zap"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
)

type loadCmd struct {
	Name    string `arg:"" help:"Name of configuration."`
	Path    string `help:"Path to configuration package archive."`
	Stdin   bool   `help:"Load configuration package from STDIN."`
	Apply   bool   `help:"Apply configuration after load."`
	Upgrade bool   `help:"Upgrade existing configuration."`
}

func (c *loadCmd) Run(ctx context.Context, config *rest.Config, dc *dynamic.DynamicClient, logger *zap.SugaredLogger) error {

	client, err := kube.Client(config)
	if err != nil {
		return err
	}

	if !registry.IsLocalRegistry(ctx, client) {
		logger.Warn("Local registry is not installed.")
		return nil
	}

	reg := registry.Registry{
		Local:   true,
		Default: true,
	}
	reg.Create(ctx, config, logger)

	cfg := configuration.Configuration{}
	cfg.Name = c.Name

	if c.Upgrade {
		cfg.UpgradeVersion(ctx, dc)
	}

	logger.Debugf("Loading image to: %s", cfg.Name)
	if c.Path != "" {
		logger.Debugf("Loading from path: %s", c.Path)
		cfg.Image, err = loader.LoadPathArchive(c.Path)
		if err != nil {
			return err
		}
	} else if c.Stdin {
		logger.Debug("Loading from STDIN")
		reader := bufio.NewReader(os.Stdin)
		err = cfg.LoadStdinArchive(reader)
		if err != nil {
			return err
		}
	} else {
		logger.Warn("Archive path or STDIN required for load configuration.")
		return nil
	}

	logger.Debug("Pushing to local registry")
	err = registry.PushLocalRegistry(ctx, cfg.Name, cfg.Image, config, logger)
	if err != nil {
		return err
	}
	logger.Infof("Image archive %s loaded to local registry.", cfg.Name)

	if c.Apply {
		return configuration.ApplyConfiguration(ctx, cfg.Name, config, logger)
	}
	return nil
}
