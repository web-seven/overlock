package provider

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/web-seven/overlock/internal/image"
	"github.com/web-seven/overlock/internal/kube"
	"github.com/web-seven/overlock/internal/loader"
	"github.com/web-seven/overlock/internal/packages"
	"github.com/web-seven/overlock/pkg/registry"
	"go.uber.org/zap"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
)

const (
	providerFileName             = "provider"
	fileMode         os.FileMode = 0o777
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

	p.Image.Image, _ = loader.LoadPathArchive(path)
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

// Load provider package from directory
func (p *Provider) LoadDirectory(ctx context.Context, config *rest.Config, logger *zap.SugaredLogger, path string, mainPath string) error {

	logger.Debug("Building provider...")
	fileContent, err := p.build(fmt.Sprintf("%s/%s", strings.TrimRight(path, "/"), mainPath))
	if err != nil {
		logger.Infof("Error provider build: %v", err)
	}

	logger.Debug("Loading provider binaries...")
	providerLayer, err := image.LoadBinaryLayer(fileContent, providerFileName, fileMode)
	if err != nil {
		return err
	}

	logger.Debug("Loading provider package...")
	packageLayer, err := image.LoadPackageLayerDirectory(ctx, config, fmt.Sprintf("%s/%s", strings.TrimRight(path, "/"), packages.PackagePath), []string{"Provider", "CustomResourceDefinition"})
	if err != nil {
		return err
	}
	logger.Debugf("Package layer created.")
	logger.Debugf("Creating image configuration...")

	cfg, err := p.Image.Image.ConfigFile()
	if err != nil {
		return err
	}
	cfg = cfg.DeepCopy()
	cfg.Config.WorkingDir = "/"
	cfg.Config.ArgsEscaped = true
	cfg.Config.Entrypoint = []string{
		"/provider",
	}
	cfg.Config.ExposedPorts = map[string]struct{}{
		"9443": {},
	}
	logger.Debugf("Image configuration created.")
	logger.Debug("Update image configuration...")
	p.Image.Image, err = mutate.ConfigFile(p.Image.Image, cfg)
	if err != nil {
		return err
	}

	p.Image.Image, err = mutate.Append(p.Image.Image,
		mutate.Addendum{
			Layer: packageLayer,
		},
		mutate.Addendum{
			Layer: providerLayer,
		},
	)
	if err != nil {
		return err
	}
	return p.load(ctx, config, logger)
}

// Build Go module
func (p *Provider) build(path string) ([]byte, error) {

	args := []string{
		"build", "-C", path, "-o", providerFileName,
	}
	cmd := exec.Command("go", args...)
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, "CGO_ENABLED=0")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	if err != nil {
		return nil, err
	}

	fileContent, err := os.ReadFile(fmt.Sprintf("%s/%s", strings.TrimRight(path, "/"), providerFileName))
	if err != nil {
		return nil, err
	}
	return fileContent, nil
}

// Load provider to registry
func (p *Provider) load(ctx context.Context, config *rest.Config, logger *zap.SugaredLogger) error {
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

	err = registry.PushLocalRegistry(ctx, p.Name, p.Image, config, logger)
	if err != nil {
		return err
	}
	logger.Infof("Image archive %s loaded to local registry.", p.Name)

	return nil
}
