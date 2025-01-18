package image

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/go-containerregistry/pkg/crane"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"k8s.io/client-go/rest"
)

const (
	AnnotationKey string = "io.crossplane.xpkg"
)

type Image struct {
	v1.Image
}

// Load layer from TAR archive path
func (im *Image) LoadPathArchive(path string) error {
	image, err := crane.Load(path)
	if err != nil {
		return err
	}

	layers, err := image.Layers()
	if err != nil {
		return err
	}
	im.Image, err = mutate.AppendLayers(im.Image, layers...)
	if err != nil {
		return err
	}

	return nil
}

// Load layer from file path
func LoadBinaryLayer(content []byte, fileName string, permissions fs.FileMode) (v1.Layer, error) {

	tarBuf := new(bytes.Buffer)
	tw := tar.NewWriter(tarBuf)

	err := tw.WriteHeader(&tar.Header{
		Name: fileName,
		Mode: int64(permissions),
		Size: int64(len(content)),
	})
	if err != nil {
		return nil, err
	}

	_, err = tw.Write(content)
	if err != nil {
		return nil, err
	}

	err = tw.Close()
	if err != nil {
		return nil, err
	}

	layer, err := tarball.LayerFromOpener(func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(tarBuf.Bytes())), nil
	})
	if err != nil {
		return nil, err
	}

	return layer, nil
}

// Load configuration package from directory
func LoadPackageLayerDirectory(ctx context.Context, config *rest.Config, path string) (v1.Layer, error) {
	files, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}
	pkgContent := [][]byte{}
	for _, file := range files {
		if file.Type().IsRegular() && filepath.Ext(file.Name()) == ".yaml" {
			fileContent, err := os.ReadFile(fmt.Sprintf("%s/%s", strings.TrimRight(path, "/"), file.Name()))
			if err != nil {
				return nil, err
			}
			pkgContent = append(pkgContent, fileContent)
		}
	}

	layer, err := crane.Layer(map[string][]byte{
		"package.yaml": bytes.Join(pkgContent, []byte("\n---\n")),
	})
	if err != nil {
		return nil, err
	}
	return layer, nil
}
