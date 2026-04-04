package environment

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
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

// formatTaint converts a user-provided "key:value" taint to K8s format "key=value:NoSchedule".
func formatTaint(taint string) string {
	taint = strings.Trim(taint, `"'`)
	parts := strings.SplitN(taint, ":", 2)
	if len(parts) == 2 {
		return fmt.Sprintf("%s=%s:%s", parts[0], parts[1], scopeEffect)
	}
	return fmt.Sprintf("%s:%s", taint, scopeEffect)
}

// formatLabel converts a user-provided "key:value" taint to a K8s label "key=value".
func formatLabel(taint string) string {
	taint = strings.Trim(taint, `"'`)
	parts := strings.SplitN(taint, ":", 2)
	if len(parts) == 2 {
		return fmt.Sprintf("%s=%s", parts[0], parts[1])
	}
	return taint
}

// nodeContainerName returns the Docker container name for an agent node.
// Pattern: <k3s-docker-prefix><environment>-<nodeName>
func (e *Environment) nodeContainerName(nodeName string) string {
	return k3sDockerContainerPrefix + e.name + "-" + nodeName
}

// CreateNode creates a new K3s agent node as a Docker container that joins the
// existing K3s server for this environment. Only supported for the k3s-docker engine.
// When remote is non-nil, the Docker container is created on the remote host via SSH.
func (e *Environment) CreateNode(ctx context.Context, nodeName string, scopes []string, taints []string, remote *SSHClient, logger *zap.SugaredLogger) error {
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

	// If a node with this name already exists, skip creation gracefully.
	existingNode, _ := findNodeByLabel(ctx, kubeClient, nodeName)
	if existingNode != "" {
		logger.Infof("Node %q already exists. Skipping creation.", nodeName)
		return nil
	}

	// The Kubernetes node name prefix follows the pattern <environment>-<nodeName>.
	// --with-node-id appends a random suffix so each container gets a unique
	// name in K3s's password store, avoiding conflicts on retries.
	k3sNodeName := e.name + "-" + nodeName
	agentContainerName := e.nodeContainerName(nodeName)

	if remote != nil {
		// Limit one remote node per host IP to avoid K3s tunnel routing
		// conflicts when multiple nodes share the same ExternalIP.
		if existing := findRemoteNodeByHost(ctx, kubeClient, remote.Host); existing != "" {
			logger.Infof("Remote host %s already has node %q. Only one remote node per host is supported.", remote.Host, existing)
			return nil
		}
		if err := e.createRemoteNode(ctx, dockerClient, serverContainer.ID, remote, agentContainerName, k3sNodeName, nodeName, token, scopes, taints, logger); err != nil {
			return err
		}
	} else {
		if err := e.createLocalNode(ctx, dockerClient, serverContainer.ID, agentContainerName, k3sNodeName, nodeName, token, scopes, taints, logger); err != nil {
			return err
		}
	}

	// Discover the node by the overlock.io/node label since --with-node-id
	// appends a random suffix to the actual Kubernetes node name.
	actualNodeName, err := e.waitForNodeReadyByLabel(ctx, kubeClient, nodeName, logger)
	if err != nil {
		return err
	}

	// Apply role labels matching scopes (cannot be set via --node-labels
	// because node-role.kubernetes.io is a protected namespace on the kubelet).
	if err := labelNodeRoles(ctx, kubeClient, actualNodeName, scopes); err != nil {
		logger.Warnf("Failed to label node %q with roles: %v", actualNodeName, err)
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
func (e *Environment) createLocalNode(ctx context.Context, dockerClient *docker.Client, _, agentContainerName, k3sNodeName, nodeName, token string, scopes []string, taints []string, logger *zap.SugaredLogger) error {
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

	// Each agent on the same host (all using --network host) competes for the
	// same loopback/wildcard ports. Allocate unique free ports for every
	// component that binds a fixed default:
	//   - lb-server-port      (default 6444)  k3s supervisor proxy
	//   - containerd stream   (default 10010) written via config.toml.tmpl
	//   - kubelet healthz     (default 10248) --kubelet-arg healthz-port
	//   - kubelet API         (default 10250) --kubelet-arg port
	//   - kube-proxy metrics  (default 10249) --kube-proxy-arg metrics-bind-address
	//   - kube-proxy healthz  (default 10256) --kube-proxy-arg healthz-bind-address
	ports, err := freePorts(6)
	if err != nil {
		return fmt.Errorf("failed to allocate free ports for agent: %w", err)
	}
	lbPort, streamPort := ports[0], ports[1]
	kubeletPort, kubeletHealthzPort := ports[2], ports[3]
	kubeProxyMetricsPort, kubeProxyHealthzPort := ports[4], ports[5]

	agentCmd := []string{"agent", "--with-node-id", "--node-name", k3sNodeName,
		"--node-label", fmt.Sprintf("%s=%s", nodeLabel, nodeName),
		"--node-external-ip", hostIP,
		"--lb-server-port", strconv.Itoa(lbPort),
		"--kubelet-arg", fmt.Sprintf("port=%d", kubeletPort),
		"--kubelet-arg", "read-only-port=0",
		"--kubelet-arg", fmt.Sprintf("healthz-port=%d", kubeletHealthzPort),
		"--kube-proxy-arg", fmt.Sprintf("metrics-bind-address=127.0.0.1:%d", kubeProxyMetricsPort),
		"--kube-proxy-arg", fmt.Sprintf("healthz-bind-address=0.0.0.0:%d", kubeProxyHealthzPort),
	}
	for _, scope := range scopes {
		agentCmd = append(agentCmd, "--node-label", fmt.Sprintf("%s=%s", scopeLabel, scope))
	}
	for _, taint := range taints {
		agentCmd = append(agentCmd, "--node-taint", formatTaint(taint))
		agentCmd = append(agentCmd, "--node-label", formatLabel(taint))
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
		Privileged:  true,
		NetworkMode: "host",
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

	hostConfig.Binds = append(hostConfig.Binds, e.mounts...)

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

	// Pre-populate the volume with a containerd config template that uses a
	// unique streaming server port, preventing conflict with other agents on
	// the same host (which also use --network host and would otherwise all
	// try to bind 127.0.0.1:10010).
	if err := initAgentVolume(ctx, dockerClient, volumeName, streamPort, logger); err != nil {
		return fmt.Errorf("failed to initialise agent volume: %w", err)
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
func (e *Environment) createRemoteNode(_ context.Context, _ *docker.Client, _ string, remote *SSHClient, agentContainerName, k3sNodeName, nodeName, token string, scopes []string, taints []string, logger *zap.SugaredLogger) error {
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
	}
	for _, taint := range taints {
		scopeFlags += fmt.Sprintf(" --node-taint %s", formatTaint(taint))
		scopeFlags += fmt.Sprintf(" --node-label %s", formatLabel(taint))
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
		"docker run -d --privileged --network host --name %s -v /lib/modules:/lib/modules:ro -v %s:/var/lib/rancher/k3s --tmpfs /run --tmpfs /var/run -e K3S_URL=%s -e K3S_TOKEN=%s%s %s agent --with-node-id --node-name %s --node-label %s=%s --node-external-ip %s%s",
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

// findRemoteNodeByHost returns the short node name of an existing remote node
// on the given host, or empty string if none exists.
func findRemoteNodeByHost(ctx context.Context, kubeClient *kubernetes.Clientset, host string) string {
	nodes, err := kubeClient.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return ""
	}
	for _, node := range nodes.Items {
		if node.Annotations[annSSHHost] == host {
			if name := node.Labels[nodeLabel]; name != "" {
				return name
			}
			return node.Name
		}
	}
	return ""
}

// freePorts allocates n free TCP ports on the loopback interface and returns
// them. All listeners are held open until the slice is fully built to avoid
// the OS re-using a port before the caller has had a chance to use it.
func freePorts(n int) ([]int, error) {
	listeners := make([]net.Listener, 0, n)
	ports := make([]int, 0, n)
	for i := 0; i < n; i++ {
		l, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			for _, ll := range listeners {
				ll.Close()
			}
			return nil, err
		}
		listeners = append(listeners, l)
		ports = append(ports, l.Addr().(*net.TCPAddr).Port)
	}
	for _, l := range listeners {
		l.Close()
	}
	return ports, nil
}

// initAgentVolume pre-populates the named k3s data volume with a containerd
// config.toml.tmpl that overrides the streaming server port. Without this,
// all agents on the same host (--network host) would collide on 127.0.0.1:10010.
func initAgentVolume(ctx context.Context, dockerClient *docker.Client, volumeName string, streamPort int, logger *zap.SugaredLogger) error {
	configContent := buildContainerdConfig(streamPort)
	script := "mkdir -p /var/lib/rancher/k3s/agent/etc/containerd && cat > /var/lib/rancher/k3s/agent/etc/containerd/config.toml.tmpl"

	resp, err := dockerClient.ContainerCreate(ctx, &container.Config{
		Image:      k3sDockerImage,
		Entrypoint: []string{"/bin/sh"},
		Cmd:        []string{"-c", script},
		OpenStdin:  true,
		StdinOnce:  true,
	}, &container.HostConfig{
		Binds: []string{volumeName + ":/var/lib/rancher/k3s"},
	}, nil, nil, "")
	if err != nil {
		return fmt.Errorf("failed to create init container: %w", err)
	}
	defer func() {
		if rerr := dockerClient.ContainerRemove(ctx, resp.ID, types.ContainerRemoveOptions{Force: true}); rerr != nil {
			logger.Warnf("Failed to remove init container: %v", rerr)
		}
	}()

	hijack, err := dockerClient.ContainerAttach(ctx, resp.ID, types.ContainerAttachOptions{
		Stdin:  true,
		Stream: true,
	})
	if err != nil {
		return fmt.Errorf("failed to attach to init container: %w", err)
	}

	if err := dockerClient.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{}); err != nil {
		hijack.Close()
		return fmt.Errorf("failed to start init container: %w", err)
	}

	if _, err := io.WriteString(hijack.Conn, configContent); err != nil {
		logger.Warnf("Failed to write containerd config template: %v", err)
	}
	hijack.Close()

	statusCh, errCh := dockerClient.ContainerWait(ctx, resp.ID, container.WaitConditionNotRunning)
	select {
	case err := <-errCh:
		return fmt.Errorf("init container error: %w", err)
	case status := <-statusCh:
		if status.StatusCode != 0 {
			return fmt.Errorf("init container exited with status %d", status.StatusCode)
		}
	}
	return nil
}

// buildContainerdConfig returns a containerd config.toml.tmpl with the given
// streaming server port. The template is static (no Go template markers) so
// k3s uses it verbatim, overriding the default port 10010.
func buildContainerdConfig(streamPort int) string {
	return fmt.Sprintf(`version = 3
root = "/var/lib/rancher/k3s/agent/containerd"
state = "/run/k3s/containerd"

[grpc]
  address = "/run/k3s/containerd/containerd.sock"

[plugins.'io.containerd.internal.v1.opt']
  path = "/var/lib/rancher/k3s/agent/containerd"

[plugins.'io.containerd.grpc.v1.cri']
  stream_server_address = "127.0.0.1"
  stream_server_port = "%d"

[plugins.'io.containerd.cri.v1.runtime']
  enable_selinux = false
  enable_unprivileged_ports = true
  enable_unprivileged_icmp = true
  device_ownership_from_security_context = false

[plugins.'io.containerd.cri.v1.images']
  snapshotter = "overlayfs"
  disable_snapshot_annotations = true

[plugins.'io.containerd.cri.v1.images'.pinned_images]
  sandbox = "rancher/mirrored-pause:3.6"

[plugins.'io.containerd.cri.v1.runtime'.cni]
  bin_dir = "/bin"
  conf_dir = "/var/lib/rancher/k3s/agent/etc/cni/net.d"

[plugins.'io.containerd.cri.v1.runtime'.containerd.runtimes.runc]
  runtime_type = "io.containerd.runc.v2"

[plugins.'io.containerd.cri.v1.runtime'.containerd.runtimes.runc.options]
  SystemdCgroup = false

[plugins.'io.containerd.cri.v1.runtime'.containerd.runtimes.runhcs-wcow-process]
  runtime_type = "io.containerd.runhcs.v1"

[plugins.'io.containerd.cri.v1.images'.registry]
  config_path = "/var/lib/rancher/k3s/agent/etc/containerd/certs.d"
`, streamPort)
}

// labelNodeRoles adds node-role.kubernetes.io/<scope> labels to a node.
func labelNodeRoles(ctx context.Context, kubeClient *kubernetes.Clientset, nodeName string, scopes []string) error {
	if len(scopes) == 0 {
		return nil
	}
	node, err := kubeClient.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get node %q: %w", nodeName, err)
	}
	if node.Labels == nil {
		node.Labels = make(map[string]string)
	}
	for _, scope := range scopes {
		node.Labels[fmt.Sprintf("node-role.kubernetes.io/%s", scope)] = ""
	}
	_, err = kubeClient.CoreV1().Nodes().Update(ctx, node, metav1.UpdateOptions{})
	return err
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
