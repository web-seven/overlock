package environment

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	docker "github.com/docker/docker/client"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client/config"

	"github.com/web-seven/overlock/internal/certmanager"
	"github.com/web-seven/overlock/internal/engine"
	"github.com/web-seven/overlock/internal/policy"
)

const (
	nodeScopeEngine    = "engine"
	nodeScopeWorkloads = "workloads"
	engineTaintKey     = "overlock.io/scope"
	engineTaintValue   = "engine"
)

var (
	engineNodeSelector = map[string]interface{}{
		engineTaintKey: engineTaintValue,
	}
	engineTolerations = []interface{}{
		map[string]interface{}{
			"key":      engineTaintKey,
			"operator": "Equal",
			"value":    engineTaintValue,
			"effect":   "NoSchedule",
		},
	}
)

// Node represents a K3s agent node running as a Docker container.
type Node struct {
	name        string
	environment string
	scopes      []string
}

// NewNode creates a new Node entity.
func NewNode(name, environment string, scopes []string) *Node {
	return &Node{
		name:        name,
		environment: environment,
		scopes:      scopes,
	}
}

func (n *Node) containerName() string {
	return n.environment + "-" + n.name
}

func (n *Node) k3sDockerContext() string {
	return "k3s-docker-" + n.environment
}

// Create creates a new K3s agent node Docker container that joins the k3s-docker environment.
// Only supported for k3s-docker environments.
func (n *Node) Create(ctx context.Context, logger *zap.SugaredLogger) error {
	cli, err := docker.NewClientWithOpts(docker.FromEnv, docker.WithAPIVersionNegotiation())
	if err != nil {
		return fmt.Errorf("failed to create Docker client: %w", err)
	}

	serverName := n.environment + k3sServerSuffix
	networkName := n.environment + k3sNetworkSuffix
	containerName := n.containerName()

	// Verify this is a k3s-docker environment by checking the server container exists
	if err := n.verifyK3sDockerEnvironment(ctx, cli, serverName); err != nil {
		return err
	}

	// Read K3s token from server container
	token, err := readFileFromContainer(ctx, cli, serverName, "/var/lib/rancher/k3s/server/node-token")
	if err != nil {
		return fmt.Errorf("failed to read K3s token from server container: %w", err)
	}
	token = strings.TrimSpace(token)

	// Create K3s agent container
	hostConfig := &container.HostConfig{
		Privileged: true,
		Tmpfs: map[string]string{
			"/run":     "",
			"/var/run": "",
		},
	}

	_, err = cli.ContainerCreate(ctx,
		&container.Config{
			Image: k3sDockerImage,
			Cmd:   []string{"agent", "--node-name=" + containerName},
			Env: []string{
				"K3S_URL=https://" + serverName + ":6443",
				"K3S_TOKEN=" + token,
			},
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
		containerName,
	)
	if err != nil {
		return fmt.Errorf("failed to create agent container: %w", err)
	}

	if err := cli.ContainerStart(ctx, containerName, types.ContainerStartOptions{}); err != nil {
		return fmt.Errorf("failed to start agent container: %w", err)
	}

	logger.Infof("K3s agent container %s started, waiting for node to join the cluster...", containerName)

	// Get Kubernetes client for the environment
	restCfg, err := config.GetConfigWithContext(n.k3sDockerContext())
	if err != nil {
		return fmt.Errorf("failed to get cluster config for environment %s: %w", n.environment, err)
	}
	clientset, err := kubernetes.NewForConfig(restCfg)
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	if err := waitForNodeReady(ctx, clientset, containerName, logger); err != nil {
		return fmt.Errorf("node %s did not become ready: %w", containerName, err)
	}

	// Apply scope-specific configuration
	for _, scope := range n.scopes {
		switch scope {
		case nodeScopeEngine:
			if err := applyEngineScope(ctx, clientset, restCfg, containerName, logger); err != nil {
				return fmt.Errorf("failed to apply engine scope: %w", err)
			}
		case nodeScopeWorkloads:
			logger.Infof("Node %s configured for workloads scope (no taints applied)", containerName)
		default:
			logger.Warnf("Unknown scope %q ignored for node %s", scope, containerName)
		}
	}

	logger.Infof("Node %s created successfully.", containerName)
	return nil
}

// Delete stops and removes a K3s agent node container.
// Only supported for k3s-docker environments.
func (n *Node) Delete(ctx context.Context, logger *zap.SugaredLogger) error {
	cli, err := docker.NewClientWithOpts(docker.FromEnv, docker.WithAPIVersionNegotiation())
	if err != nil {
		return fmt.Errorf("failed to create Docker client: %w", err)
	}

	serverName := n.environment + k3sServerSuffix
	containerName := n.containerName()

	// Verify this is a k3s-docker environment
	if err := n.verifyK3sDockerEnvironment(ctx, cli, serverName); err != nil {
		return err
	}

	// Handle engine scope cleanup before removing the container
	for _, scope := range n.scopes {
		if scope == nodeScopeEngine {
			restCfg, err := config.GetConfigWithContext(n.k3sDockerContext())
			if err != nil {
				logger.Warnf("Failed to get cluster config for engine scope cleanup: %v", err)
			} else {
				if err := removeEngineScope(ctx, restCfg, logger); err != nil {
					logger.Warnf("Failed to remove engine scope from Helm charts: %v", err)
				}
			}
		}
	}

	// Remove node from Kubernetes cluster
	restCfg, err := config.GetConfigWithContext(n.k3sDockerContext())
	if err == nil {
		clientset, err := kubernetes.NewForConfig(restCfg)
		if err == nil {
			_ = clientset.CoreV1().Nodes().Delete(ctx, containerName, metav1.DeleteOptions{})
		}
	}

	// Stop and remove the agent container
	containers, err := cli.ContainerList(ctx, types.ContainerListOptions{All: true})
	if err != nil {
		return fmt.Errorf("failed to list containers: %w", err)
	}
	for _, c := range containers {
		for _, name := range c.Names {
			if name == "/"+containerName {
				_ = cli.ContainerStop(ctx, c.ID, container.StopOptions{})
				if err := cli.ContainerRemove(ctx, c.ID, types.ContainerRemoveOptions{Force: true}); err != nil {
					return fmt.Errorf("failed to remove container %s: %w", containerName, err)
				}
				logger.Infof("Node %s deleted successfully.", containerName)
				return nil
			}
		}
	}

	return fmt.Errorf("node container %s not found", containerName)
}

// verifyK3sDockerEnvironment checks that the environment is a k3s-docker environment
// by verifying the server container exists.
func (n *Node) verifyK3sDockerEnvironment(ctx context.Context, cli *docker.Client, serverName string) error {
	containers, err := cli.ContainerList(ctx, types.ContainerListOptions{All: true})
	if err != nil {
		return fmt.Errorf("failed to list containers: %w", err)
	}
	for _, c := range containers {
		for _, name := range c.Names {
			if name == "/"+serverName {
				return nil
			}
		}
	}
	return fmt.Errorf("environment %q is not a k3s-docker environment or does not exist: only k3s-docker engine is supported for node management", n.environment)
}

func waitForNodeReady(ctx context.Context, clientset *kubernetes.Clientset, nodeName string, logger *zap.SugaredLogger) error {
	deadline := time.Now().Add(3 * time.Minute)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		node, err := clientset.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
		if err == nil {
			for _, cond := range node.Status.Conditions {
				if cond.Type == corev1.NodeReady && cond.Status == corev1.ConditionTrue {
					return nil
				}
			}
		}
		logger.Debugf("Waiting for node %s to be ready...", nodeName)
		time.Sleep(5 * time.Second)
	}
	return fmt.Errorf("timeout waiting for node %s to be ready", nodeName)
}

