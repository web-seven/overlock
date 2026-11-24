package registry

import (
	"context"
	"fmt"
	"strings"

	semver "github.com/Masterminds/semver/v3"
	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/name"
	regv1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"go.uber.org/zap"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/web-seven/overlock/pkg/registry"
)

const tagDelim = ":"

// OCI media types
const (
	HelmConfigMediaType  = "application/vnd.cncf.helm.config.v1+json"
	HelmContentMediaType = "application/vnd.cncf.helm.chart.content.v1.tar+gzip"
	OCIManifestSchema1   = "application/vnd.oci.image.manifest.v1+json"
	OCIConfigMediaType   = "application/vnd.oci.image.config.v1+json"
	OCILayerMediaType    = "application/vnd.oci.image.layer.v1.tar+gzip"
)

type loadImageCmd struct {
	Registry string `arg:"" help:"Name of the registry to load the image to."`
	Path     string `arg:"" help:"Path to OCI image TAR archive."`
	Name     string `required:"" short:"i" help:"Image name and tag (e.g., my-image:1.0)."`
	Upgrade  bool   `help:"Upgrade patch version if image exists."`
	Helm     bool   `help:"Add Helm chart OCI manifest layers with proper media types."`
}

func (c *loadImageCmd) Run(ctx context.Context, client *kubernetes.Clientset, config *rest.Config, logger *zap.SugaredLogger) error {
	logger.Debugf("Loading image from: %s", c.Path)

	// Check if local registry exists
	isLocal, err := registry.IsLocalRegistry(ctx, client)
	if !isLocal || err != nil {
		if err != nil {
			logger.Debug(err)
		}
		reg := registry.NewLocal()
		reg.SetDefault(true)
		err := reg.Create(ctx, config, logger)
		if err != nil {
			return fmt.Errorf("failed to create local registry: %w", err)
		}
	}

	var image regv1.Image

	// Always create OCI image from empty base with archive as layer
	logger.Debug("Creating OCI image from empty base")
	image, err = createOCIImage(c.Path, c.Helm)
	if err != nil {
		return fmt.Errorf("failed to create OCI image: %w", err)
	}

	imageName := c.Name
	if c.Upgrade {
		logger.Debug("Upgrading image version")
		imageName, err = c.upgradeImageVersion(ctx, config, logger)
		if err != nil {
			return fmt.Errorf("failed to upgrade image version: %w", err)
		}
	}

	// Push to local registry
	logger.Debugf("Pushing image to local registry as: %s", imageName)
	err = registry.PushLocalRegistry(ctx, imageName, image, config, logger)
	if err != nil {
		return fmt.Errorf("failed to push image to registry: %w", err)
	}

	logger.Infof("Image %s loaded to local registry.", imageName)
	return nil
}

// upgradeImageVersion finds existing versions and increments patch version
func (c *loadImageCmd) upgradeImageVersion(ctx context.Context, config *rest.Config, logger *zap.SugaredLogger) (string, error) {
	pRef, err := name.ParseReference(c.Name, name.WithDefaultRegistry(""))
	if err != nil {
		return "", fmt.Errorf("failed to parse image reference: %w", err)
	}

	requestedVersion, err := semver.NewVersion(pRef.Identifier())
	if err != nil {
		return "", fmt.Errorf("failed to parse version: %w", err)
	}

	// Get existing tags from local registry
	existingTags, err := registry.ListLocalRegistryTags(ctx, pRef.Context().Name(), config, logger)
	if err != nil {
		logger.Debugf("Could not list existing tags: %v", err)
		// If we can't list tags, start with patch 0
		newVersion := semver.New(requestedVersion.Major(), requestedVersion.Minor(), 0, "", "")
		return strings.Join([]string{pRef.Context().Name(), newVersion.String()}, tagDelim), nil
	}

	// Find the highest patch version for the requested minor version
	requestedMinorVersion := semver.New(requestedVersion.Major(), requestedVersion.Minor(), 0, "", "")
	highestPatch := uint64(0)
	found := false

	for _, tag := range existingTags {
		existingVersion, err := semver.NewVersion(tag)
		if err != nil {
			continue
		}
		existingMinorVersion := semver.New(existingVersion.Major(), existingVersion.Minor(), 0, "", "")
		if requestedMinorVersion.Equal(existingMinorVersion) {
			found = true
			if existingVersion.Patch() >= highestPatch {
				highestPatch = existingVersion.Patch() + 1
			}
		}
	}

	var newVersion *semver.Version
	if found {
		newVersion = semver.New(requestedVersion.Major(), requestedVersion.Minor(), highestPatch, "", "")
	} else {
		newVersion = semver.New(requestedVersion.Major(), requestedVersion.Minor(), 0, "", "")
	}

	return strings.Join([]string{pRef.Context().Name(), newVersion.String()}, tagDelim), nil
}

// Image wraps regv1.Image for registry operations
type Image struct {
	regv1.Image
}

// createOCIImage creates an OCI image from an archive with empty base layer
// If helm is true, applies Helm-specific media types
func createOCIImage(archivePath string, helm bool) (regv1.Image, error) {
	// Start with empty OCI base image and append the archive as a layer
	img, err := crane.Append(empty.Image, archivePath)
	if err != nil {
		return nil, fmt.Errorf("failed to append layer to empty image: %w", err)
	}

	// Get the layer we just added
	layers, err := img.Layers()
	if err != nil {
		return nil, fmt.Errorf("failed to get image layers: %w", err)
	}

	if len(layers) == 0 {
		return nil, fmt.Errorf("no layers found in image")
	}

	if helm {
		// Rebuild image with Helm-specific media types
		baseImg := mutate.ConfigMediaType(empty.Image, HelmConfigMediaType)

		// Add the layer with Helm content media type
		img, err = mutate.Append(baseImg, mutate.Addendum{
			Layer:     layers[len(layers)-1],
			MediaType: HelmContentMediaType,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to append layer with Helm media type: %w", err)
		}

		// Set the manifest media type to OCI
		img = mutate.MediaType(img, OCIManifestSchema1)
	} else {
		// Use standard OCI media types
		baseImg := mutate.ConfigMediaType(empty.Image, OCIConfigMediaType)

		// Add the layer with standard OCI layer media type
		img, err = mutate.Append(baseImg, mutate.Addendum{
			Layer:     layers[len(layers)-1],
			MediaType: OCILayerMediaType,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to append layer: %w", err)
		}

		// Set the manifest media type to OCI
		img = mutate.MediaType(img, OCIManifestSchema1)
	}

	return img, nil
}
