package environment

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	docker "github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

const (
	scopeLabel     = "overlock.io/scope"
	nodeLabel      = "overlock.io/node"
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
	defer func() {
		if cerr := dockerClient.Close(); cerr != nil {
			logger.Warnf("Failed to close Docker client: %v", cerr)
		}
	}()

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

	contextName := e.K3sDockerContextName()
	restConfig, err := config.GetConfigWithContext(contextName)
	if err != nil {
		return fmt.Errorf("failed to get kubeconfig for environment %q: %w", e.name, err)
	}
	kubeClient, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	// Prevent name conflicts: reject if a node with this name already exists.
	existingNode, _ := findNodeByLabel(ctx, kubeClient, nodeName)
	if existingNode != "" {
		return fmt.Errorf("node %q already exists; use a different name", nodeName)
	}

	// The Kubernetes node name prefix follows the pattern <environment>-<nodeName>.
	// --with-node-id appends a random suffix so each container gets a unique
	// name in K3s's password store, avoiding conflicts on retries.
	k3sNodeName := e.name + "-" + nodeName
	agentContainerName := e.nodeContainerName(nodeName)

	if remote != nil {
		if err := e.createRemoteNode(ctx, dockerClient, serverContainer.ID, remote, agentContainerName, k3sNodeName, nodeName, token, scopes, logger); err != nil {
			return err
		}
	} else {
		if err := e.createLocalNode(ctx, dockerClient, serverContainer.ID, agentContainerName, k3sNodeName, nodeName, token, scopes, logger); err != nil {
			return err
		}
	}

	// Discover the node by the overlock.io/node label since --with-node-id
	// appends a random suffix to the actual Kubernetes node name.
	actualNodeName, err := e.waitForNodeReadyByLabel(ctx, kubeClient, nodeName, logger)
	if err != nil {
		return err
	}

	// Store SSH connection info as annotations for remote node cleanup on env delete.
	if remote != nil {
		if err := annotateRemoteNode(ctx, kubeClient, actualNodeName, remote); err != nil {
			logger.Warnf("Failed to annotate remote node %q: %v", actualNodeName, err)
		}
	}

	// Find and delete previous nodes that had the same scope.
	e.replaceScopedNodes(ctx, kubeClient, scopes, actualNodeName, logger)

	logger.Infof("Node %q created successfully.", nodeName)
	return nil
}

// replaceScopedNodes finds and deletes previous nodes that had the same scope.
// The new node already has scope labels/taints via K3s agent flags,
// so pods migrate automatically via label selectors.
func (e *Environment) replaceScopedNodes(ctx context.Context, kubeClient *kubernetes.Clientset, scopes []string, actualNodeName string, logger *zap.SugaredLogger) {
	for _, scope := range scopes {
		oldNodes, err := findNodesWithScope(ctx, kubeClient, scope)
		if err != nil {
			continue
		}
		for i := range oldNodes {
			oldNode := &oldNodes[i]
			if oldNode.Name == actualNodeName {
				continue
			}
			logger.Debugf("Replacing old %s-scoped node %q...", scope, oldNode.Name)

			oldRemote := remoteFromNodeAnnotations(ctx, kubeClient, oldNode.Name, logger)
			if oldRemote != nil {
				defer oldRemote.Close()
			}

			if err := e.deleteNodeByName(ctx, kubeClient, oldNode.Name, logger); err != nil {
				logger.Warnf("Failed to delete old node %q from cluster: %v", oldNode.Name, err)
			}

			oldShortName := oldNode.Labels[nodeLabel]
			if oldShortName == "" {
				oldShortName = strings.TrimPrefix(oldNode.Name, e.name+"-")
			}
			agentContainer := e.nodeContainerName(oldShortName)
			if oldRemote != nil {
				if err := e.deleteRemoteNode(oldRemote, agentContainer, logger); err != nil {
					logger.Warnf("Failed to delete remote container %q: %v", agentContainer, err)
				}
			} else {
				if err := e.deleteLocalNode(ctx, agentContainer, logger); err != nil {
					logger.Warnf("Failed to delete local container %q: %v", agentContainer, err)
				}
			}
		}
	}
}

