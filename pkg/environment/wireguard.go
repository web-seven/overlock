package environment

import (
	"bytes"
	"context"
	"fmt"
	"hash/fnv"
	"io"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/network"
	docker "github.com/docker/docker/client"
	"go.uber.org/zap"
)

const (
	wgSetupImage = "docker:cli"
	wgPort       = 51820
	envNetMTU    = "1420"
)

// envNetAddrs holds the local-side addresses for an environment.
type envNetAddrs struct {
	localWGAddr       string // 10.100.<idx>.1
	localDockerSubnet string // 10.101.<idx>.0/24
	localDockerGW     string // 10.101.<idx>.1
	serverIP          string // 10.101.<idx>.2
}

// remoteNetAddrs holds the per-peer addresses for one remote host.
type remoteNetAddrs struct {
	peerIdx            int
	wgLocalAddr        string // 10.100.<idx>.1  (same for all peers)
	wgRemoteAddr       string // 10.100.<idx>.<peerIdx+2>
	localDockerSubnet  string // 10.101.<idx>.0/24 (same for all peers)
	remoteDockerSubnet string // 10.<102+peerIdx>.<idx>.0/24
	remoteDockerGW     string // 10.<102+peerIdx>.<idx>.1
}

// envSubnetIndex returns a deterministic 0-255 index from the environment name.
func envSubnetIndex(name string) uint8 {
	h := fnv.New32a()
	_, _ = h.Write([]byte(name))
	return uint8(h.Sum32())
}

// computeEnvNetAddrs computes the local-side addresses for the environment.
func computeEnvNetAddrs(name string) envNetAddrs {
	idx := envSubnetIndex(name)
	return envNetAddrs{
		localWGAddr:       fmt.Sprintf("10.100.%d.1", idx),
		localDockerSubnet: fmt.Sprintf("10.101.%d.0/24", idx),
		localDockerGW:     fmt.Sprintf("10.101.%d.1", idx),
		serverIP:          fmt.Sprintf("10.101.%d.2", idx),
	}
}

// computeRemoteNetAddrs computes per-peer addresses for one remote host.
func computeRemoteNetAddrs(name string, peerIdx int) remoteNetAddrs {
	idx := envSubnetIndex(name)
	return remoteNetAddrs{
		peerIdx:            peerIdx,
		wgLocalAddr:        fmt.Sprintf("10.100.%d.1", idx),
		wgRemoteAddr:       fmt.Sprintf("10.100.%d.%d", idx, peerIdx+2),
		localDockerSubnet:  fmt.Sprintf("10.101.%d.0/24", idx),
		remoteDockerSubnet: fmt.Sprintf("10.%d.%d.0/24", 102+peerIdx, idx),
		remoteDockerGW:     fmt.Sprintf("10.%d.%d.1", 102+peerIdx, idx),
	}
}

// envNetworkName returns the Docker network name for this environment.
func (e *Environment) envNetworkName() string {
	return "overlock-" + e.name
}

// createEnvironmentNetwork creates (idempotent) the local Docker bridge network.
func (e *Environment) createEnvironmentNetwork(ctx context.Context, dockerClient *docker.Client) error {
	netName := e.envNetworkName()
	addrs := computeEnvNetAddrs(e.name)

	f := filters.NewArgs()
	f.Add("name", netName)
	existing, err := dockerClient.NetworkList(ctx, types.NetworkListOptions{Filters: f})
	if err != nil {
		return fmt.Errorf("failed to list Docker networks: %w", err)
	}
	for _, n := range existing {
		if n.Name == netName {
			return nil
		}
	}

	_, err = dockerClient.NetworkCreate(ctx, netName, types.NetworkCreate{
		Driver: "bridge",
		IPAM: &network.IPAM{
			Config: []network.IPAMConfig{
				{Subnet: addrs.localDockerSubnet, Gateway: addrs.localDockerGW},
			},
		},
		Options: map[string]string{
			"com.docker.network.bridge.enable_ip_masquerade": "false",
			"com.docker.network.driver.mtu":                  envNetMTU,
		},
		Labels: map[string]string{
			"managed-by":              "overlock",
			"overlock.io/environment": e.name,
		},
	})
	if err != nil {
		return fmt.Errorf("failed to create Docker network %q: %w", netName, err)
	}
	return nil
}

