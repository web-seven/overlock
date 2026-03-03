package environment

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	docker "github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	"go.uber.org/zap"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

const (
	k3sDockerImage   = "rancher/k3s:latest"
	k3sServerSuffix  = "-server"
	k3sNetworkSuffix = "-net"
	k3sAPIPort       = "6443/tcp"
	k3sHostPort      = "6443"
	k3sManagedLabel  = "app.kubernetes.io/managed-by"
	k3sManagedValue  = "overlock"
)

// K3sDockerContextName returns the kubeconfig context name for this k3s-docker environment.
func (e *Environment) K3sDockerContextName() string {
	return "k3s-docker-" + e.name
}

func (e *Environment) k3sDockerServerName() string {
	return e.name + k3sServerSuffix
}

func (e *Environment) k3sDockerNetworkName() string {
	return e.name + k3sNetworkSuffix
}

// CreateK3sDockerEnvironment creates a K3s server Docker container and sets up the kubeconfig.
func (e *Environment) CreateK3sDockerEnvironment(ctx context.Context, logger *zap.SugaredLogger) (string, error) {
	cli, err := docker.NewClientWithOpts(docker.FromEnv, docker.WithAPIVersionNegotiation())
	if err != nil {
		return "", fmt.Errorf("failed to create Docker client: %w", err)
	}

	networkName := e.k3sDockerNetworkName()
	serverName := e.k3sDockerServerName()
	contextName := e.K3sDockerContextName()

	// Check if server container already exists
	containers, err := cli.ContainerList(ctx, types.ContainerListOptions{All: true})
	if err != nil {
		return "", fmt.Errorf("failed to list containers: %w", err)
	}
	for _, c := range containers {
		for _, name := range c.Names {
			if name == "/"+serverName {
				logger.Infof("Environment '%s' already exists. Using existing environment.", e.name)
				return contextName, nil
			}
		}
	}

	// Create Docker network
	if err := ensureDockerNetwork(ctx, cli, networkName); err != nil {
		return "", fmt.Errorf("failed to ensure Docker network: %w", err)
	}

	// Build port binding for the K3s API server
	portBindings := nat.PortMap{
		nat.Port(k3sAPIPort): []nat.PortBinding{{HostIP: "127.0.0.1", HostPort: k3sHostPort}},
	}
	exposedPorts := nat.PortSet{
		nat.Port(k3sAPIPort): struct{}{},
	}

	hostConfig := &container.HostConfig{
		Privileged:   true,
		PortBindings: portBindings,
		Tmpfs: map[string]string{
			"/run":     "",
			"/var/run": "",
		},
	}

	if e.mountPath != "" {
		hostConfig.Binds = []string{e.mountPath + ":" + e.containerPath}
	}

	resp, err := cli.ContainerCreate(ctx,
		&container.Config{
			Image:        k3sDockerImage,
			Cmd:          []string{"server", "--disable=traefik", "--node-name=" + serverName},
			ExposedPorts: exposedPorts,
			Labels: map[string]string{
				k3sManagedLabel: k3sManagedValue,
			},
		},
		hostConfig,
		&network.NetworkingConfig{
			EndpointsConfig: map[string]*network.EndpointSettings{
				networkName: {},
			},
		},
		nil,
		serverName,
	)
	if err != nil {
		return "", fmt.Errorf("failed to create K3s server container: %w", err)
	}

	if err := cli.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{}); err != nil {
		return "", fmt.Errorf("failed to start K3s server container: %w", err)
	}

	logger.Info("K3s server container started, waiting for cluster to be ready...")

	if err := waitForK3sReady(ctx, cli, serverName, logger); err != nil {
		return "", fmt.Errorf("K3s server did not become ready: %w", err)
	}

	if err := e.writeK3sDockerKubeconfig(ctx, cli, serverName, contextName); err != nil {
		return "", fmt.Errorf("failed to configure kubeconfig: %w", err)
	}

	logger.Info("k3s-docker environment created successfully")
	return contextName, nil
}

// DeleteK3sDockerEnvironment stops and removes the K3s server container and Docker network.
func (e *Environment) DeleteK3sDockerEnvironment(ctx context.Context, logger *zap.SugaredLogger) error {
	cli, err := docker.NewClientWithOpts(docker.FromEnv, docker.WithAPIVersionNegotiation())
	if err != nil {
		return fmt.Errorf("failed to create Docker client: %w", err)
	}

	// Stop and remove all containers belonging to this environment
	containers, err := cli.ContainerList(ctx, types.ContainerListOptions{All: true})
	if err != nil {
		return fmt.Errorf("failed to list containers: %w", err)
	}
	for _, c := range containers {
		for _, name := range c.Names {
			if strings.HasPrefix(name, "/"+e.name+"-") {
				_ = cli.ContainerStop(ctx, c.ID, container.StopOptions{})
				if err := cli.ContainerRemove(ctx, c.ID, types.ContainerRemoveOptions{Force: true}); err != nil {
					logger.Warnf("Failed to remove container %s: %v", name, err)
				} else {
					logger.Infof("Removed container %s", name)
				}
				break
			}
		}
	}

	// Remove Docker network
	networkName := e.k3sDockerNetworkName()
	networks, err := cli.NetworkList(ctx, types.NetworkListOptions{})
	if err == nil {
		for _, net := range networks {
			if net.Name == networkName {
				if err := cli.NetworkRemove(ctx, net.ID); err != nil {
					logger.Warnf("Failed to remove network %s: %v", networkName, err)
				}
				break
			}
		}
	}

	// Remove kubeconfig context
	removeK3sDockerKubeconfig(e.K3sDockerContextName())

	logger.Infof("Environment %s deleted successfully.", e.name)
	return nil
}

