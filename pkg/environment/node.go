package environment

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/url"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	docker "github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client/config"

	"github.com/web-seven/overlock/internal/engine"
	"github.com/web-seven/overlock/internal/install"
	"github.com/web-seven/overlock/internal/install/helm"
	"github.com/web-seven/overlock/internal/namespace"
)

const (
	engineScopeLabel  = "overlock.io/scope"
	engineScopeValue  = "engine"
	engineScopeEffect = "NoSchedule"

	kyvernoChartNameNode   = "kyverno"
	kyvernoReleaseNameNode = "kyverno"
	kyvernoRepoURLNode     = "https://kyverno.github.io/kyverno/"
	kyvernoNamespaceNode   = "kyverno"

	certManagerChartNameNode   = "cert-manager"
	certManagerReleaseNameNode = "cert-manager"
	certManagerRepoURLNode     = "https://charts.jetstack.io"
	certManagerNamespaceNode   = "cert-manager"
)

// chartDef holds metadata required to create a Helm manager for a given chart.
type chartDef struct {
	name      string
	repoURL   string
	relName   string
	namespace string
}

// engineCharts returns the three Helm charts that carry engine-scope scheduling.
func engineCharts() []chartDef {
	return []chartDef{
		{engine.ChartName, engine.RepoUrl, engine.ReleaseName, namespace.Namespace},
		{kyvernoChartNameNode, kyvernoRepoURLNode, kyvernoReleaseNameNode, kyvernoNamespaceNode},
		{certManagerChartNameNode, certManagerRepoURLNode, certManagerReleaseNameNode, certManagerNamespaceNode},
	}
}

// nodeContainerName returns the Docker container name for an agent node.
// Pattern: <k3s-docker-prefix><environment>-<nodeName>
func (e *Environment) nodeContainerName(nodeName string) string {
	return k3sDockerContainerPrefix + e.name + "-" + nodeName
}

