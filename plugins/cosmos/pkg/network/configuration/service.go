package configuration

import (
	"bufio"
	"bytes"
	"context"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/web-seven/overlock/pkg/configuration"
	"go.uber.org/zap"
	"gopkg.in/yaml.v2"
	"k8s.io/client-go/rest"
)

func PackageYamlToImageTarball(yamlDocs []map[string]interface{}, packageURL string) (*bytes.Buffer, error) {
	var allDocs bytes.Buffer
	enc := yaml.NewEncoder(&allDocs)

	for _, doc := range yamlDocs {
		if err := enc.Encode(doc); err != nil {
			return nil, err
		}
	}
	enc.Close()

	layer, err := LoadBinaryLayerStream(allDocs.Bytes(), "package.yaml", 0o777)
	if err != nil {
		return nil, err
	}

	image, err := mutate.Append(empty.Image, mutate.Addendum{Layer: layer})
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	tag, err := name.NewTag(packageURL)
	if err != nil {
		return nil, err
	}

	if err := tarball.Write(tag, image, &buf); err != nil {
		return nil, err
	}

	return &buf, nil
}

func LoadConfigFromTar(ctx context.Context, name string, config *rest.Config, logger *zap.SugaredLogger, buf *bytes.Buffer) error {
	reader := bufio.NewReader(buf)
	cfg := configuration.New(name)
	if err := cfg.LoadStdinArchive(ctx, config, logger, reader); err != nil {
		return err
	}
	return cfg.Apply(ctx, config, logger)
}