// createLocalNode creates a K3s agent container on the local Docker daemon.
// Each agent uses default bridge networking so it gets its own network
// namespace with no port conflicts. The server's --egress-selector-mode=pod
// allows it to reach agent pods through the agent tunnel.
func (e *Environment) createLocalNode(ctx context.Context, dockerClient *docker.Client, _, agentContainerName, k3sNodeName, nodeName, token string, scopes []string, logger *zap.SugaredLogger) error {
	// Remove existing container so a fresh node is always created.
	existing, err := e.findK3sDockerContainer(ctx, dockerClient, agentContainerName)
	if err != nil {
		return err
	}
	if existing != nil {
		logger.Debugf("Removing existing node container %q to create a fresh node.", agentContainerName)
		timeout := 10
		if err := dockerClient.ContainerStop(ctx, existing.ID, container.StopOptions{Timeout: &timeout}); err != nil {
			logger.Warnf("Failed to stop existing node container: %v", err)
		}
		if err := dockerClient.ContainerRemove(ctx, existing.ID, types.ContainerRemoveOptions{Force: true}); err != nil {
			return fmt.Errorf("failed to remove existing node container: %w", err)
		}
		volumeName := agentContainerName + "-data"
		if err := dockerClient.VolumeRemove(ctx, volumeName, true); err != nil {
			logger.Warnf("Failed to remove volume %s: %v", volumeName, err)
		}
	}

	// The server uses host networking; local agents reach it via the host IP.
	hostIP, err := localIP()
	if err != nil {
		return fmt.Errorf("failed to determine host IP: %w", err)
	}

	agentCmd := []string{"agent", "--with-node-id", "--node-name", k3sNodeName,
		"--node-label", fmt.Sprintf("%s=%s", nodeLabel, nodeName)}
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

	// Use a named volume for the K3s data directory to avoid the
	// overlayfs-on-overlayfs problem that crashes containerd.
	volumeName := agentContainerName + "-data"

	nanoCPUs, err := parseCPU(e.cpu)
	if err != nil {
		return fmt.Errorf("invalid --cpu value: %w", err)
	}

	hostConfig := &container.HostConfig{
		Privileged: true,
		Binds: []string{
			"/lib/modules:/lib/modules:ro",
			volumeName + ":/var/lib/rancher/k3s",
		},
		Tmpfs: map[string]string{
			"/run":     "",
			"/var/run": "",
		},
		Resources: container.Resources{
			NanoCPUs: nanoCPUs,
		},
	}

	if e.mountPath != "" {
		hostConfig.Binds = append(hostConfig.Binds, e.mountPath+":"+e.containerPath)
	}

	// Pull the image (likely already cached from environment creation).
	logger.Debugf("Pulling image %s...", k3sDockerImage)
	pullReader, err := dockerClient.ImagePull(ctx, k3sDockerImage, types.ImagePullOptions{})
	if err != nil {
		return fmt.Errorf("failed to pull image %s: %w", k3sDockerImage, err)
	}
	if _, err := io.Copy(io.Discard, pullReader); err != nil {
		logger.Warnf("Failed to drain pull reader: %v", err)
	}
	if err := pullReader.Close(); err != nil {
		logger.Warnf("Failed to close pull reader: %v", err)
	}

	resp, err := dockerClient.ContainerCreate(ctx, containerConfig, hostConfig, nil, nil, agentContainerName)
	if err != nil {
		return fmt.Errorf("failed to create node container: %w", err)
	}

	if err := dockerClient.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{}); err != nil {
		if rerr := dockerClient.ContainerRemove(ctx, resp.ID, types.ContainerRemoveOptions{Force: true}); rerr != nil {
			logger.Warnf("Failed to remove container after start failure: %v", rerr)
		}
		return fmt.Errorf("failed to start node container: %w", err)
	}

	logger.Debugf("Node container %q started, waiting for node to join the cluster...", agentContainerName)
	return nil
}

