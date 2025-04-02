package function

import (
	"bufio"
	"context"
	"os"

	"github.com/web-seven/overlock/internal/function"
	"github.com/web-seven/overlock/internal/kube"
	"github.com/web-seven/overlock/internal/packages"
	"github.com/web-seven/overlock/pkg/registry"
	"go.uber.org/zap"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
)

type loadCmd struct {
	Name    string `arg:"" help:"Name of function."`
	Path    string `help:"Path to function package archive."`
	Stdin   bool   `help:"Load function package from STDIN."`
	Apply   bool   `help:"Apply function after load."`
	Upgrade bool   `help:"Upgrade existing function."`
}

func (c *loadCmd) Run(ctx context.Context, config *rest.Config, dc *dynamic.DynamicClient, logger *zap.SugaredLogger) error {

	client, err := kube.Client(config)
	if err != nil {
		return err
	}

	isLocal, err := registry.IsLocalRegistry(ctx, client)
	if !isLocal || err != nil {
		reg := registry.NewLocal()
		reg.SetDefault(true)
		err := reg.Create(ctx, config, logger)
		if err != nil {
			return err
		}
	}

	fnc := function.New(c.Name)

	fncs := function.GetFunctions(ctx, dc)
	var pkgs []packages.Package
	for _, c := range fncs {
		pkg := packages.Package{
			Name: c.Name,
			Url:  c.Spec.Package,
		}
		pkgs = append(pkgs, pkg)
	}
	if c.Upgrade {
		fnc.Name, err = fnc.UpgradeVersion(ctx, dc, fnc.Name, pkgs)
		if err != nil {
			return err
		}
	}

	logger.Debugf("Loading image to: %s", fnc.Name)
	if c.Path != "" {
		logger.Debugf("Loading from path: %s", c.Path)
		err = fnc.Image.LoadPathArchive(c.Path)
		if err != nil {
			return err
		}
	} else if c.Stdin {
		logger.Debug("Loading from STDIN")
		reader := bufio.NewReader(os.Stdin)
		err = fnc.LoadStdinArchive(reader)
		if err != nil {
			return err
		}
	} else {
		logger.Warn("Archive path or STDIN required for load function.")
		return nil
	}

	logger.Debug("Pushing to local registry")
	err = registry.PushLocalRegistry(ctx, fnc.Name, fnc.Image, config, logger)
	if err != nil {
		return err
	}
	logger.Infof("Image archive %s loaded to local registry.", fnc.Name)

	if c.Apply {
		return function.ApplyFunction(ctx, fnc.Name, config, logger)
	}
	return nil
}
