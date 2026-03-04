package environment

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	docker "github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/web-seven/overlock/internal/chart"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	policyv1 "k8s.io/api/policy/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

const (
	engineScopeLabel  = "overlock.io/scope"
	engineScopeValue  = "engine"
	engineScopeEffect = "NoSchedule"
)

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

	// Drain and delete the Kubernetes node before removing the container.
	k3sNodeName := e.name + "-" + nodeName
	contextName := e.K3sDockerContextName()
	restConfig, err := config.GetConfigWithContext(contextName)
	if err != nil {
		logger.Warnf("Failed to get kubeconfig, skipping node drain: %v", err)
	} else {
		kubeClient, err := kubernetes.NewForConfig(restConfig)
		if err != nil {
			logger.Warnf("Failed to create Kubernetes client, skipping node drain: %v", err)
		} else {
			// Check if node has engine taint and remove engine scope from Helm charts.
			if e.nodeHasEngineTaint(ctx, kubeClient, k3sNodeName) {
				if err := e.removeEngineScope(ctx, kubeClient, restConfig, logger); err != nil {
					logger.Warnf("Failed to remove engine scope from Helm charts: %v", err)
				}
			}

			if err := e.drainNode(ctx, kubeClient, k3sNodeName, logger); err != nil {
				logger.Warnf("Failed to drain node %q: %v", k3sNodeName, err)
			}
			if err := kubeClient.CoreV1().Nodes().Delete(ctx, k3sNodeName, metav1.DeleteOptions{}); err != nil {
				logger.Warnf("Failed to delete node %q from cluster: %v", k3sNodeName, err)
			} else {
				logger.Infof("Node %q removed from cluster.", k3sNodeName)
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

// drainNode cordons the node and evicts all pods before deletion.
func (e *Environment) drainNode(ctx context.Context, kubeClient *kubernetes.Clientset, nodeName string, logger *zap.SugaredLogger) error {
	// Cordon the node.
	node, err := kubeClient.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get node %q: %w", nodeName, err)
	}
	node.Spec.Unschedulable = true
	if _, err := kubeClient.CoreV1().Nodes().Update(ctx, node, metav1.UpdateOptions{}); err != nil {
		return fmt.Errorf("failed to cordon node %q: %w", nodeName, err)
	}
	logger.Infof("Node %q cordoned.", nodeName)

	// List pods on the node.
	podList, err := kubeClient.CoreV1().Pods("").List(ctx, metav1.ListOptions{
		FieldSelector: fields.SelectorFromSet(fields.Set{"spec.nodeName": nodeName}).String(),
	})
	if err != nil {
		return fmt.Errorf("failed to list pods on node %q: %w", nodeName, err)
	}

	// Evict non-DaemonSet pods.
	for i := range podList.Items {
		pod := &podList.Items[i]
		if isDaemonSetPod(pod) {
			continue
		}
		eviction := &policyv1.Eviction{
			ObjectMeta: metav1.ObjectMeta{
				Name:      pod.Name,
				Namespace: pod.Namespace,
			},
		}
		if err := kubeClient.CoreV1().Pods(pod.Namespace).EvictV1(ctx, eviction); err != nil {
			logger.Warnf("Failed to evict pod %s/%s: %v", pod.Namespace, pod.Name, err)
		}
	}
	logger.Infof("Node %q drained.", nodeName)
	return nil
}

// nodeHasEngineTaint returns true if the node has the engine scope taint.
func (e *Environment) nodeHasEngineTaint(ctx context.Context, kubeClient *kubernetes.Clientset, nodeName string) bool {
	node, err := kubeClient.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
	if err != nil {
		return false
	}
	for _, t := range node.Spec.Taints {
		if t.Key == engineScopeLabel && t.Value == engineScopeValue && t.Effect == corev1.TaintEffectNoSchedule {
			return true
		}
	}
	return false
}

// isDaemonSetPod returns true if the pod is owned by a DaemonSet.
func isDaemonSetPod(pod *corev1.Pod) bool {
	for _, ref := range pod.OwnerReferences {
		if ref.Kind == "DaemonSet" {
			return true
		}
	}
	return false
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

// findMasterNode returns the name of the control-plane/master node.
func findMasterNode(ctx context.Context, kubeClient *kubernetes.Clientset) (string, error) {
	nodes, err := kubeClient.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to list nodes: %w", err)
	}
	for _, n := range nodes.Items {
		for k := range n.Labels {
			if k == "node-role.kubernetes.io/master" || k == "node-role.kubernetes.io/control-plane" {
				return n.Name, nil
			}
		}
	}
	return "", fmt.Errorf("no master node found")
}

// addEngineScopeToNode adds the engine scope label and taint to a node.
func addEngineScopeToNode(ctx context.Context, kubeClient *kubernetes.Clientset, nodeName string, logger *zap.SugaredLogger) error {
	node, err := kubeClient.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get node %q: %w", nodeName, err)
	}

	if node.Labels == nil {
		node.Labels = make(map[string]string)
	}
	node.Labels[engineScopeLabel] = engineScopeValue

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
	return nil
}

// removeEngineScopeFromNode removes the engine scope label and taint from a node.
func removeEngineScopeFromNode(ctx context.Context, kubeClient *kubernetes.Clientset, nodeName string, logger *zap.SugaredLogger) error {
	node, err := kubeClient.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get node %q: %w", nodeName, err)
	}

	delete(node.Labels, engineScopeLabel)

	var cleanTaints []corev1.Taint
	for _, t := range node.Spec.Taints {
		if t.Key == engineScopeLabel && t.Value == engineScopeValue && t.Effect == corev1.TaintEffectNoSchedule {
			continue
		}
		cleanTaints = append(cleanTaints, t)
	}
	node.Spec.Taints = cleanTaints

	if _, err := kubeClient.CoreV1().Nodes().Update(ctx, node, metav1.UpdateOptions{}); err != nil {
		return fmt.Errorf("failed to update node %q: %w", nodeName, err)
	}
	logger.Infof("Removed label and taint %s=%s:%s from node %q.", engineScopeLabel, engineScopeValue, engineScopeEffect, nodeName)
	return nil
}

