package environment

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/network"
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

	addrs := computeEnvNetAddrs(e.name)

	// Create the Docker bridge network for this environment.
	if err := e.createEnvironmentNetwork(ctx, dockerClient); err != nil {
		return "", fmt.Errorf("failed to create environment network: %w", err)
	}

	containerConfig := &container.Config{
		Image:    k3sDockerImage,
		Hostname: e.name + "-server",
		Cmd: []string{
			"server",
			"--disable-agent",
			"--disable=traefik",
			"--disable-network-policy",
			"--flannel-backend=vxlan",
			"--flannel-iface", "eth0",
			"--egress-selector-mode", "cluster",
			"--node-ip", addrs.serverIP,
			"--tls-san", addrs.serverIP,
		},
		Env: []string{
			"K3S_KUBECONFIG_MODE=644",
		},
	}

	binds := []string{
		"/lib/modules:/lib/modules:ro",
	}
	binds = append(binds, e.mounts...)

	nanoCPUs, err := parseCPU(e.cpu)
	if err != nil {
		return "", fmt.Errorf("invalid --cpu value: %w", err)
	}

	hostConfig := &container.HostConfig{
		Privileged: true,
		Binds:      binds,
		Tmpfs: map[string]string{
			"/run":     "",
			"/var/run": "",
		},
		Resources: container.Resources{
			NanoCPUs: nanoCPUs,
		},
	}

	netCfg := &network.NetworkingConfig{
		EndpointsConfig: map[string]*network.EndpointSettings{
			e.envNetworkName(): {
				IPAMConfig: &network.EndpointIPAMConfig{
					IPv4Address: addrs.serverIP,
				},
			},
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

	resp, err := dockerClient.ContainerCreate(ctx, containerConfig, hostConfig, netCfg, nil, containerName)
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

	// Merge kubeconfig into host kubeconfig. The API server is reachable
	// via the Docker bridge network at the server's fixed IP.
	contextName := e.K3sDockerContextName()
	serverURL := fmt.Sprintf("https://%s:6443", computeEnvNetAddrs(e.name).serverIP)
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

	e.deleteEnvironmentNetwork(ctx, dockerClient, logger)

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

// waitForAPIServer polls the Kubernetes API server until it responds successfully
// or the context deadline / k3sReadinessTimeout is reached.
func (e *Environment) waitForAPIServer(ctx context.Context, kubeClient *kubernetes.Clientset, logger *zap.SugaredLogger) error {
	timeout := time.NewTimer(k3sReadinessTimeout)
	defer timeout.Stop()
	ticker := time.NewTicker(k3sReadinessPollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timeout.C:
			return fmt.Errorf("timeout waiting for API server to be ready after %s", k3sReadinessTimeout)
		case <-ticker.C:
			if _, err := kubeClient.CoreV1().Nodes().List(ctx, metav1.ListOptions{}); err == nil {
				return nil
			}
			logger.Debug("Waiting for API server to be ready...")
		}
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

	if action == "start" {
		logger.Debug("Waiting for API server to be ready before starting remote nodes...")
		if err := e.waitForAPIServer(ctx, kubeClient, logger); err != nil {
			logger.Warnf("API server not ready, skipping remote node start: %v", err)
			return
		}
	}

	nodes, err := kubeClient.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		logger.Debugf("Could not list nodes for remote node %s: %v", action, err)
		return
	}

	var dockerClient *docker.Client
	if action == "start" {
		dockerClient, err = docker.NewClientWithOpts(docker.FromEnv, docker.WithAPIVersionNegotiation())
		if err != nil {
			logger.Warnf("Failed to create Docker client for WireGuard tunnel: %v", err)
		} else {
			defer dockerClient.Close()
		}
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

		if action == "start" && dockerClient != nil {
			peerIdx := -1
			if s := node.Annotations[annWGPeerIdx]; s != "" {
				if idx, err := strconv.Atoi(s); err == nil {
					peerIdx = idx
				}
			}
			if peerIdx >= 0 {
				if err := e.ensureRemotePeer(ctx, dockerClient, remote, peerIdx, logger); err != nil {
					logger.Warnf("Failed to ensure WireGuard peer for %s: %v", remote.Host, err)
				}
			}
		}

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

