package function

import (
	"archive/tar"
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"strings"

	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/web-seven/overlock/internal/kube"
	"github.com/web-seven/overlock/internal/loader"
	"github.com/web-seven/overlock/internal/packages"
	"github.com/web-seven/overlock/internal/registry"
	"go.uber.org/zap"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
)

const (
	tagDelim         = ":"
	regRepoDelimiter = "/"
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
	if err != nil {
		return err
	}
	c.Image, err = loader.LoadPathArchive(tmpFile.Name())
	return err
}

// Load function package from directory
func (c *Function) LoadDirectory(ctx context.Context, config *rest.Config, logger *zap.SugaredLogger, path string) error {
	files, err := os.ReadDir(path)
	if err != nil {
		logger.Error(err)
	}

	pkgFile, err := os.CreateTemp("", "overlock-configuration-*")
	if err != nil {
		return err
	}
	layerFile, err := os.CreateTemp("", "overlock-configuration-*")
	if err != nil {
		return err
	}

	pkgContent := [][]byte{}
	for _, file := range files {
		if file.Type().IsRegular() {
			fileContent, err := os.ReadFile(fmt.Sprintf("%s/%s", strings.TrimRight(path, "/"), file.Name()))
			if err != nil {
				return err
			}
			pkgContent = append(pkgContent, fileContent)
		}
	}
	os.WriteFile(pkgFile.Name(), bytes.Join(pkgContent, []byte("\n---\n")), fs.ModeAppend)
	err = addToArchive(createArchive(layerFile), pkgFile, "package.yaml")
	if err != nil {
		return err
	}

	logger.Debugf("Archive %s created, loading to registry.", layerFile)
	c.Image, err = crane.Append(empty.Image, layerFile.Name())
	if err != nil {
		return err
	}
	return c.load(ctx, config, logger)
}

func (c *Function) build() {
}

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

func createArchive(buf io.Writer) *tar.Writer {
	tw := tar.NewWriter(buf)
	return tw
}

func addToArchive(tw *tar.Writer, file *os.File, filename string) error {
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return err
	}

	header, err := tar.FileInfoHeader(info, info.Name())
	if err != nil {
		return err
	}
	header.Name = filename
	err = tw.WriteHeader(header)
	if err != nil {
		return err
	}
	_, err = io.Copy(tw, file)
	if err != nil {
		return err
	}

	return nil
}
