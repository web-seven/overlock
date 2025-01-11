package function

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/web-seven/overlock/internal/image"
	"github.com/web-seven/overlock/internal/kube"
	"github.com/web-seven/overlock/internal/packages"
	"github.com/web-seven/overlock/internal/registry"
	"go.uber.org/zap"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
)

const (
	tagDelim                     = ":"
	regRepoDelimiter             = "/"
	functionFileName             = "function"
	fileMode         os.FileMode = 0o777
)

func (c *Function) UpgradeFunction(ctx context.Context, config *rest.Config, dc *dynamic.DynamicClient) error {
	cfgs := GetFunctions(ctx, dc)
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

// Load function package from STDIN
func (c *Function) LoadStdinArchive(stream *bufio.Reader) error {
	stdin, err := io.ReadAll(stream)
	if err != nil {
		return err
	}
	tmpFile, err := os.CreateTemp("", "overlock-function-*")
	if err != nil {
		return err
	}
	tmpFile.Write(stdin)
	return c.Image.LoadPathArchive(tmpFile.Name())
}

// Load function package from directory
func (c *Function) LoadDirectory(ctx context.Context, config *rest.Config, logger *zap.SugaredLogger, path string) error {

	logger.Debug("Building function...")
	fileContent, err := c.build(path)
	if err != nil {
		logger.Infof("Error function build: %v", err)
	}

	logger.Debug("Loading function binaries...")
	functionLayer, err := image.LoadBinaryLayer(fileContent, functionFileName, fileMode)
	if err != nil {
		return err
	}

	logger.Debug("Loading function package...")
	packageLayer, err := image.LoadPackageLayerDirectory(ctx, config, fmt.Sprintf("%s/%s", strings.TrimRight(path, "/"), packages.PackagePath))
	if err != nil {
		return err
	}
	logger.Debug("Function package loaded.")
	cfg, err := c.Image.ConfigFile()
	if err != nil {
		return err
	}
	cfg = cfg.DeepCopy()
	cfg.Config.WorkingDir = "/"
	cfg.Config.ArgsEscaped = true
	cfg.Config.Entrypoint = []string{
		"/function",
	}
	cfg.Config.ExposedPorts = map[string]struct{}{
		"9443": {},
	}
	logger.Debug("Update image configuration...")
	c.Image.Image, err = mutate.ConfigFile(c.Image.Image, cfg)
	if err != nil {
		return err
	}

	c.Image.Image, err = mutate.Append(c.Image.Image,
		mutate.Addendum{
			Layer: packageLayer,
		},
		mutate.Addendum{
			Layer: functionLayer,
		},
	)
	if err != nil {
		return err
	}
	return c.load(ctx, config, logger)
}

// Build Go module
func (c *Function) build(path string) ([]byte, error) {

	args := []string{
		"build", "-C", path, "-o", functionFileName,
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

	fileContent, err := os.ReadFile(fmt.Sprintf("%s/%s", strings.TrimRight(path, "/"), functionFileName))
	if err != nil {
		return nil, err
	}
	return fileContent, nil
}

// Load function to registry
func (c *Function) load(ctx context.Context, config *rest.Config, logger *zap.SugaredLogger) error {
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