// deleteEnvironmentNetwork removes the local Docker bridge network.
func (e *Environment) deleteEnvironmentNetwork(ctx context.Context, dockerClient *docker.Client, logger *zap.SugaredLogger) {
	netName := e.envNetworkName()
	f := filters.NewArgs()
	f.Add("name", netName)
	nets, err := dockerClient.NetworkList(ctx, types.NetworkListOptions{Filters: f})
	if err != nil {
		logger.Warnf("Failed to list networks for cleanup: %v", err)
		return
	}
	for _, n := range nets {
		if n.Name == netName {
			if err := dockerClient.NetworkRemove(ctx, n.ID); err != nil {
				logger.Warnf("Failed to remove Docker network %q: %v", netName, err)
			}
			return
		}
	}
}

// getLocalWGPubkey returns the local WireGuard public key, generating the
// private key file if it does not already exist.
func (e *Environment) getLocalWGPubkey(ctx context.Context, dockerClient *docker.Client) (string, error) {
	keyFile := fmt.Sprintf("/tmp/wg-%s.key", e.name)
	script := fmt.Sprintf(
		`apk add -q wireguard-tools >/dev/null 2>&1; [ -f %s ] || (umask 077; wg genkey > %s); wg pubkey < %s`,
		keyFile, keyFile, keyFile,
	)
	return runPrivilegedScript(ctx, dockerClient, script)
}

// ensureLocalWG0 creates wg0 on the local host if it does not already exist.
func (e *Environment) ensureLocalWG0(ctx context.Context, dockerClient *docker.Client) error {
	addrs := computeEnvNetAddrs(e.name)
	keyFile := fmt.Sprintf("/tmp/wg-%s.key", e.name)
	script := fmt.Sprintf(`
apk add -q wireguard-tools iproute2 iptables >/dev/null 2>&1
if ip link show wg0 >/dev/null 2>&1; then
    ip addr show wg0 | grep -q '%s/24' || {
        ip addr flush dev wg0
        ip addr add %s/24 dev wg0
    }
    exit 0
fi
[ -f %s ] || (umask 077; wg genkey > %s)
ip link add wg0 type wireguard
ip addr add %s/24 dev wg0
wg set wg0 private-key %s listen-port %d
ip link set wg0 up
echo 1 > /proc/sys/net/ipv4/ip_forward
iptables -I INPUT -p udp --dport %d -j ACCEPT 2>/dev/null || true`,
		addrs.localWGAddr, addrs.localWGAddr,
		keyFile, keyFile,
		addrs.localWGAddr,
		keyFile, wgPort,
		wgPort,
	)
	_, err := runPrivilegedScript(ctx, dockerClient, script)
	return err
}

