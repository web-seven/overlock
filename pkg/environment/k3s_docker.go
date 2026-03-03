package environment

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"io"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	docker "github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	"go.uber.org/zap"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	k3sDockerImage           = "rancher/k3s:v1.32.2-k3s1"
	k3sDockerContainerPrefix = "k3s-docker-"
	k3sKubeconfigPath        = "/etc/rancher/k3s/k3s.yaml"
	k3sReadinessTimeout      = 120 * time.Second
	k3sReadinessPollInterval = 2 * time.Second
)

// CreateK3sDockerEnvironment creates a k3s cluster running inside a Docker
// container using the Docker Go client directly (no external CLI required).
func (e *Environment) CreateK3sDockerEnvironment(logger *zap.SugaredLogger) (_ string, retErr error) {
	ctx := context.Background()

	dockerClient, err := docker.NewClientWithOpts(docker.FromEnv, docker.WithAPIVersionNegotiation())
	if err != nil {
		return "", fmt.Errorf("failed to create Docker client: %w", err)
	}
	defer dockerClient.Close()

	containerName := e.k3sDockerContainerName()

	// Check if container already exists.
	existing, err := e.findK3sDockerContainer(ctx, dockerClient, containerName)
	if err != nil {
		return "", err
	}
	if existing != nil {
		logger.Infof("Environment '%s' already exists. Using existing environment.", e.name)
		if existing.State != "running" {
			if err := dockerClient.ContainerStart(ctx, existing.ID, types.ContainerStartOptions{}); err != nil {
				return "", fmt.Errorf("failed to start existing container: %w", err)
			}
		}
		return e.K3sDockerContextName(), nil
	}

	// Build port bindings: always expose 6443 (API server) on a random host port.
	apiContainerPort, _ := nat.NewPort("tcp", "6443")
	exposedPorts := nat.PortSet{
		apiContainerPort: struct{}{},
	}
	portBindings := nat.PortMap{
		apiContainerPort: []nat.PortBinding{{HostIP: "0.0.0.0", HostPort: "0"}},
	}

	if !e.disablePorts {
		httpContainerPort, _ := nat.NewPort("tcp", "80")
		exposedPorts[httpContainerPort] = struct{}{}
		portBindings[httpContainerPort] = []nat.PortBinding{
			{HostIP: "0.0.0.0", HostPort: fmt.Sprintf("%d", e.httpPort)},
		}

		httpsContainerPort, _ := nat.NewPort("tcp", "443")
		exposedPorts[httpsContainerPort] = struct{}{}
		portBindings[httpsContainerPort] = []nat.PortBinding{
			{HostIP: "0.0.0.0", HostPort: fmt.Sprintf("%d", e.httpsPort)},
		}
	}

	containerConfig := &container.Config{
		Image:        k3sDockerImage,
		Cmd:          []string{"server"},
		ExposedPorts: exposedPorts,
		Env: []string{
			"K3S_KUBECONFIG_MODE=644",
		},
	}

	var binds []string
	if e.mountPath != "" {
		binds = append(binds, e.mountPath+":"+e.containerPath)
	}

	hostConfig := &container.HostConfig{
		Privileged:   true,
		PortBindings: portBindings,
		Binds:        binds,
		Tmpfs: map[string]string{
			"/run":     "",
			"/var/run": "",
		},
	}

	// Pull the image explicitly; the Docker daemon does not auto-pull when
	// using ContainerCreate via the Go client.
	logger.Infof("Pulling image %s...", k3sDockerImage)
	pullReader, err := dockerClient.ImagePull(ctx, k3sDockerImage, types.ImagePullOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to pull image %s: %w", k3sDockerImage, err)
	}
	_, _ = io.Copy(io.Discard, pullReader)
	pullReader.Close()

	resp, err := dockerClient.ContainerCreate(ctx, containerConfig, hostConfig, nil, nil, containerName)
	if err != nil {
		return "", fmt.Errorf("failed to create k3s-docker container: %w", err)
	}

	// Remove the container if any subsequent step fails, to avoid leaving
	// orphaned containers behind on a failed create.
	defer func() {
		if retErr != nil {
			if removeErr := dockerClient.ContainerRemove(ctx, resp.ID, types.ContainerRemoveOptions{Force: true}); removeErr != nil {
				logger.Warnf("Failed to clean up container %s after error: %v", resp.ID, removeErr)
			}
		}
	}()

	if err := dockerClient.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{}); err != nil {
		return "", fmt.Errorf("failed to start k3s-docker container: %w", err)
	}

	logger.Info("k3s-docker container started, waiting for k3s to be ready...")

	if err := e.waitForK3sDockerReady(ctx, dockerClient, resp.ID, logger); err != nil {
		return "", err
	}

	// Determine the host port that Docker assigned to the API server.
	containerInfo, err := dockerClient.ContainerInspect(ctx, resp.ID)
	if err != nil {
		return "", fmt.Errorf("failed to inspect container: %w", err)
	}
	mappedPorts := containerInfo.NetworkSettings.Ports[apiContainerPort]
	if len(mappedPorts) == 0 {
		return "", fmt.Errorf("no host port mapped for API server port 6443")
	}
	apiHostPort := mappedPorts[0].HostPort

	// Copy kubeconfig from the container.
	kubeconfigData, err := e.copyKubeconfigFromContainer(ctx, dockerClient, resp.ID)
	if err != nil {
		return "", err
	}

	// Merge kubeconfig into host kubeconfig with the rewritten server address.
	contextName := e.K3sDockerContextName()
	serverURL := fmt.Sprintf("https://localhost:%s", apiHostPort)
	if err := mergeK3sDockerKubeconfig(kubeconfigData, contextName, serverURL); err != nil {
		return "", fmt.Errorf("failed to merge kubeconfig: %w", err)
	}

	logger.Info("k3s-docker environment created successfully")
	return contextName, nil
}