// CreateNode creates a new K3s agent node as a Docker container that joins the
// existing K3s server for this environment. Only supported for the k3s-docker engine.
func (e *Environment) CreateNode(ctx context.Context, nodeName string, scopes []string, logger *zap.SugaredLogger) error {
	if e.engine != "k3s-docker" {
		return fmt.Errorf("node management is only supported for the k3s-docker engine, got %q", e.engine)
	}

	dockerClient, err := docker.NewClientWithOpts(docker.FromEnv, docker.WithAPIVersionNegotiation())
	if err != nil {
		return fmt.Errorf("failed to create Docker client: %w", err)
	}
	defer dockerClient.Close()

	// Find the server container.
	serverContainerName := e.k3sDockerContainerName()
	serverContainer, err := e.findK3sDockerContainer(ctx, dockerClient, serverContainerName)
	if err != nil {
		return err
	}
	if serverContainer == nil {
		return fmt.Errorf("server container %q not found; make sure the environment exists and is running", serverContainerName)
	}

	// Get the server container's internal IP address.
	serverInfo, err := dockerClient.ContainerInspect(ctx, serverContainer.ID)
	if err != nil {
		return fmt.Errorf("failed to inspect server container: %w", err)
	}
	serverIP := ""
	for _, network := range serverInfo.NetworkSettings.Networks {
		serverIP = network.IPAddress
		break
	}
	if serverIP == "" {
		return fmt.Errorf("could not determine server container IP address")
	}

	// Retrieve the K3s node join token from the server container.
	token, err := e.getK3sToken(ctx, dockerClient, serverContainer.ID)
	if err != nil {
		return fmt.Errorf("failed to get K3s token: %w", err)
	}

	// The Kubernetes node name follows the pattern <environment>-<nodeName>.
	k3sNodeName := e.name + "-" + nodeName
	agentContainerName := e.nodeContainerName(nodeName)

	// Check if the agent container already exists.
	existing, err := e.findK3sDockerContainer(ctx, dockerClient, agentContainerName)
	if err != nil {
		return err
	}
	if existing != nil {
		logger.Infof("Node container %q already exists.", agentContainerName)
		if existing.State != "running" {
			if err := dockerClient.ContainerStart(ctx, existing.ID, types.ContainerStartOptions{}); err != nil {
				return fmt.Errorf("failed to start existing node container: %w", err)
			}
		}
		return nil
	}

	containerConfig := &container.Config{
		Image: k3sDockerImage,
		Cmd:   []string{"agent", "--node-name", k3sNodeName},
		Env: []string{
			fmt.Sprintf("K3S_URL=https://%s:6443", serverIP),
			fmt.Sprintf("K3S_TOKEN=%s", token),
		},
		Labels: map[string]string{
			"app.kubernetes.io/managed-by": "overlock",
		},
	}

	hostConfig := &container.HostConfig{
		Privileged: true,
		Tmpfs: map[string]string{
			"/run":     "",
			"/var/run": "",
		},
	}

	// Pull the image (likely already cached from environment creation).
	logger.Infof("Pulling image %s...", k3sDockerImage)
	pullReader, err := dockerClient.ImagePull(ctx, k3sDockerImage, types.ImagePullOptions{})
	if err != nil {
		return fmt.Errorf("failed to pull image %s: %w", k3sDockerImage, err)
	}
	_, _ = io.Copy(io.Discard, pullReader)
	pullReader.Close()

	resp, err := dockerClient.ContainerCreate(ctx, containerConfig, hostConfig, nil, nil, agentContainerName)
	if err != nil {
		return fmt.Errorf("failed to create node container: %w", err)
	}

	if err := dockerClient.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{}); err != nil {
		_ = dockerClient.ContainerRemove(ctx, resp.ID, types.ContainerRemoveOptions{Force: true})
		return fmt.Errorf("failed to start node container: %w", err)
	}

	logger.Infof("Node container %q started, waiting for node to join the cluster...", agentContainerName)

	// Get kubeconfig context for this environment.
	contextName := e.K3sDockerContextName()
	restConfig, err := config.GetConfigWithContext(contextName)
	if err != nil {
		return fmt.Errorf("failed to get kubeconfig for environment %q: %w", e.name, err)
	}

	kubeClient, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	if err := e.waitForNodeReady(ctx, kubeClient, k3sNodeName, logger); err != nil {
		return err
	}

	// Apply requested scopes.
	for _, scope := range scopes {
		if scope == "engine" {
			if err := e.applyEngineScope(ctx, kubeClient, restConfig, k3sNodeName, logger); err != nil {
				return fmt.Errorf("failed to apply engine scope: %w", err)
			}
		}
	}

	logger.Infof("Node %q created successfully.", nodeName)
	return nil
}

// DeleteNode stops and removes the K3s agent node container. When the engine
// scope was applied, this also clears nodeSelector and tolerations from
// engine-related Helm charts. Only supported for the k3s-docker engine.
func (e *Environment) DeleteNode(ctx context.Context, nodeName string, scopes []string, logger *zap.SugaredLogger) error {
	if e.engine != "k3s-docker" {
		return fmt.Errorf("node management is only supported for the k3s-docker engine, got %q", e.engine)
	}

	// Remove engine scheduling constraints from Helm charts before removing the container.
	for _, scope := range scopes {
		if scope == "engine" {
			contextName := e.K3sDockerContextName()
			restConfig, err := config.GetConfigWithContext(contextName)
			if err != nil {
				return fmt.Errorf("failed to get kubeconfig for environment %q: %w", e.name, err)
			}
			if err := e.removeEngineScope(ctx, restConfig, logger); err != nil {
				logger.Warnf("Failed to remove engine scope from Helm charts: %v", err)
			}
		}
	}

	dockerClient, err := docker.NewClientWithOpts(docker.FromEnv, docker.WithAPIVersionNegotiation())
	if err != nil {
		return fmt.Errorf("failed to create Docker client: %w", err)
	}
	defer dockerClient.Close()

	agentContainerName := e.nodeContainerName(nodeName)
	c, err := e.findK3sDockerContainer(ctx, dockerClient, agentContainerName)
	if err != nil {
		return err
	}
	if c == nil {
		logger.Infof("Node container %q not found, nothing to delete.", agentContainerName)
		return nil
	}

	timeout := 10
	if err := dockerClient.ContainerStop(ctx, c.ID, container.StopOptions{Timeout: &timeout}); err != nil {
		logger.Warnf("Failed to stop node container %s: %v", c.ID, err)
	}
	if err := dockerClient.ContainerRemove(ctx, c.ID, types.ContainerRemoveOptions{Force: true}); err != nil {
		return fmt.Errorf("failed to remove node container: %w", err)
	}

	logger.Infof("Node %q deleted successfully.", nodeName)
	return nil
}