func applyEngineScope(ctx context.Context, clientset *kubernetes.Clientset, restCfg *rest.Config, nodeName string, logger *zap.SugaredLogger) error {
	// Apply taint to the node
	node, err := clientset.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get node %s: %w", nodeName, err)
	}

	taintExists := false
	for _, taint := range node.Spec.Taints {
		if taint.Key == engineTaintKey && taint.Value == engineTaintValue {
			taintExists = true
			break
		}
	}
	if !taintExists {
		node.Spec.Taints = append(node.Spec.Taints, corev1.Taint{
			Key:    engineTaintKey,
			Value:  engineTaintValue,
			Effect: corev1.TaintEffectNoSchedule,
		})
		if _, err := clientset.CoreV1().Nodes().Update(ctx, node, metav1.UpdateOptions{}); err != nil {
			return fmt.Errorf("failed to apply engine taint to node %s: %w", nodeName, err)
		}
		logger.Infof("Applied engine taint (%s=%s:NoSchedule) to node %s", engineTaintKey, engineTaintValue, nodeName)
	}

	// Update Helm charts with nodeSelector and tolerations
	if err := updateEngineHelmCharts(ctx, restCfg, engineNodeSelector, engineTolerations, logger); err != nil {
		return fmt.Errorf("failed to update Helm charts with engine node configuration: %w", err)
	}

	return nil
}

