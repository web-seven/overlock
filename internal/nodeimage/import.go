package nodeimage

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	docker "github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	regv1 "github.com/google/go-containerregistry/pkg/v1"
	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const (
	// containerPrefix and the node name form the Docker container name of a
	// K3s Docker node.
	containerPrefix = "k3s-docker-"
	// remoteNodeAnnotation marks nodes running on remote hosts.
	remoteNodeAnnotation = "overlock.app/ssh-host"
	// tmpDir receives archives before they are imported with ctr. The K3s
	// images directory is not used: K3s only watches it when it exists at
	// startup, which is not the case for nodes created by the CLI.
	tmpDir        = "/tmp/"
	importTimeout = 2 * time.Minute
	pollInterval  = 2 * time.Second
)

// Import makes img available under each of names in the containerd of every
// local K3s Docker node of the cluster, so pods referencing one of the names
// start without pulling it. Remote nodes and clusters of other engines are
// skipped. Import waits until every node reports the image with its digest.
func Import(ctx context.Context, config *rest.Config, img regv1.Image, names []string, logger *zap.SugaredLogger) error {
	dockerClient, err := docker.NewClientWithOpts(docker.FromEnv, docker.WithAPIVersionNegotiation())
	if err != nil {
		return fmt.Errorf("failed to create Docker client: %w", err)
	}
	defer dockerClient.Close()

	containers, err := localNodeContainers(ctx, config, dockerClient)
	if err != nil || len(containers) == 0 {
		return err
	}

	digest, err := img.Digest()
	if err != nil {
		return err
	}
	archive, err := ociArchive(img, names)
	if err != nil {
		return err
	}
	sum := sha256.Sum256([]byte(strings.Join(names, ",")))
	fileName := "overlock-" + hex.EncodeToString(sum[:])[:16] + ".tar"
	file, err := fileArchive(fileName, archive)
	if err != nil {
		return err
	}

	for _, container := range containers {
		if err := importInto(ctx, dockerClient, container, fileName, file); err != nil {
			return err
		}
		for _, name := range names {
			if err := waitImported(ctx, dockerClient, container, name, digest.String()); err != nil {
				return err
			}
		}
		logger.Debugf("Imported %s into node %s", strings.Join(names, ", "), container)
	}
	return nil
}

// importInto copies the archive into the container and imports it into the
// containerd namespace of the CRI, where the kubelet finds it.
func importInto(ctx context.Context, dockerClient *docker.Client, container, fileName string, file []byte) error {
	if err := dockerClient.CopyToContainer(ctx, container, tmpDir, bytes.NewReader(file), types.CopyToContainerOptions{}); err != nil {
		return fmt.Errorf("failed to copy image archive to %s: %w", container, err)
	}
	path := tmpDir + fileName
	defer func() {
		// Best effort: a leftover file in /tmp does no harm.
		_, _ = exec(ctx, dockerClient, container, "rm", "-f", path)
	}()
	if _, err := exec(ctx, dockerClient, container, "ctr", "-n", "k8s.io", "images", "import", path); err != nil {
		return fmt.Errorf("failed to import image into %s: %w", container, err)
	}
	return nil
}

// localNodeContainers returns the Docker containers of the local K3s Docker
// nodes of the cluster.
func localNodeContainers(ctx context.Context, config *rest.Config, dockerClient *docker.Client) ([]string, error) {
	kube, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	nodes, err := kube.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list nodes: %w", err)
	}

	var containers []string
	for _, node := range nodes.Items {
		if _, remote := node.Annotations[remoteNodeAnnotation]; remote {
			continue
		}
		name := containerPrefix + node.Name
		info, err := dockerClient.ContainerInspect(ctx, name)
		if err != nil || info.State == nil || !info.State.Running {
			continue
		}
		containers = append(containers, name)
	}
	return containers, nil
}

// waitImported waits until the CRI of container knows the image name with the
// expected digest.
func waitImported(ctx context.Context, dockerClient *docker.Client, container, name, digest string) error {
	var got string
	err := wait.PollUntilContextTimeout(ctx, pollInterval, importTimeout, true, func(ctx context.Context) (bool, error) {
		if _, err := exec(ctx, dockerClient, container, "crictl", "inspecti", "-q", name); err != nil {
			return false, nil
		}
		out, err := exec(ctx, dockerClient, container, "ctr", "-n", "k8s.io", "images", "ls", "name=="+name)
		if err != nil {
			return false, nil
		}
		got = imageDigest(out, name)
		return got != "", nil
	})
	if err != nil {
		return fmt.Errorf("image %s was not imported into %s: %w", name, container, err)
	}
	if got != digest {
		return fmt.Errorf("image %s in %s has digest %s, want %s", name, container, got, digest)
	}
	return nil
}

// imageDigest returns the digest of name in the output of "ctr images ls".
func imageDigest(out, name string) string {
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 3 && fields[0] == name {
			return fields[2]
		}
	}
	return ""
}

// exec runs a command in a container and returns its standard output.
func exec(ctx context.Context, dockerClient *docker.Client, container string, cmd ...string) (string, error) {
	created, err := dockerClient.ContainerExecCreate(ctx, container, types.ExecConfig{Cmd: cmd, AttachStdout: true, AttachStderr: true})
	if err != nil {
		return "", err
	}
	attach, err := dockerClient.ContainerExecAttach(ctx, created.ID, types.ExecStartCheck{})
	if err != nil {
		return "", err
	}
	defer attach.Close()
	var stdout, stderr bytes.Buffer
	if _, err := stdcopy.StdCopy(&stdout, &stderr, attach.Reader); err != nil {
		return "", err
	}
	inspect, err := dockerClient.ContainerExecInspect(ctx, created.ID)
	if err != nil {
		return "", err
	}
	if inspect.ExitCode != 0 {
		return "", fmt.Errorf("%s exited %d: %s", strings.Join(cmd, " "), inspect.ExitCode, strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}
