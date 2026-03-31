package environment

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	docker "github.com/docker/docker/client"
	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
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
		e.skipNodeSetup = true
		return e.K3sDockerContextName(), nil
	}

	// Determine the host's outbound IP so the K3s server advertises a
	// routable address instead of 127.0.0.1. Without this, remote agents
	// receive 127.0.0.1:6444 as the supervisor URL and fail to connect.
	hostIP, err := localIP()
	if err != nil {
		return "", fmt.Errorf("failed to determine host IP: %w", err)
	}

	containerConfig := &container.Config{
		Image: k3sDockerImage,
		Cmd:   []string{"server", "--disable-agent", "--disable=traefik", "--disable-network-policy", "--flannel-backend=wireguard-native", "--flannel-external-ip", "--egress-selector-mode", "cluster", "--bind-address", "0.0.0.0", "--node-ip", hostIP, "--node-external-ip", hostIP, "--advertise-address", hostIP, "--tls-san", hostIP},
		Env: []string{
			"K3S_KUBECONFIG_MODE=644",
		},
	}

	binds := []string{
		"/lib/modules:/lib/modules:ro",
	}
	if e.mountPath != "" {
		binds = append(binds, e.mountPath+":"+e.containerPath)
	}

	nanoCPUs, err := parseCPU(e.cpu)
	if err != nil {
		return "", fmt.Errorf("invalid --cpu value: %w", err)
	}

	hostConfig := &container.HostConfig{
		Privileged:  true,
		NetworkMode: "host",
		Binds:       binds,
		Tmpfs: map[string]string{
			"/run":     "",
			"/var/run": "",
		},
		Resources: container.Resources{
			NanoCPUs: nanoCPUs,
		},
	}

	// Pull the image explicitly; the Docker daemon does not auto-pull when
	// using ContainerCreate via the Go client.
	logger.Debugf("Pulling image %s...", k3sDockerImage)
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

	logger.Debug("k3s-docker container started, waiting for k3s to be ready...")

	if err := e.waitForK3sDockerReady(ctx, dockerClient, resp.ID, logger); err != nil {
		return "", err
	}

	// Copy kubeconfig from the container.
	kubeconfigData, err := e.copyKubeconfigFromContainer(ctx, dockerClient, resp.ID)
	if err != nil {
		return "", err
	}

	// Merge kubeconfig into host kubeconfig. With host networking the API
	// server listens directly on port 6443.
	contextName := e.K3sDockerContextName()
	serverURL := "https://localhost:6443"
	if err := mergeK3sDockerKubeconfig(kubeconfigData, contextName, serverURL); err != nil {
		return "", fmt.Errorf("failed to merge kubeconfig: %w", err)
	}

	logger.Debug("k3s-docker environment created successfully")
	return contextName, nil
}

// DeleteK3sDockerEnvironment stops and removes the k3s-docker server and all
// agent node containers for this environment.
func (e *Environment) DeleteK3sDockerEnvironment(logger *zap.SugaredLogger) error {
	ctx := context.Background()

	dockerClient, err := docker.NewClientWithOpts(docker.FromEnv, docker.WithAPIVersionNegotiation())
	if err != nil {
		return fmt.Errorf("failed to create Docker client: %w", err)
	}
	defer dockerClient.Close()

	// Remove remote node containers discovered via K8s node annotations.
	e.deleteRemoteNodes(ctx, logger)

	// Remove all local agent node containers (k3s-docker-<env>-*).
	serverName := e.k3sDockerContainerName()
	agentPrefix := serverName + "-"
	containers, err := dockerClient.ContainerList(ctx, types.ContainerListOptions{All: true})
	if err != nil {
		return fmt.Errorf("failed to list containers: %w", err)
	}
	timeout := 10
	for _, c := range containers {
		for _, name := range c.Names {
			n := strings.TrimPrefix(name, "/")
			if strings.HasPrefix(n, agentPrefix) {
				logger.Infof("Removing node container %q...", n)
				if err := dockerClient.ContainerStop(ctx, c.ID, container.StopOptions{Timeout: &timeout}); err != nil {
					logger.Warnf("Failed to stop container %s: %v", n, err)
				}
				if err := dockerClient.ContainerRemove(ctx, c.ID, types.ContainerRemoveOptions{Force: true}); err != nil {
					logger.Warnf("Failed to remove container %s: %v", n, err)
				}
			}
		}
	}

	// Remove the server container.
	c, err := e.findK3sDockerContainer(ctx, dockerClient, serverName)
	if err != nil {
		return err
	}
	if c == nil {
		logger.Infof("Container '%s' not found, nothing to delete.", serverName)
		return nil
	}

	if err := dockerClient.ContainerStop(ctx, c.ID, container.StopOptions{Timeout: &timeout}); err != nil {
		logger.Warnf("Failed to stop container %s: %v", c.ID, err)
	}
	if err := dockerClient.ContainerRemove(ctx, c.ID, types.ContainerRemoveOptions{Force: true}); err != nil {
		return fmt.Errorf("failed to remove container %s: %w", c.ID, err)
	}

	logger.Info("k3s-docker environment deleted successfully")
	return nil
}