// createRemoteNode creates a K3s agent container on a remote host via SSH.
func (e *Environment) createRemoteNode(_ context.Context, _ *docker.Client, _ string, remote *SSHClient, agentContainerName, k3sNodeName, nodeName, token string, scopes []string, logger *zap.SugaredLogger) error {
	// Determine the local IP reachable from the remote host.
	localIP, err := remote.LocalIPFor()
	if err != nil {
		return fmt.Errorf("failed to determine local IP: %w", err)
	}

	// With host networking on the server, the API server listens on port 6443
	// directly on the host IP.
	k3sURL := fmt.Sprintf("https://%s:6443", localIP)
	logger.Debugf("Remote node will connect to K3s server at %s", k3sURL)

	// Remove existing container so a fresh node is always created.
	checkCmd := fmt.Sprintf("docker inspect %s >/dev/null 2>&1 && echo exists || echo missing", agentContainerName)
	checkOut, err := remote.Run(checkCmd)
	if err != nil {
		logger.Debugf("Failed to check remote container: %v", err)
	}
	if strings.TrimSpace(checkOut) == "exists" {
		logger.Debugf("Removing existing node container %q on remote host to create a fresh node.", agentContainerName)
		if _, err := remote.Run(fmt.Sprintf("docker stop %s", agentContainerName)); err != nil {
			logger.Warnf("Failed to stop existing remote container: %v", err)
		}
		if _, err := remote.Run(fmt.Sprintf("docker rm %s", agentContainerName)); err != nil {
			return fmt.Errorf("failed to remove existing remote container: %w", err)
		}
		volumeName := agentContainerName + "-data"
		if _, err := remote.Run(fmt.Sprintf("docker volume rm %s", volumeName)); err != nil {
			logger.Warnf("Failed to remove remote volume %s: %v", volumeName, err)
		}
	}

	// Run the K3s agent container on the remote host using default bridge
	// networking. Multiple agents can run on the same host without port
	// conflicts. The server's --egress-selector-mode=pod routes traffic
	// to pods through each agent's tunnel.
	scopeFlags := ""
	for _, scope := range scopes {
		scopeFlags += fmt.Sprintf(" --node-label %s=%s", scopeLabel, scope)
		if scope == scopeEngine {
			scopeFlags += fmt.Sprintf(" --node-taint %s=%s:%s", scopeLabel, scope, scopeEffect)
		}
	}

	cpuFlag := ""
	if e.cpu != "" && e.cpu != "0" {
		nanoCPUs, err := parseCPU(e.cpu)
		if err != nil {
			return fmt.Errorf("invalid --cpu value: %w", err)
		}
		if nanoCPUs > 0 {
			cpuFlag = fmt.Sprintf(" --cpus %g", float64(nanoCPUs)/1e9)
		}
	}

	volumeName := agentContainerName + "-data"
	dockerRunCmd := fmt.Sprintf(
		"docker run -d --privileged --name %s -v /lib/modules:/lib/modules:ro -v %s:/var/lib/rancher/k3s --tmpfs /run --tmpfs /var/run -e K3S_URL=%s -e K3S_TOKEN=%s%s %s agent --with-node-id --node-name %s --node-label %s=%s --node-external-ip %s%s",
		agentContainerName, volumeName, k3sURL, token, cpuFlag, k3sDockerImage, k3sNodeName, nodeLabel, nodeName, remote.Host, scopeFlags,
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
	// Find the actual node name by label since --with-node-id appends a random suffix.
	contextName := e.K3sDockerContextName()
	restConfig, err := config.GetConfigWithContext(contextName)
	if err != nil {
		logger.Warnf("Failed to get kubeconfig, skipping node drain: %v", err)
	} else {
		kubeClient, err := kubernetes.NewForConfig(restConfig)
		if err != nil {
			logger.Warnf("Failed to create Kubernetes client, skipping node drain: %v", err)
		} else {
			actualName, err := findNodeByLabel(ctx, kubeClient, nodeName)
			if err != nil {
				logger.Warnf("Failed to find node with label %s=%s: %v", nodeLabel, nodeName, err)
			} else {
				// If remote is not provided, check node annotations for SSH details.
				if remote == nil {
					remote = remoteFromNodeAnnotations(ctx, kubeClient, actualName, logger)
					if remote != nil {
						defer remote.Close()
					}
				}

				if err := e.drainNode(ctx, kubeClient, actualName, logger); err != nil {
					logger.Warnf("Failed to drain node %q: %v", actualName, err)
				}
				if err := kubeClient.CoreV1().Nodes().Delete(ctx, actualName, metav1.DeleteOptions{}); err != nil {
					logger.Warnf("Failed to delete node %q from cluster: %v", actualName, err)
				} else {
					logger.Debugf("Node %q removed from cluster.", actualName)
				}
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
	defer func() {
		if cerr := dockerClient.Close(); cerr != nil {
			logger.Warnf("Failed to close Docker client: %v", cerr)
		}
	}()

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

	// Remove the named volume used for the K3s data directory.
	volumeName := agentContainerName + "-data"
	if err := dockerClient.VolumeRemove(ctx, volumeName, true); err != nil {
		logger.Warnf("Failed to remove volume %s: %v", volumeName, err)
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

	// Remove the named volume used for the K3s data directory.
	volumeName := agentContainerName + "-data"
	if _, err := remote.Run(fmt.Sprintf("docker volume rm %s", volumeName)); err != nil {
		logger.Warnf("Failed to remove remote volume %s: %v", volumeName, err)
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

// findNodesWithScope returns all nodes that have the given scope label.
func findNodesWithScope(ctx context.Context, kubeClient *kubernetes.Clientset, scope string) ([]corev1.Node, error) {
	nodes, err := kubeClient.CoreV1().Nodes().List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", scopeLabel, scope),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list nodes with scope %q: %w", scope, err)
	}
	return nodes.Items, nil
}

// remoteFromNodeAnnotations builds an SSHClient from the node's SSH annotations.
// Returns nil if the node has no SSH host annotation (i.e. it's a local node).
func remoteFromNodeAnnotations(ctx context.Context, kubeClient *kubernetes.Clientset, nodeName string, logger *zap.SugaredLogger) *SSHClient {
	node, err := kubeClient.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
	if err != nil {
		return nil
	}
	host := node.Annotations[annSSHHost]
	if host == "" {
		return nil
	}
	user := node.Annotations[annSSHUser]
	if user == "" {
		user = "root"
	}
	port := 22
	if p, err := strconv.Atoi(node.Annotations[annSSHPort]); err == nil && p > 0 {
		port = p
	}
	key := node.Annotations[annSSHKey]
	remote, err := NewSSHClient(host, user, port, key)
	if err != nil {
		logger.Warnf("Failed to connect to remote host %s for node %q cleanup: %v", host, nodeName, err)
		return nil
	}
	return remote
}

// annotateRemoteNode stores SSH connection info as annotations on the node
// so that env delete can discover and clean up remote containers.
func annotateRemoteNode(ctx context.Context, kubeClient *kubernetes.Clientset, nodeName string, remote *SSHClient) error {
	node, err := kubeClient.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get node %q: %w", nodeName, err)
	}
	if node.Annotations == nil {
		node.Annotations = make(map[string]string)
	}
	node.Annotations[annSSHHost] = remote.Host
	node.Annotations[annSSHUser] = remote.User
	node.Annotations[annSSHPort] = fmt.Sprintf("%d", remote.Port)
	node.Annotations[annSSHKey] = remote.Key
	if _, err = kubeClient.CoreV1().Nodes().Update(ctx, node, metav1.UpdateOptions{}); err != nil {
		return fmt.Errorf("failed to update node %q annotations: %w", nodeName, err)
	}
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

// waitForNodeReadyByLabel polls the Kubernetes API until a node with the
// overlock.io/node=<nodeName> label appears and has the Ready condition.
// Returns the actual Kubernetes node name (which includes the random suffix).
func (e *Environment) waitForNodeReadyByLabel(ctx context.Context, kubeClient *kubernetes.Clientset, nodeName string, logger *zap.SugaredLogger) (string, error) {
	labelSelector := fmt.Sprintf("%s=%s", nodeLabel, nodeName)
	timeoutTimer := time.NewTimer(k3sReadinessTimeout)
	defer timeoutTimer.Stop()
	ticker := time.NewTicker(k3sReadinessPollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return "", fmt.Errorf("context cancelled while waiting for node: %w", ctx.Err())
		case <-timeoutTimer.C:
			return "", fmt.Errorf("timeout waiting for node with label %s to become ready after %s", labelSelector, k3sReadinessTimeout)
		case <-ticker.C:
			nodes, err := kubeClient.CoreV1().Nodes().List(ctx, metav1.ListOptions{
				LabelSelector: labelSelector,
			})
			if err != nil || len(nodes.Items) == 0 {
				logger.Debugf("Waiting for node with label %s to appear...", labelSelector)
				continue
			}
			node := &nodes.Items[0]
			for _, cond := range node.Status.Conditions {
				if cond.Type == corev1.NodeReady && cond.Status == corev1.ConditionTrue {
					logger.Debugf("Node %q is ready.", node.Name)
					return node.Name, nil
				}
			}
			logger.Debugf("Waiting for node %q to be ready...", node.Name)
		}
	}
}

// findNodeByLabel returns the actual Kubernetes node name for a node with
// the overlock.io/node=<nodeName> label.
func findNodeByLabel(ctx context.Context, kubeClient *kubernetes.Clientset, nodeName string) (string, error) {
	nodes, err := kubeClient.CoreV1().Nodes().List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", nodeLabel, nodeName),
	})
	if err != nil {
		return "", fmt.Errorf("failed to list nodes with label %s=%s: %w", nodeLabel, nodeName, err)
	}
	if len(nodes.Items) == 0 {
		return "", fmt.Errorf("no node found with label %s=%s", nodeLabel, nodeName)
	}
	return nodes.Items[0].Name, nil
}

// deleteNodeByName drains and deletes a Kubernetes node by its actual name.
func (e *Environment) deleteNodeByName(ctx context.Context, kubeClient *kubernetes.Clientset, actualName string, logger *zap.SugaredLogger) error {
	if err := e.drainNode(ctx, kubeClient, actualName, logger); err != nil {
		logger.Warnf("Failed to drain node %q: %v", actualName, err)
	}
	if err := kubeClient.CoreV1().Nodes().Delete(ctx, actualName, metav1.DeleteOptions{}); err != nil {
		return fmt.Errorf("failed to delete node %q from cluster: %w", actualName, err)
	}
	logger.Debugf("Node %q removed from cluster.", actualName)
	return nil
}