// getK3sToken reads the K3s node join token from inside the server container.
func (e *Environment) getK3sToken(ctx context.Context, dockerClient *docker.Client, containerID string) (string, error) {
	execID, err := dockerClient.ContainerExecCreate(ctx, containerID, types.ExecConfig{
		Cmd:          []string{"cat", "/var/lib/rancher/k3s/server/node-token"},
		AttachStdout: true,
		AttachStderr: true,
	})
	if err != nil {
		return "", fmt.Errorf("failed to create exec: %w", err)
	}

	attachResp, err := dockerClient.ContainerExecAttach(ctx, execID.ID, types.ExecStartCheck{})
	if err != nil {
		return "", fmt.Errorf("failed to attach to exec: %w", err)
	}
	defer attachResp.Close()

	var stdout, stderr bytes.Buffer
	if _, err := stdcopy.StdCopy(&stdout, &stderr, attachResp.Reader); err != nil {
		return "", fmt.Errorf("failed to read exec output: %w", err)
	}

	token := strings.TrimSpace(stdout.String())
	if token == "" {
		return "", fmt.Errorf("K3s token is empty; server may not be fully initialised yet")
	}
	return token, nil
}

// waitForNodeReady polls the Kubernetes API until the named node has the Ready
// condition set to True.
func (e *Environment) waitForNodeReady(ctx context.Context, kubeClient *kubernetes.Clientset, nodeName string, logger *zap.SugaredLogger) error {
	timeoutTimer := time.NewTimer(k3sReadinessTimeout)
	defer timeoutTimer.Stop()
	ticker := time.NewTicker(k3sReadinessPollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timeoutTimer.C:
			return fmt.Errorf("timeout waiting for node %q to become ready after %s", nodeName, k3sReadinessTimeout)
		case <-ticker.C:
			node, err := kubeClient.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
			if err != nil {
				logger.Debugf("Waiting for node %q to appear: %v", nodeName, err)
				continue
			}
			for _, cond := range node.Status.Conditions {
				if cond.Type == corev1.NodeReady && cond.Status == corev1.ConditionTrue {
					logger.Infof("Node %q is ready.", nodeName)
					return nil
				}
			}
			logger.Debugf("Waiting for node %q to be ready...", nodeName)
		}
	}
}

// applyEngineScope labels and taints the node for engine-only workloads, then
// updates Crossplane, Kyverno, and cert-manager Helm releases to schedule onto
// that node via nodeSelector and tolerations.
func (e *Environment) applyEngineScope(ctx context.Context, kubeClient *kubernetes.Clientset, restConfig *rest.Config, nodeName string, logger *zap.SugaredLogger) error {
	node, err := kubeClient.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get node %q: %w", nodeName, err)
	}

	// Add label so nodeSelector can target this node.
	if node.Labels == nil {
		node.Labels = make(map[string]string)
	}
	node.Labels[engineScopeLabel] = engineScopeValue

	// Add taint so only pods with a matching toleration are scheduled here.
	taint := corev1.Taint{
		Key:    engineScopeLabel,
		Value:  engineScopeValue,
		Effect: corev1.TaintEffectNoSchedule,
	}
	alreadyTainted := false
	for _, t := range node.Spec.Taints {
		if t.Key == taint.Key && t.Value == taint.Value && t.Effect == taint.Effect {
			alreadyTainted = true
			break
		}
	}
	if !alreadyTainted {
		node.Spec.Taints = append(node.Spec.Taints, taint)
	}

	if _, err := kubeClient.CoreV1().Nodes().Update(ctx, node, metav1.UpdateOptions{}); err != nil {
		return fmt.Errorf("failed to update node %q: %w", nodeName, err)
	}
	logger.Infof("Applied label and taint %s=%s:%s to node %q.", engineScopeLabel, engineScopeValue, engineScopeEffect, nodeName)

	nodeSelector := map[string]interface{}{
		engineScopeLabel: engineScopeValue,
	}
	tolerations := []interface{}{
		map[string]interface{}{
			"key":      engineScopeLabel,
			"operator": "Equal",
			"value":    engineScopeValue,
			"effect":   engineScopeEffect,
		},
	}

	return e.applyNodeScopeToCharts(restConfig, nodeSelector, tolerations, logger)
}

