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
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	policyv1 "k8s.io/api/policy/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

const (
	scopeLabel     = "overlock.io/scope"
	scopeEngine    = "engine"
	scopeWorkloads = "workloads"
	scopeEffect    = "NoSchedule"

	annSSHHost = "overlock.io/ssh-host"
	annSSHUser = "overlock.io/ssh-user"
	annSSHPort = "overlock.io/ssh-port"
	annSSHKey  = "overlock.io/ssh-key"
)

// nodeContainerName returns the Docker container name for an agent node.
// Pattern: <k3s-docker-prefix><environment>-<nodeName>
func (e *Environment) nodeContainerName(nodeName string) string {
	return k3sDockerContainerPrefix + e.name + "-" + nodeName
}

// CreateNode creates a new K3s agent node as a Docker container that joins the
// existing K3s server for this environment. Only supported for the k3s-docker engine.
// When remote is non-nil, the Docker container is created on the remote host via SSH.
func (e *Environment) CreateNode(ctx context.Context, nodeName string, scopes []string, remote *SSHClient, logger *zap.SugaredLogger) error {
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

	// Retrieve the K3s node join token from the server container.
	token, err := e.getK3sToken(ctx, dockerClient, serverContainer.ID)
	if err != nil {
		return fmt.Errorf("failed to get K3s token: %w", err)
	}

	// The Kubernetes node name follows the pattern <environment>-<nodeName>.
	k3sNodeName := e.name + "-" + nodeName
	agentContainerName := e.nodeContainerName(nodeName)

	if remote != nil {
		if err := e.createRemoteNode(ctx, dockerClient, serverContainer.ID, remote, agentContainerName, k3sNodeName, token, scopes, logger); err != nil {
			return err
		}
	} else {
		if err := e.createLocalNode(ctx, dockerClient, serverContainer.ID, agentContainerName, k3sNodeName, token, scopes, logger); err != nil {
			return err
		}
	}

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

	// Store SSH connection info as annotations for remote node cleanup on env delete.
	if remote != nil {
		if err := annotateRemoteNode(ctx, kubeClient, k3sNodeName, remote); err != nil {
			logger.Warnf("Failed to annotate remote node %q: %v", k3sNodeName, err)
		}
	}

	// Find and delete previous nodes that had the same scope.
	// The new node already has scope labels/taints via K3s agent flags,
	// so pods migrate automatically via label selectors.
	for _, scope := range scopes {
		oldNode, err := findNodeWithScope(ctx, kubeClient, scope)
		if err == nil && oldNode != k3sNodeName {
			oldShortName := strings.TrimPrefix(oldNode, e.name+"-")
			logger.Debugf("Replacing old %s-scoped node %q...", scope, oldNode)
			if err := e.DeleteNode(ctx, oldShortName, nil, nil, logger); err != nil {
				logger.Warnf("Failed to delete old %s-scoped node %q: %v", scope, oldNode, err)
			}
		}
	}

	logger.Infof("Node %q created successfully.", nodeName)
	return nil
}