// DeleteK3sDockerEnvironment stops and removes the k3s-docker container.
func (e *Environment) DeleteK3sDockerEnvironment(logger *zap.SugaredLogger) error {
	ctx := context.Background()

	dockerClient, err := docker.NewClientWithOpts(docker.FromEnv, docker.WithAPIVersionNegotiation())
	if err != nil {
		return fmt.Errorf("failed to create Docker client: %w", err)
	}
	defer dockerClient.Close()

	containerName := e.k3sDockerContainerName()
	c, err := e.findK3sDockerContainer(ctx, dockerClient, containerName)
	if err != nil {
		return err
	}
	if c == nil {
		logger.Infof("Container '%s' not found, nothing to delete.", containerName)
		return nil
	}

	timeout := 10
	if err := dockerClient.ContainerStop(ctx, c.ID, container.StopOptions{Timeout: &timeout}); err != nil {
		logger.Warnf("Failed to stop container %s: %v", c.ID, err)
	}
	if err := dockerClient.ContainerRemove(ctx, c.ID, types.ContainerRemoveOptions{Force: true}); err != nil {
		return fmt.Errorf("failed to remove container %s: %w", c.ID, err)
	}

	logger.Info("k3s-docker environment deleted successfully")
	return nil
}

// K3sDockerContextName returns the kubeconfig context name for this engine.
func (e *Environment) K3sDockerContextName() string {
	return k3sDockerContainerPrefix + e.name
}

// k3sDockerContainerName returns the Docker container name for this environment.
func (e *Environment) k3sDockerContainerName() string {
	return k3sDockerContainerPrefix + e.name
}

// findK3sDockerContainer returns the first container whose name exactly matches
// containerName, or nil if not found.
func (e *Environment) findK3sDockerContainer(ctx context.Context, dockerClient *docker.Client, containerName string) (*types.Container, error) {
	filterArgs := filters.NewArgs()
	filterArgs.Add("name", containerName)
	containers, err := dockerClient.ContainerList(ctx, types.ContainerListOptions{
		All:     true,
		Filters: filterArgs,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list containers: %w", err)
	}
	for i := range containers {
		for _, name := range containers[i].Names {
			if name == "/"+containerName || name == containerName {
				return &containers[i], nil
			}
		}
	}
	return nil, nil
}

// waitForK3sDockerReady polls until k3s has written its kubeconfig inside the
// container, signalling that the API server is ready to accept connections.
func (e *Environment) waitForK3sDockerReady(ctx context.Context, dockerClient *docker.Client, containerID string, logger *zap.SugaredLogger) error {
	timeout := time.NewTimer(k3sReadinessTimeout)
	defer timeout.Stop()
	ticker := time.NewTicker(k3sReadinessPollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timeout.C:
			return fmt.Errorf("timeout waiting for k3s to be ready after %s", k3sReadinessTimeout)
		case <-ticker.C:
			execID, err := dockerClient.ContainerExecCreate(ctx, containerID, types.ExecConfig{
				Cmd:          []string{"kubectl", "get", "--raw", "/healthz"},
				AttachStdout: true,
				AttachStderr: true,
			})
			if err != nil {
				logger.Debugf("Waiting for k3s container to initialize: %v", err)
				continue
			}

			attachResp, err := dockerClient.ContainerExecAttach(ctx, execID.ID, types.ExecStartCheck{})
			if err != nil {
				continue
			}
			// Drain the output so the exec can complete.
			_, _ = io.Copy(io.Discard, attachResp.Reader)
			attachResp.Close()

			inspectResp, err := dockerClient.ContainerExecInspect(ctx, execID.ID)
			if err == nil && inspectResp.ExitCode == 0 {
				logger.Info("k3s is ready")
				return nil
			}

			logger.Debug("Waiting for k3s to be ready...")
		}
	}
}

