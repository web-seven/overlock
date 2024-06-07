package configuration

import (
	"bufio"
	"context"
	"os"

	"github.com/charmbracelet/log"
	cfg "github.com/kndpio/kndp/internal/configuration"
	"github.com/kndpio/kndp/internal/kube"
	"github.com/kndpio/kndp/internal/registry"
	"k8s.io/client-go/rest"
)

type loadCmd struct {
	Name  string `arg:"" help:"Name of configuration."`
	Path  string `help:"Path to configuration package archive."`
	Stdin bool   `help:"Load configuration package from STDIN."`
}

func (c *loadCmd) Run(ctx context.Context, config *rest.Config, logger *log.Logger) error {

	client, err := kube.Client(config)
	if err != nil {
		return err
	}

	if !registry.IsLocalRegistry(ctx, client) {
		logger.Warn("Local registry is not installed.")
		return nil
	}

	cfg := cfg.Configuration{}
	cfg.Name = c.Name
	logger.Debugf("Loading image to: %s", cfg.Name)
	if c.Path != "" {
		logger.Debugf("Loading from path: %s", c.Path)
		err = cfg.LoadPathArchive(c.Path)
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
	return nil
}
