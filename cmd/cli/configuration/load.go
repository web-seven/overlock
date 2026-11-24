package configuration

import (
	"bufio"
	"context"
	"fmt"
	"os"

	"go.uber.org/zap"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"

	"github.com/web-seven/overlock/pkg/configuration"
)

type loadCmd struct {
	Name    string `arg:"" help:"Name of configuration."`
	Path    string `help:"Path to configuration package archive."`
	Stdin   bool   `help:"Load configuration package from STDIN."`
	Apply   bool   `help:"Apply configuration after load."`
	Upgrade bool   `help:"Upgrade existing configuration."`
}

func (c *loadCmd) Run(ctx context.Context, config *rest.Config, dc *dynamic.DynamicClient, logger *zap.SugaredLogger) error {
	cfg := configuration.New(c.Name)
	if c.Upgrade {
		if err := cfg.UpgradeConfiguration(ctx, config, dc); err != nil {
			return fmt.Errorf("failed to upgrade configuration: %w", err)
		}
	}
	if c.Path != "" {
		fi, err := os.Stat(c.Path)
		if err != nil {
			return fmt.Errorf("failed to stat path %s: %w", c.Path, err)
		}
		switch mode := fi.Mode(); {
		case mode.IsDir():
			logger.Debugf("Loading from directory: %s", c.Path)
			err = cfg.LoadDirectory(ctx, config, logger, c.Path)
			if err != nil {
				return fmt.Errorf("failed to load directory %s: %w", c.Path, err)
			}
		case mode.IsRegular():
			logger.Debugf("Loading from file: %s", c.Path)
			err = cfg.LoadPathArchive(ctx, config, logger, c.Path)
			if err != nil {
				return fmt.Errorf("failed to load path archive %s: %w", c.Path, err)
			}
		}
	} else if c.Stdin {
		logger.Debug("Loading from STDIN")
		reader := bufio.NewReader(os.Stdin)
		err := cfg.LoadStdinArchive(ctx, config, logger, reader)
		if err != nil {
			return err
		}
	} else {
		logger.Warn("Archive path or STDIN required for load configuration.")
		return nil
	}

	if c.Apply {
		return cfg.Apply(ctx, config, logger)
	}

	return nil
}
