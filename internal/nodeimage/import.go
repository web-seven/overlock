package nodeimage

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	docker "github.com/docker/docker/client"
	regv1 "github.com/google/go-containerregistry/pkg/v1"
	"go.uber.org/zap"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/web-seven/overlock/api/v1alpha1"
	"github.com/web-seven/overlock/internal/bootstrap"
)

const (
	// ImagesDir is the K3s agent images directory. K3s imports image archives
	// placed there into containerd while running, if the directory exists
	// when K3s starts.
	ImagesDir = "/var/lib/rancher/k3s/agent/images"
	// volumePrefix and the Environment name form the name of the Docker
	// volume shared by the local nodes of an environment as ImagesDir.
	volumePrefix = "overlock-images-"
)

// VolumeName returns the name of the images volume of an environment. Node
// containers that mount it as ImagesDir receive the images loaded by the CLI.
func VolumeName(environment string) string {
	return volumePrefix + environment
}

// Import writes img, named names, into the images volume of the cluster's
// Environment, where K3s imports it into the containerd of every local node
// mounting the volume, so pods start without pulling it. Environments without
// such a volume (e.g. not created by the CLI and not configured with it) are
// skipped.
func Import(ctx context.Context, config *rest.Config, img regv1.Image, names []string, logger *zap.SugaredLogger) error {
	_, err := write(ctx, config, img, names, logger)
	return err
}

// Store writes img named ref into the images volume like Import, but requires
// the volume: with an image store, it is how packages reach the nodes.
func Store(ctx context.Context, config *rest.Config, img regv1.Image, ref string, logger *zap.SugaredLogger) error {
	volume, err := write(ctx, config, img, []string{ref}, logger)
	if err != nil {
		return err
	}
	if volume != "" {
		return fmt.Errorf("no running node container mounts volume %s at %s; recreate the environment with this version of the CLI or mount the volume", volume, ImagesDir)
	}
	return nil
}

// write writes the archive into the images volume. It returns the volume name
// when no container mounts it, and an empty name when the archive was written.
func write(ctx context.Context, config *rest.Config, img regv1.Image, names []string, logger *zap.SugaredLogger) (string, error) {
	environment, err := environmentName(ctx, config)
	if err != nil {
		return "", err
	}
	volume := VolumeName(environment)

	dockerClient, err := docker.NewClientWithOpts(docker.FromEnv, docker.WithAPIVersionNegotiation())
	if err != nil {
		return "", fmt.Errorf("failed to create Docker client: %w", err)
	}
	defer dockerClient.Close()

	container, dir, err := volumeContainer(ctx, dockerClient, volume)
	if err != nil {
		return "", err
	}
	if container == "" {
		logger.Debugf("No running container mounts volume %s, images are pulled from the registry", volume)
		return volume, nil
	}

	archive, err := ociArchive(img, names)
	if err != nil {
		return "", err
	}
	file, err := fileArchive(archiveName(names), archive)
	if err != nil {
		return "", err
	}
	if err := dockerClient.CopyToContainer(ctx, container, dir, bytes.NewReader(file), types.CopyToContainerOptions{}); err != nil {
		return "", fmt.Errorf("failed to write image archive to volume %s: %w", volume, err)
	}
	logger.Debugf("Wrote %s to volume %s", strings.Join(names, ", "), volume)
	return "", nil
}

// archiveName returns a stable file name for an image, so loading the same
// name again replaces its archive and K3s imports the new content.
func archiveName(names []string) string {
	sum := sha256.Sum256([]byte(strings.Join(names, ",")))
	return "overlock-" + hex.EncodeToString(sum[:])[:16] + ".tar"
}

// environmentName returns the name of the cluster's Environment.
func environmentName(ctx context.Context, config *rest.Config) (string, error) {
	scheme := runtime.NewScheme()
	for _, add := range []func(*runtime.Scheme) error{apiextensionsv1.AddToScheme, v1alpha1.AddToScheme} {
		if err := add(scheme); err != nil {
			return "", err
		}
	}
	c, err := client.New(config, client.Options{Scheme: scheme})
	if err != nil {
		return "", err
	}
	env, err := bootstrap.GetEnvironment(ctx, c)
	if err != nil {
		return "", err
	}
	return env.Name, nil
}

// volumeContainer returns a running container mounting volume and the path it
// is mounted at, or an empty container name when there is none.
func volumeContainer(ctx context.Context, dockerClient *docker.Client, volume string) (string, string, error) {
	containers, err := dockerClient.ContainerList(ctx, types.ContainerListOptions{
		Filters: filters.NewArgs(filters.Arg("volume", volume), filters.Arg("status", "running")),
	})
	if err != nil {
		return "", "", fmt.Errorf("failed to list containers: %w", err)
	}
	for _, c := range containers {
		for _, m := range c.Mounts {
			if m.Name == volume {
				return c.ID, m.Destination, nil
			}
		}
	}
	return "", "", nil
}