// removeEngineScope clears nodeSelector and tolerations from the engine-related
// Helm releases so workloads fall back to default scheduling.
func (e *Environment) removeEngineScope(ctx context.Context, restConfig *rest.Config, logger *zap.SugaredLogger) error {
	for _, ch := range engineCharts() {
		// Retrieve current release to get its existing user-provided values.
		readMgr, err := nodeHelmManager(restConfig, ch.name, ch.repoURL, ch.relName, ch.namespace, true)
		if err != nil {
			return fmt.Errorf("failed to create Helm manager for %q: %w", ch.relName, err)
		}
		rel, err := readMgr.GetRelease()
		if err != nil {
			logger.Warnf("Could not find release %q, skipping: %v", ch.relName, err)
			continue
		}

		// Build a clean config without the engine-scope scheduling keys.
		cleanCfg := make(map[string]any, len(rel.Config))
		for k, v := range rel.Config {
			cleanCfg[k] = v
		}
		delete(cleanCfg, "nodeSelector")
		delete(cleanCfg, "tolerations")

		version := rel.Chart.Metadata.Version

		// Upgrade without reuseValues so the old nodeSelector/tolerations are
		// not merged back from the stored release state.
		upgMgr, err := nodeHelmManager(restConfig, ch.name, ch.repoURL, ch.relName, ch.namespace, false)
		if err != nil {
			return fmt.Errorf("failed to create Helm manager for %q: %w", ch.relName, err)
		}
		if err := upgMgr.Upgrade(version, cleanCfg); err != nil {
			return fmt.Errorf("failed to upgrade %q: %w", ch.relName, err)
		}
		logger.Infof("Removed node scope from %q Helm release.", ch.relName)
	}
	return nil
}

// applyNodeScopeToCharts upgrades the engine Helm releases with the provided
// nodeSelector and tolerations, merging them on top of existing values.
func (e *Environment) applyNodeScopeToCharts(restConfig *rest.Config, nodeSelector map[string]interface{}, tolerations []interface{}, logger *zap.SugaredLogger) error {
	scopeParams := map[string]any{
		"nodeSelector": nodeSelector,
		"tolerations":  tolerations,
	}

	for _, ch := range engineCharts() {
		mgr, err := nodeHelmManager(restConfig, ch.name, ch.repoURL, ch.relName, ch.namespace, true)
		if err != nil {
			return fmt.Errorf("failed to create Helm manager for %q: %w", ch.relName, err)
		}
		version, err := mgr.GetCurrentVersion()
		if err != nil {
			return fmt.Errorf("failed to get current version for %q: %w", ch.relName, err)
		}
		if err := mgr.Upgrade(version, scopeParams); err != nil {
			return fmt.Errorf("failed to upgrade %q with node scope values: %w", ch.relName, err)
		}
		logger.Infof("Updated %q Helm release with node scope values.", ch.relName)
	}
	return nil
}

// nodeHelmManager creates a Helm manager for upgrading an existing release.
// When reuseValues is true, existing chart values are preserved and the
// supplied parameters are merged on top.
func nodeHelmManager(restConfig *rest.Config, chartName, repoURLStr, releaseName, ns string, reuseValues bool) (install.Manager, error) {
	repoURL, err := url.Parse(repoURLStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse repo URL %q: %w", repoURLStr, err)
	}

	return helm.NewManager(
		restConfig,
		chartName,
		repoURL,
		releaseName,
		helm.InstallerModifierFn(helm.Wait()),
		helm.InstallerModifierFn(helm.WithNamespace(ns)),
		helm.InstallerModifierFn(helm.WithReuseValues(reuseValues)),
		helm.InstallerModifierFn(helm.WithUpgradeInstall(false)),
		helm.InstallerModifierFn(helm.WithCreateNamespace(false)),
	)
}