// addRemotePeer sets up WireGuard on the remote host and adds it as a new peer
// on local wg0. Returns the remote's WireGuard public key.
func (e *Environment) addRemotePeer(ctx context.Context, dockerClient *docker.Client, remote *SSHClient, peerIdx int, logger *zap.SugaredLogger) (string, error) {
	addrs := computeRemoteNetAddrs(e.name, peerIdx)
	remotePubFile := fmt.Sprintf("/tmp/wg-%s-%d-remote.pub", e.name, peerIdx)
	remoteKeyFile := fmt.Sprintf("/tmp/wg-%s-%d.key", e.name, peerIdx)

	// Pull setup image.
	pullReader, err := dockerClient.ImagePull(ctx, wgSetupImage, types.ImagePullOptions{})
	if err != nil {
		logger.Debugf("Failed to pull %s: %v", wgSetupImage, err)
	} else {
		_, _ = io.Copy(io.Discard, pullReader)
		pullReader.Close()
	}

	// Ensure local wg0 exists.
	if err := e.ensureLocalWG0(ctx, dockerClient); err != nil {
		return "", fmt.Errorf("failed to ensure local wg0: %w", err)
	}

	localPubkey, err := e.getLocalWGPubkey(ctx, dockerClient)
	if err != nil {
		return "", fmt.Errorf("failed to get local WireGuard public key: %w", err)
	}
	logger.Debugf("Local WireGuard public key: %s", localPubkey)

	// Set up the remote side.
	logger.Debugf("Adding WireGuard peer %d on remote %s...", peerIdx, remote.Host)
	remoteScript := buildRemoteAddPeerScript(addrs, localPubkey, e.name, remoteKeyFile, remotePubFile)
	remoteDockerCmd := fmt.Sprintf(
		"docker run --rm -i --privileged --network host -v /tmp:/tmp -v /var/run/docker.sock:/var/run/docker.sock %s sh",
		wgSetupImage,
	)
	remoteOut, err := remote.RunWithStdin(remoteDockerCmd, remoteScript)
	if err != nil {
		return "", fmt.Errorf("failed to set up remote peer %d: %w\noutput: %s", peerIdx, err, remoteOut)
	}

	remotePubkey := extractWGPubkey(remoteOut)
	if remotePubkey == "" {
		return "", fmt.Errorf("remote peer setup did not produce a public key; output: %s", remoteOut)
	}
	logger.Debugf("Remote WireGuard public key (peer %d): %s", peerIdx, remotePubkey)

	// Add remote as a new peer on local wg0.
	localScript := buildLocalAddPeerScript(addrs, remotePubkey, remote.Host, e.name)
	if _, err := runPrivilegedScript(ctx, dockerClient, localScript); err != nil {
		return "", fmt.Errorf("failed to add remote peer to local wg0: %w", err)
	}

	// Poll until the tunnel is up (handshake timing varies).
	pingScript := fmt.Sprintf("ping -c 1 -W 2 %s >/dev/null 2>&1 && echo OK || echo FAIL", addrs.wgRemoteAddr)
	tunnelUp := false
	for i := 0; i < 10; i++ {
		time.Sleep(1 * time.Second)
		pingOut, _ := runPrivilegedScript(ctx, dockerClient, pingScript)
		if strings.Contains(pingOut, "OK") {
			tunnelUp = true
			break
		}
	}
	if tunnelUp {
		logger.Infof("WireGuard peer %d up: %s <-> %s", peerIdx, addrs.wgLocalAddr, addrs.wgRemoteAddr)
	} else {
		logger.Warnf("WireGuard peer %d ping failed — check that UDP %d is open on %s", peerIdx, wgPort, remote.Host)
	}

	return remotePubkey, nil
}

// ensureRemotePeer ensures the WireGuard peer for the given remote host is
// active, setting it up from scratch if necessary (e.g. after a reboot).
func (e *Environment) ensureRemotePeer(ctx context.Context, dockerClient *docker.Client, remote *SSHClient, peerIdx int, remotePubkey string, logger *zap.SugaredLogger) error {
	addrs := computeRemoteNetAddrs(e.name, peerIdx)
	checkScript := fmt.Sprintf(
		`apk add -q iproute2 >/dev/null 2>&1; ip route show %s >/dev/null 2>&1 && echo UP || echo DOWN`,
		addrs.remoteDockerSubnet,
	)
	out, _ := runPrivilegedScript(ctx, dockerClient, checkScript)
	if strings.Contains(out, "UP") {
		logger.Debugf("WireGuard peer %d (%s) already up, skipping.", peerIdx, remote.Host)
		return nil
	}
	_, err := e.addRemotePeer(ctx, dockerClient, remote, peerIdx, logger)
	return err
}

// removeRemotePeer removes one WireGuard peer from local wg0 and tears down the
// remote side. Best-effort — logs warnings on errors.
func (e *Environment) removeRemotePeer(ctx context.Context, dockerClient *docker.Client, remote *SSHClient, peerIdx int, remotePubkey string, logger *zap.SugaredLogger) {
	addrs := computeRemoteNetAddrs(e.name, peerIdx)
	netName := e.envNetworkName()

	localScript := buildLocalRemovePeerScript(addrs, remotePubkey)
	if _, err := runPrivilegedScript(ctx, dockerClient, localScript); err != nil {
		logger.Warnf("Failed to remove local WireGuard peer %d: %v", peerIdx, err)
	}

	if remote == nil {
		return
	}

	remoteScript := buildRemoteRemovePeerScript(addrs, e.name, peerIdx, netName)
	remoteDockerCmd := fmt.Sprintf(
		"docker run --rm -i --privileged --network host -v /tmp:/tmp -v /var/run/docker.sock:/var/run/docker.sock %s sh",
		wgSetupImage,
	)
	if _, err := remote.RunWithStdin(remoteDockerCmd, remoteScript); err != nil {
		logger.Warnf("Failed to tear down remote WireGuard peer %d: %v", peerIdx, err)
	}
}