func removeEngineScope(ctx context.Context, restCfg *rest.Config, logger *zap.SugaredLogger) error {
	// Remove nodeSelector and tolerations from Helm charts
	emptyNodeSelector := map[string]interface{}{}
	emptyTolerations := []interface{}{}
	return updateEngineHelmCharts(ctx, restCfg, emptyNodeSelector, emptyTolerations, logger)
}

// updateEngineHelmCharts updates Crossplane, Kyverno, and cert-manager Helm charts
// with the provided nodeSelector and tolerations values.
func updateEngineHelmCharts(ctx context.Context, restCfg *rest.Config, nodeSelector map[string]interface{}, tolerations []interface{}, logger *zap.SugaredLogger) error {
	scopeValues := map[string]any{
		"nodeSelector": nodeSelector,
		"tolerations":  tolerations,
	}

	// Update Crossplane
	crossplaneManager, err := engine.GetEngine(restCfg)
	if err != nil {
		return fmt.Errorf("failed to get Crossplane Helm manager: %w", err)
	}
	crossplaneVersion, err := crossplaneManager.GetCurrentVersion()
	if err != nil {
		logger.Warnf("Could not get current Crossplane version, skipping update: %v", err)
	} else {
		if err := crossplaneManager.Upgrade(crossplaneVersion, scopeValues); err != nil {
			return fmt.Errorf("failed to update Crossplane Helm chart: %w", err)
		}
		logger.Info("Updated Crossplane Helm chart with engine node configuration")
	}

	// Update Kyverno
	kyvernoManager, err := policy.GetKyvernoManager(restCfg)
	if err != nil {
		return fmt.Errorf("failed to get Kyverno Helm manager: %w", err)
	}
	kyvernoVersion, err := kyvernoManager.GetCurrentVersion()
	if err != nil {
		logger.Warnf("Could not get current Kyverno version, skipping update: %v", err)
	} else {
		if err := kyvernoManager.Upgrade(kyvernoVersion, scopeValues); err != nil {
			return fmt.Errorf("failed to update Kyverno Helm chart: %w", err)
		}
		logger.Info("Updated Kyverno Helm chart with engine node configuration")
	}

	// Update cert-manager
	certManagerManager, err := certmanager.GetCertManagerManager(restCfg)
	if err != nil {
		return fmt.Errorf("failed to get cert-manager Helm manager: %w", err)
	}
	certManagerVersion, err := certManagerManager.GetCurrentVersion()
	if err != nil {
		logger.Warnf("Could not get current cert-manager version, skipping update: %v", err)
	} else {
		if err := certManagerManager.Upgrade(certManagerVersion, scopeValues); err != nil {
			return fmt.Errorf("failed to update cert-manager Helm chart: %w", err)
		}
		logger.Info("Updated cert-manager Helm chart with engine node configuration")
	}

	return nil
}