// applyEngineScope moves the engine scope from the master node to the target
// node, then updates Helm releases and the default DeploymentRuntimeConfig
// to schedule engine workloads onto the target node.
func (e *Environment) applyEngineScope(ctx context.Context, kubeClient *kubernetes.Clientset, restConfig *rest.Config, nodeName string, logger *zap.SugaredLogger) error {
	// Remove engine scope from the master node.
	masterName, err := findMasterNode(ctx, kubeClient)
	if err != nil {
		logger.Warnf("Could not find master node: %v", err)
	} else if masterName != nodeName {
		if err := removeEngineScopeFromNode(ctx, kubeClient, masterName, logger); err != nil {
			logger.Warnf("Failed to remove engine scope from master node %q: %v", masterName, err)
		}
	}

	// Add engine scope to the target node.
	if err := addEngineScopeToNode(ctx, kubeClient, nodeName, logger); err != nil {
		return err
	}

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

	for _, ch := range chart.EngineCharts() {
		if err := ch.Apply(restConfig, nodeSelector, tolerations, logger); err != nil {
			return err
		}
	}
	return nil
}

// removeEngineScope clears engine scope from Helm releases and the default
// DeploymentRuntimeConfig, then restores the engine scope label on the master node.
func (e *Environment) removeEngineScope(ctx context.Context, kubeClient *kubernetes.Clientset, restConfig *rest.Config, logger *zap.SugaredLogger) error {
	for _, ch := range chart.EngineCharts() {
		if err := ch.Remove(restConfig, logger); err != nil {
			return err
		}
	}

	// Restore engine scope on the master node.
	masterName, err := findMasterNode(ctx, kubeClient)
	if err != nil {
		logger.Warnf("Could not find master node to restore engine scope: %v", err)
	} else {
		if err := addEngineScopeToNode(ctx, kubeClient, masterName, logger); err != nil {
			logger.Warnf("Failed to restore engine scope on master node %q: %v", masterName, err)
		}
	}
	return nil
}