// createLocalNode creates a K3s agent container on the local Docker daemon.
// Each agent uses default bridge networking so it gets its own network
// namespace with no port conflicts. The server's --egress-selector-mode=pod
// allows it to reach agent pods through the agent tunnel.
func (e *Environment) createLocalNode(ctx context.Context, dockerClient *docker.Client, serverContainerID, agentContainerName, k3sNodeName, token string, scopes []string, logger *zap.SugaredLogger) error {
	// Check if the agent container already exists.
	existing, err := e.findK3sDockerContainer(ctx, dockerClient, agentContainerName)
	if err != nil {
		return err
	}
	if existing != nil {
		logger.Debugf("Node container %q already exists.", agentContainerName)
		if existing.State != "running" {
			if err := dockerClient.ContainerStart(ctx, existing.ID, types.ContainerStartOptions{}); err != nil {
				return fmt.Errorf("failed to start existing node container: %w", err)
			}
		}
		return nil
	}

	// The server uses host networking; local agents reach it via the host IP.
	hostIP, err := localIP()
	if err != nil {
		return fmt.Errorf("failed to determine host IP: %w", err)
	}

	agentCmd := []string{"agent", "--node-name", k3sNodeName}
	for _, scope := range scopes {
		agentCmd = append(agentCmd, "--node-label", fmt.Sprintf("%s=%s", scopeLabel, scope))
		// Only taint the engine node; workloads node stays open so
		// kube-system pods (CoreDNS, metrics-server, etc.) can schedule there.
		if scope == scopeEngine {
			agentCmd = append(agentCmd, "--node-taint", fmt.Sprintf("%s=%s:%s", scopeLabel, scope, scopeEffect))
		}
	}

	containerConfig := &container.Config{
		Image: k3sDockerImage,
		Cmd:   agentCmd,
		Env: []string{
			fmt.Sprintf("K3S_URL=https://%s:6443", hostIP),
			fmt.Sprintf("K3S_TOKEN=%s", token),
		},
		Labels: map[string]string{
			"app.kubernetes.io/managed-by": "overlock",
		},
	}

	hostConfig := &container.HostConfig{
		Privileged: true,
		Binds: []string{
			"/lib/modules:/lib/modules:ro",
		},
		Tmpfs: map[string]string{
			"/run":     "",
			"/var/run": "",
		},
	}

	// Pull the image (likely already cached from environment creation).
	logger.Debugf("Pulling image %s...", k3sDockerImage)
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

	logger.Debugf("Node container %q started, waiting for node to join the cluster...", agentContainerName)
	return nil
}

// createRemoteNode creates a K3s agent container on a remote host via SSH.
func (e *Environment) createRemoteNode(ctx context.Context, dockerClient *docker.Client, serverContainerID string, remote *SSHClient, agentContainerName, k3sNodeName, token string, scopes []string, logger *zap.SugaredLogger) error {
	// Determine the local IP reachable from the remote host.
	localIP, err := remote.LocalIPFor()
	if err != nil {
		return fmt.Errorf("failed to determine local IP: %w", err)
	}

	// With host networking on the server, the API server listens on port 6443
	// directly on the host IP.
	k3sURL := fmt.Sprintf("https://%s:6443", localIP)
	logger.Debugf("Remote node will connect to K3s server at %s", k3sURL)

	// Check if container already exists on remote host.
	checkCmd := fmt.Sprintf("docker inspect %s >/dev/null 2>&1 && echo exists || echo missing", agentContainerName)
	checkOut, _ := remote.Run(checkCmd)
	if strings.TrimSpace(checkOut) == "exists" {
		logger.Debugf("Node container %q already exists on remote host.", agentContainerName)
		_, _ = remote.Run(fmt.Sprintf("docker start %s", agentContainerName))
		return nil
	}

	// Run the K3s agent container on the remote host with host networking
	// so the agent's tunnel and kubelet bind to the host's real IP.
	scopeFlags := ""
	for _, scope := range scopes {
		scopeFlags += fmt.Sprintf(" --node-label %s=%s", scopeLabel, scope)
		if scope == scopeEngine {
			scopeFlags += fmt.Sprintf(" --node-taint %s=%s:%s", scopeLabel, scope, scopeEffect)
		}
	}
	dockerRunCmd := fmt.Sprintf(
		"docker run -d --privileged --network host --name %s -v /lib/modules:/lib/modules:ro --tmpfs /run --tmpfs /var/run -e K3S_URL=%s -e K3S_TOKEN=%s %s agent --node-name %s --node-external-ip %s --node-ip %s%s",
		agentContainerName, k3sURL, token, k3sDockerImage, k3sNodeName, remote.Host, remote.Host, scopeFlags,
	)

	logger.Debugf("Creating node container %q on remote host %s...", agentContainerName, remote.Host)
	if _, err := remote.Run(dockerRunCmd); err != nil {
		return fmt.Errorf("failed to create remote node container: %w", err)
	}

	logger.Debugf("Node container %q started on remote host, waiting for node to join the cluster...", agentContainerName)
	return nil
}

