package image

import (
	"archive/tar"
	"bytes"
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"

	"github.com/google/go-containerregistry/pkg/crane"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"gopkg.in/yaml.v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
func LoadPackageLayerDirectory(ctx context.Context, config *rest.Config, path string, kindsFilter []string) (v1.Layer, error) {
	var files []string

	err := filepath.WalkDir(path, func(path string, d fs.DirEntry, err error) error {
		if !d.IsDir() && filepath.Ext(path) == ".yaml" {
			res := &metav1.TypeMeta{}
			yamlFile, err := os.ReadFile(path)
			if err != nil {
				return errors.New("can't read")
			}
			err = yaml.Unmarshal(yamlFile, res)
			if err != nil {
				return errors.New("can't unmarshal")
			}

			if slices.Contains(kindsFilter, res.Kind) {
				files = append(files, path)
			}
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	pkgContent := [][]byte{}
	for _, file := range files {
		fileContent, err := os.ReadFile(file)
		if err != nil {
			return nil, err
		}
		pkgContent = append(pkgContent, fileContent)
	}

	layer, err := crane.Layer(map[string][]byte{
		"package.yaml": bytes.Join(pkgContent, []byte("\n---\n")),
	})
	if err != nil {
		return nil, err
	}
	return layer, nil
}