// runPrivilegedScript runs a shell script in a privileged Docker container with
// host networking and /tmp mounted. Returns the combined stdout output.
func runPrivilegedScript(ctx context.Context, dockerClient *docker.Client, script string) (string, error) {
	resp, err := dockerClient.ContainerCreate(ctx, &container.Config{
		Image: wgSetupImage,
		Cmd:   []string{"sh", "-c", script},
		Tty:   true,
	}, &container.HostConfig{
		Privileged:  true,
		NetworkMode: "host",
		Binds:       []string{"/tmp:/tmp", "/var/run/docker.sock:/var/run/docker.sock"},
	}, nil, nil, "")
	if err != nil {
		return "", fmt.Errorf("failed to create privileged container: %w", err)
	}
	defer func() {
		_ = dockerClient.ContainerRemove(ctx, resp.ID, types.ContainerRemoveOptions{Force: true})
	}()

	if err := dockerClient.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{}); err != nil {
		return "", fmt.Errorf("failed to start privileged container: %w", err)
	}

	statusCh, errCh := dockerClient.ContainerWait(ctx, resp.ID, container.WaitConditionNotRunning)
	select {
	case err := <-errCh:
		return "", fmt.Errorf("container wait error: %w", err)
	case status := <-statusCh:
		logs, _ := dockerClient.ContainerLogs(ctx, resp.ID, types.ContainerLogsOptions{ShowStdout: true, ShowStderr: true})
		var buf bytes.Buffer
		if logs != nil {
			_, _ = io.Copy(&buf, logs)
			logs.Close()
		}
		if status.StatusCode != 0 {
			return "", fmt.Errorf("script exited with code %d: %s", status.StatusCode, strings.TrimSpace(buf.String()))
		}
		return strings.TrimSpace(buf.String()), nil
	}
}

// extractWGPubkey finds the last 44-character base64 line in the output,
// which is the WireGuard public key.
func extractWGPubkey(output string) string {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if len(line) == 44 {
			return line
		}
	}
	return ""
}

// buildRemoteAddPeerScript generates the script sent to the remote to configure
// its wg0 with local as a peer and create the remote Docker bridge network.
func buildRemoteAddPeerScript(addrs remoteNetAddrs, localPubkey, envName, keyFile, pubFile string) string {
	netName := "overlock-" + envName
	return fmt.Sprintf(`apk add -q wireguard-tools iproute2 iptables nftables >/dev/null 2>&1

# Generate/reuse remote key for peer %d
[ -f %s ] || (umask 077; wg genkey > %s)
wg pubkey < %s > %s
chmod 644 %s

# Create wg0 if not present
ip link show wg0 >/dev/null 2>&1 || {
    ip link add wg0 type wireguard
    ip addr add %s/24 dev wg0
    wg set wg0 private-key %s listen-port %d
    ip link set wg0 up
    echo 1 > /proc/sys/net/ipv4/ip_forward
}

# Add/update local peer
wg set wg0 peer %s allowed-ips %s/32,%s

# Create Docker network if not present
docker network ls --format "{{.Name}}" | grep -q "^%s$" || \
    docker network create --driver bridge \
        --subnet %s \
        --gateway %s \
        --opt com.docker.network.bridge.enable_ip_masquerade=false \
        --opt com.docker.network.driver.mtu=%s \
        %s

# Route to local Docker subnet
ip route replace %s via %s

# FORWARD + nft rules
BR=$(docker network inspect %s --format "{{.Id}}" | head -c 12)
iptables -I FORWARD -i br-$BR -o wg0 -j ACCEPT 2>/dev/null || true
iptables -I FORWARD -i wg0 -o br-$BR -j ACCEPT 2>/dev/null || true
nft insert rule ip raw PREROUTING iifname wg0 ip daddr %s return 2>/dev/null || true
iptables -t nat -A POSTROUTING -s %s ! -d %s ! -o wg0 -j MASQUERADE 2>/dev/null || true

cat %s`,
		addrs.peerIdx,
		keyFile, keyFile, keyFile, pubFile, pubFile,
		addrs.wgRemoteAddr,
		keyFile, wgPort,
		localPubkey, addrs.wgLocalAddr, addrs.localDockerSubnet,
		netName,
		addrs.remoteDockerSubnet, addrs.remoteDockerGW,
		envNetMTU,
		netName,
		addrs.localDockerSubnet, addrs.wgLocalAddr,
		netName,
		addrs.remoteDockerSubnet,
		addrs.remoteDockerSubnet, addrs.localDockerSubnet,
		pubFile,
	)
}

