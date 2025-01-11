package configuration

import (
	"bufio"
	"context"
	"io"
	"os"

	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/web-seven/overlock/internal/image"
	"github.com/web-seven/overlock/internal/kube"
	"github.com/web-seven/overlock/internal/packages"
	"github.com/web-seven/overlock/internal/registry"
	"go.uber.org/zap"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
)

func (c *Configuration) UpgradeConfiguration(ctx context.Context, config *rest.Config, dc *dynamic.DynamicClient) error {
	cfgs := GetConfigurations(ctx, dc)
	var pkgs []packages.Package
	for _, c := range cfgs {
		pkg := packages.Package{
			Name: c.Name,
			Url:  c.Spec.Package,
		}
		pkgs = append(pkgs, pkg)
	}
	var err error
	c.Name, err = c.UpgradeVersion(ctx, dc, c.Name, pkgs)
	if err != nil {
		return err
	}
	return nil
}

// Load configuration package from path
func (c *Configuration) LoadPathArchive(ctx context.Context, config *rest.Config, logger *zap.SugaredLogger, path string) error {
	err := c.Image.LoadPathArchive(path)
	if err != nil {
		return err
	}
	return c.load(ctx, config, logger)
}

// Load configuration package from STDIN
func (c *Configuration) LoadStdinArchive(ctx context.Context, config *rest.Config, logger *zap.SugaredLogger, stream *bufio.Reader) error {
	stdin, err := io.ReadAll(stream)
	if err != nil {
		return err
	}
	tmpFile, err := os.CreateTemp("", "overlock-configuration-*")
	if err != nil {
		return err
	}
	tmpFile.Write(stdin)
	err = c.Image.LoadPathArchive(tmpFile.Name())
	if err != nil {
		return err
	}
	return c.load(ctx, config, logger)
}

// Load configuration package from directory
func (c *Configuration) LoadDirectory(ctx context.Context, config *rest.Config, logger *zap.SugaredLogger, path string) error {
	packageLayer, err := image.LoadPackageLayerDirectory(ctx, config, path)
	if err != nil {
		return err
	}

	c.Image.Image, err = mutate.AppendLayers(c.Image, packageLayer)
	if err != nil {
		return err
	}

	return c.load(ctx, config, logger)
}

// Load configuration to registry
func (c *Configuration) load(ctx context.Context, config *rest.Config, logger *zap.SugaredLogger) error {
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

	err = registry.PushLocalRegistry(ctx, c.Name, c.Image, config, logger)
	if err != nil {
		return err
	}
	logger.Infof("Image archive %s loaded to local registry.", c.Name)

	return nil
}