func ensureDockerNetwork(ctx context.Context, cli *docker.Client, networkName string) error {
	networks, err := cli.NetworkList(ctx, types.NetworkListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list networks: %w", err)
	}
	for _, net := range networks {
		if net.Name == networkName {
			return nil
		}
	}
	_, err = cli.NetworkCreate(ctx, networkName, types.NetworkCreate{
		Labels: map[string]string{
			k3sManagedLabel: k3sManagedValue,
		},
	})
	return err
}

func waitForK3sReady(ctx context.Context, cli *docker.Client, containerName string, logger *zap.SugaredLogger) error {
	deadline := time.Now().Add(2 * time.Minute)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		_, err := readFileFromContainer(ctx, cli, containerName, "/etc/rancher/k3s/k3s.yaml")
		if err == nil {
			return nil
		}
		logger.Debug("Waiting for K3s to initialize...")
		time.Sleep(5 * time.Second)
	}
	return fmt.Errorf("timeout waiting for K3s to initialize in container %s", containerName)
}

func (e *Environment) writeK3sDockerKubeconfig(ctx context.Context, cli *docker.Client, serverName, contextName string) error {
	kubeconfigData, err := readFileFromContainer(ctx, cli, serverName, "/etc/rancher/k3s/k3s.yaml")
	if err != nil {
		return fmt.Errorf("failed to read kubeconfig from container: %w", err)
	}

	cfg, err := clientcmd.Load([]byte(kubeconfigData))
	if err != nil {
		return fmt.Errorf("failed to parse kubeconfig: %w", err)
	}

	// Rename entries from K3s defaults ("default") to context-specific names
	clusterName := contextName
	userName := contextName + "-user"

	// Update server address to localhost since port 6443 is published
	for _, cluster := range cfg.Clusters {
		cluster.Server = "https://127.0.0.1:" + k3sHostPort
	}

	if cluster, ok := cfg.Clusters["default"]; ok {
		cfg.Clusters[clusterName] = cluster
		delete(cfg.Clusters, "default")
	}
	if user, ok := cfg.AuthInfos["default"]; ok {
		cfg.AuthInfos[userName] = user
		delete(cfg.AuthInfos, "default")
	}
	if kctx, ok := cfg.Contexts["default"]; ok {
		kctx.Cluster = clusterName
		kctx.AuthInfo = userName
		cfg.Contexts[contextName] = kctx
		delete(cfg.Contexts, "default")
	}
	cfg.CurrentContext = contextName

	return mergeKubeconfig(cfg)
}

func mergeKubeconfig(newCfg *clientcmdapi.Config) error {
	kubeConfigPath := clientcmd.RecommendedHomeFile
	existingCfg, err := clientcmd.LoadFromFile(kubeConfigPath)
	if err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("failed to load existing kubeconfig: %w", err)
		}
		existingCfg = clientcmdapi.NewConfig()
	}

	for k, v := range newCfg.Clusters {
		existingCfg.Clusters[k] = v
	}
	for k, v := range newCfg.AuthInfos {
		existingCfg.AuthInfos[k] = v
	}
	for k, v := range newCfg.Contexts {
		existingCfg.Contexts[k] = v
	}

	return clientcmd.WriteToFile(*existingCfg, kubeConfigPath)
}

func removeK3sDockerKubeconfig(contextName string) {
	kubeConfigPath := clientcmd.RecommendedHomeFile
	cfg, err := clientcmd.LoadFromFile(kubeConfigPath)
	if err != nil {
		return
	}
	if kctx, ok := cfg.Contexts[contextName]; ok {
		delete(cfg.Clusters, kctx.Cluster)
		delete(cfg.AuthInfos, kctx.AuthInfo)
	}
	delete(cfg.Contexts, contextName)
	if cfg.CurrentContext == contextName {
		cfg.CurrentContext = ""
	}
	_ = clientcmd.WriteToFile(*cfg, kubeConfigPath)
}

// readFileFromContainer reads a file from a Docker container via the CopyFromContainer API.
func readFileFromContainer(ctx context.Context, cli *docker.Client, containerID, path string) (string, error) {
	reader, _, err := cli.CopyFromContainer(ctx, containerID, path)
	if err != nil {
		return "", err
	}
	defer reader.Close()

	tr := tar.NewReader(reader)
	for {
		_, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}
		var buf bytes.Buffer
		if _, err := io.Copy(&buf, tr); err != nil {
			return "", err
		}
		return buf.String(), nil
	}
	return "", fmt.Errorf("file not found in container at %s", path)
}