// buildLocalAddPeerScript generates the script run locally to add a new remote
// peer to the existing wg0.
func buildLocalAddPeerScript(addrs remoteNetAddrs, remotePubkey, remoteHost, envName string) string {
	netName := "overlock-" + envName
	return fmt.Sprintf(`apk add -q wireguard-tools iproute2 iptables nftables >/dev/null 2>&1

# Add/update remote peer on wg0
wg set wg0 peer %s \
    endpoint %s:%d \
    allowed-ips %s/32,%s \
    persistent-keepalive 25

# Route to remote Docker subnet
ip route replace %s via %s

# FORWARD + nft rules (idempotent with || true)
BR=$(docker network inspect %s --format "{{.Id}}" | head -c 12)
iptables -I FORWARD -i br-$BR -o wg0 -j ACCEPT 2>/dev/null || true
iptables -I FORWARD -i wg0 -o br-$BR -j ACCEPT 2>/dev/null || true
nft insert rule ip nat nat_POST_public_allow oifname wg0 return 2>/dev/null || true
nft insert rule ip raw PREROUTING iifname wg0 ip daddr %s return 2>/dev/null || true
iptables -t nat -A POSTROUTING -s %s ! -d %s ! -o wg0 -j MASQUERADE 2>/dev/null || true`,
		remotePubkey,
		remoteHost, wgPort,
		addrs.wgRemoteAddr, addrs.remoteDockerSubnet,
		addrs.remoteDockerSubnet, addrs.wgRemoteAddr,
		netName,
		addrs.localDockerSubnet,
		addrs.localDockerSubnet, addrs.remoteDockerSubnet,
	)
}

// buildLocalRemovePeerScript removes one peer from local wg0 and cleans up its
// route. Deletes wg0 entirely if no peers remain.
func buildLocalRemovePeerScript(addrs remoteNetAddrs, remotePubkey string) string {
	return fmt.Sprintf(`apk add -q wireguard-tools iproute2 >/dev/null 2>&1
ip link show wg0 >/dev/null 2>&1 && wg set wg0 peer %s remove 2>/dev/null || true
ip route del %s 2>/dev/null || true
PEERS=$(ip link show wg0 >/dev/null 2>&1 && wg show wg0 peers 2>/dev/null | wc -l || echo 0)
[ "$PEERS" -eq 0 ] && ip link del wg0 2>/dev/null || true`,
		remotePubkey,
		addrs.remoteDockerSubnet,
	)
}

// buildRemoteRemovePeerScript tears down the remote Docker network and wg0 peer
// for the given peer index.
func buildRemoteRemovePeerScript(addrs remoteNetAddrs, envName string, peerIdx int, netName string) string {
	remoteKeyFile := fmt.Sprintf("/tmp/wg-%s-%d.key", envName, peerIdx)
	remotePubFile := fmt.Sprintf("/tmp/wg-%s-%d-remote.pub", envName, peerIdx)
	return fmt.Sprintf(`apk add -q iproute2 iptables nftables >/dev/null 2>&1
ip route del %s 2>/dev/null || true
docker ps -q --filter network=%s | xargs -r docker rm -f 2>/dev/null || true
docker network rm %s 2>/dev/null || true
rm -f %s %s
PEERS=$(ip link show wg0 >/dev/null 2>&1 && wg show wg0 peers 2>/dev/null | wc -l || echo 0)
[ "$PEERS" -eq 0 ] && ip link del wg0 2>/dev/null || true`,
		addrs.localDockerSubnet,
		netName, netName,
		remoteKeyFile, remotePubFile,
	)
}