// deleteRemoteNodes finds K8s nodes with SSH annotations and removes their
// containers on the remote hosts before the cluster is torn down.
func (e *Environment) deleteRemoteNodes(ctx context.Context, logger *zap.SugaredLogger) {
	contextName := e.K3sDockerContextName()
	restConfig, err := config.GetConfigWithContext(contextName)
	if err != nil {
		logger.Debugf("Could not get kubeconfig for remote node cleanup: %v", err)
		return
	}
	kubeClient, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		logger.Debugf("Could not create kube client for remote node cleanup: %v", err)
		return
	}

	nodes, err := kubeClient.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		logger.Debugf("Could not list nodes for remote node cleanup: %v", err)
		return
	}

	for _, node := range nodes.Items {
		remote := remoteFromNodeAnnotations(ctx, kubeClient, node.Name, logger)
		if remote == nil {
			continue
		}
		shortName := node.Labels[nodeLabel]
		if shortName == "" {
			shortName = strings.TrimPrefix(node.Name, e.name+"-")
		}
		if err := e.DeleteNode(ctx, shortName, nil, remote, logger); err != nil {
			logger.Warnf("Failed to delete remote node %q: %v", node.Name, err)
		}
		remote.Close()
	}
}

// startStopRemoteNodes connects to the K8s cluster and starts or stops the
// Docker container for each remote node (nodes with SSH annotations).
// action must be "start" or "stop".
func (e *Environment) startStopRemoteNodes(ctx context.Context, action string, logger *zap.SugaredLogger) {
	contextName := e.K3sDockerContextName()
	restConfig, err := config.GetConfigWithContext(contextName)
	if err != nil {
		logger.Debugf("Could not get kubeconfig for remote node %s: %v", action, err)
		return
	}
	kubeClient, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		logger.Debugf("Could not create kube client for remote node %s: %v", action, err)
		return
	}

	nodes, err := kubeClient.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		logger.Debugf("Could not list nodes for remote node %s: %v", action, err)
		return
	}

	for _, node := range nodes.Items {
		remote := remoteFromNodeAnnotations(ctx, kubeClient, node.Name, logger)
		if remote == nil {
			continue
		}
		shortName := node.Labels[nodeLabel]
		if shortName == "" {
			shortName = strings.TrimPrefix(node.Name, e.name+"-")
		}
		containerName := e.nodeContainerName(shortName)
		logger.Infof("Remote node %q: running docker %s %s on %s", node.Name, action, containerName, remote.Host)
		if _, err := remote.Run(fmt.Sprintf("docker %s %s", action, containerName)); err != nil {
			logger.Warnf("Failed to %s remote container %q on %s: %v", action, containerName, remote.Host, err)
		}
		remote.Close()
	}
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
				logger.Info("k3s server is ready")
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

// localIP returns the preferred outbound IP of this machine.
func localIP() (string, error) {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return "", fmt.Errorf("failed to dial for local IP: %w", err)
	}
	defer func() {
		if cerr := conn.Close(); cerr != nil {
			err = cerr
		}
	}()
	udpAddr, ok := conn.LocalAddr().(*net.UDPAddr)
	if !ok {
		return "", fmt.Errorf("unexpected address type: %T", conn.LocalAddr())
	}
	return udpAddr.IP.String(), nil
}