// DeleteNode stops and removes the K3s agent node container. When the engine
// scope was applied, this also clears nodeSelector and tolerations from
// engine-related Helm charts. Only supported for the k3s-docker engine.
// When remote is non-nil, the Docker container is removed on the remote host via SSH.
func (e *Environment) DeleteNode(ctx context.Context, nodeName string, scopes []string, remote *SSHClient, logger *zap.SugaredLogger) error {
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
			if err := e.drainNode(ctx, kubeClient, k3sNodeName, logger); err != nil {
				logger.Warnf("Failed to drain node %q: %v", k3sNodeName, err)
			}
			if err := kubeClient.CoreV1().Nodes().Delete(ctx, k3sNodeName, metav1.DeleteOptions{}); err != nil {
				logger.Warnf("Failed to delete node %q from cluster: %v", k3sNodeName, err)
			} else {
				logger.Debugf("Node %q removed from cluster.", k3sNodeName)
			}
		}
	}

	agentContainerName := e.nodeContainerName(nodeName)

	if remote != nil {
		return e.deleteRemoteNode(remote, agentContainerName, logger)
	}
	return e.deleteLocalNode(ctx, agentContainerName, logger)
}

// deleteLocalNode stops and removes a node container from the local Docker daemon.
func (e *Environment) deleteLocalNode(ctx context.Context, agentContainerName string, logger *zap.SugaredLogger) error {
	dockerClient, err := docker.NewClientWithOpts(docker.FromEnv, docker.WithAPIVersionNegotiation())
	if err != nil {
		return fmt.Errorf("failed to create Docker client: %w", err)
	}
	defer dockerClient.Close()

	c, err := e.findK3sDockerContainer(ctx, dockerClient, agentContainerName)
	if err != nil {
		return err
	}
	if c == nil {
		logger.Debugf("Node container %q not found, nothing to delete.", agentContainerName)
		return nil
	}

	timeout := 10
	if err := dockerClient.ContainerStop(ctx, c.ID, container.StopOptions{Timeout: &timeout}); err != nil {
		logger.Warnf("Failed to stop node container %s: %v", c.ID, err)
	}
	if err := dockerClient.ContainerRemove(ctx, c.ID, types.ContainerRemoveOptions{Force: true}); err != nil {
		return fmt.Errorf("failed to remove node container: %w", err)
	}

	logger.Debugf("Node container %q deleted successfully.", agentContainerName)
	return nil
}

// deleteRemoteNode stops and removes a node container on a remote host via SSH.
func (e *Environment) deleteRemoteNode(remote *SSHClient, agentContainerName string, logger *zap.SugaredLogger) error {
	logger.Debugf("Stopping node container %q on remote host %s...", agentContainerName, remote.Host)

	if _, err := remote.Run(fmt.Sprintf("docker stop %s", agentContainerName)); err != nil {
		logger.Warnf("Failed to stop remote node container: %v", err)
	}
	if _, err := remote.Run(fmt.Sprintf("docker rm %s", agentContainerName)); err != nil {
		return fmt.Errorf("failed to remove remote node container: %w", err)
	}

	logger.Debugf("Node container %q deleted from remote host.", agentContainerName)
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
	logger.Debugf("Node %q cordoned.", nodeName)

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
	logger.Debugf("Node %q drained.", nodeName)
	return nil
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

// findNodeWithScope returns the name of the first node that has the given scope label.
func findNodeWithScope(ctx context.Context, kubeClient *kubernetes.Clientset, scope string) (string, error) {
	nodes, err := kubeClient.CoreV1().Nodes().List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", scopeLabel, scope),
	})
	if err != nil {
		return "", fmt.Errorf("failed to list nodes with scope %q: %w", scope, err)
	}
	if len(nodes.Items) == 0 {
		return "", fmt.Errorf("no node found with scope %q", scope)
	}
	return nodes.Items[0].Name, nil
}

// annotateRemoteNode stores SSH connection info as annotations on the node
// so that env delete can discover and clean up remote containers.
func annotateRemoteNode(ctx context.Context, kubeClient *kubernetes.Clientset, nodeName string, remote *SSHClient) error {
	node, err := kubeClient.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	if node.Annotations == nil {
		node.Annotations = make(map[string]string)
	}
	node.Annotations[annSSHHost] = remote.Host
	node.Annotations[annSSHUser] = remote.User
	node.Annotations[annSSHPort] = fmt.Sprintf("%d", remote.Port)
	node.Annotations[annSSHKey] = remote.Key
	_, err = kubeClient.CoreV1().Nodes().Update(ctx, node, metav1.UpdateOptions{})
	return err
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
					logger.Debugf("Node %q is ready.", nodeName)
					return nil
				}
			}
			logger.Debugf("Waiting for node %q to be ready...", nodeName)
		}
	}
}