// copyKubeconfigFromContainer copies the k3s kubeconfig out of the container
// by reading the tar archive returned by CopyFromContainer.
func (e *Environment) copyKubeconfigFromContainer(ctx context.Context, dockerClient *docker.Client, containerID string) ([]byte, error) {
	reader, _, err := dockerClient.CopyFromContainer(ctx, containerID, k3sKubeconfigPath)
	if err != nil {
		return nil, fmt.Errorf("failed to copy kubeconfig from container: %w", err)
	}
	defer reader.Close()

	tr := tar.NewReader(reader)
	if _, err := tr.Next(); err != nil {
		return nil, fmt.Errorf("failed to read tar archive header: %w", err)
	}

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, tr); err != nil {
		return nil, fmt.Errorf("failed to read kubeconfig from tar archive: %w", err)
	}

	return buf.Bytes(), nil
}

// mergeK3sDockerKubeconfig loads the raw kubeconfig bytes from the container,
// renames all entries to contextName, rewrites the server URL, and merges the
// result into the host kubeconfig using clientcmd.ModifyConfig.
func mergeK3sDockerKubeconfig(kubeconfigData []byte, contextName, serverURL string) error {
	newConfig, err := clientcmd.Load(kubeconfigData)
	if err != nil {
		return fmt.Errorf("failed to parse kubeconfig: %w", err)
	}

	// Rename cluster entries and rewrite the server address.
	for oldName, cluster := range newConfig.Clusters {
		cluster.Server = serverURL
		newConfig.Clusters[contextName] = cluster
		if oldName != contextName {
			delete(newConfig.Clusters, oldName)
		}
	}

	// Rename auth info entries.
	for oldName, auth := range newConfig.AuthInfos {
		newConfig.AuthInfos[contextName] = auth
		if oldName != contextName {
			delete(newConfig.AuthInfos, oldName)
		}
	}

	// Rename context entries and point them to the renamed cluster/user.
	for oldName, ctx := range newConfig.Contexts {
		ctx.Cluster = contextName
		ctx.AuthInfo = contextName
		newConfig.Contexts[contextName] = ctx
		if oldName != contextName {
			delete(newConfig.Contexts, oldName)
		}
	}

	if len(newConfig.Clusters) == 0 || len(newConfig.Contexts) == 0 || len(newConfig.AuthInfos) == 0 {
		return fmt.Errorf("kubeconfig from container is incomplete: clusters=%d, contexts=%d, authinfos=%d",
			len(newConfig.Clusters), len(newConfig.Contexts), len(newConfig.AuthInfos))
	}

	newConfig.CurrentContext = contextName

	// Load the existing host kubeconfig and merge.
	po := clientcmd.NewDefaultPathOptions()
	existingConfig, err := po.GetStartingConfig()
	if err != nil {
		return fmt.Errorf("failed to load existing kubeconfig: %w", err)
	}

	for k, v := range newConfig.Clusters {
		existingConfig.Clusters[k] = v
	}
	for k, v := range newConfig.AuthInfos {
		existingConfig.AuthInfos[k] = v
	}
	for k, v := range newConfig.Contexts {
		existingConfig.Contexts[k] = v
	}
	existingConfig.CurrentContext = contextName

	return clientcmd.ModifyConfig(po, *existingConfig, true)
}
