package configuration

import (
	"bufio"
	"context"
	"os"

	"github.com/web-seven/overlock/internal/configuration"
	"github.com/web-seven/overlock/internal/kube"
	"github.com/web-seven/overlock/internal/loader"
	"github.com/web-seven/overlock/internal/packages"
	"github.com/web-seven/overlock/internal/registry"
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

	isLocal, err := registry.IsLocalRegistry(ctx, client)
	if !isLocal || err != nil {
		reg := registry.NewLocal()
		err := reg.Create(ctx, config, logger)
		if err != nil {
			return err
		}
	}

	cfg := configuration.Configuration{}
	cfg.Name = c.Name

	cfgs := configuration.GetConfigurations(ctx, dc)
	var pkgs []packages.Package
	for _, c := range cfgs {
		pkg := packages.Package{
			Name: c.Name,
			Url:  c.Spec.Package,
		}
		pkgs = append(pkgs, pkg)
	}
	if c.Upgrade {
		cfg.Name = cfg.UpgradeVersion(ctx, dc, cfg.Name, pkgs)
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
