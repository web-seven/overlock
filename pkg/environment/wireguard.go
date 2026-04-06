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

// envNetAddrs holds all pre-computed addresses for a given environment.
type envNetAddrs struct {
	localWGAddr        string // e.g. 10.100.<idx>.1
	remoteWGAddr       string // e.g. 10.100.<idx>.2
	localDockerSubnet  string // e.g. 10.101.<idx>.0/24
	localDockerGW      string // e.g. 10.101.<idx>.1
	remoteDockerSubnet string // e.g. 10.102.<idx>.0/24
	remoteDockerGW     string // e.g. 10.102.<idx>.1
	serverIP           string // e.g. 10.101.<idx>.2
}

// envSubnetIndex returns a deterministic 0-255 index from the environment name.
func envSubnetIndex(name string) uint8 {
	h := fnv.New32a()
	_, _ = h.Write([]byte(name))
	return uint8(h.Sum32())
}

// computeEnvNetAddrs computes all addresses for the given environment name.
func computeEnvNetAddrs(name string) envNetAddrs {
	idx := envSubnetIndex(name)
	return envNetAddrs{
		localWGAddr:        fmt.Sprintf("10.100.%d.1", idx),
		remoteWGAddr:       fmt.Sprintf("10.100.%d.2", idx),
		localDockerSubnet:  fmt.Sprintf("10.101.%d.0/24", idx),
		localDockerGW:      fmt.Sprintf("10.101.%d.1", idx),
		remoteDockerSubnet: fmt.Sprintf("10.102.%d.0/24", idx),
		remoteDockerGW:     fmt.Sprintf("10.102.%d.1", idx),
		serverIP:           fmt.Sprintf("10.101.%d.2", idx),
	}
}

// envNetworkName returns the Docker network name for this environment.
func (e *Environment) envNetworkName() string {
	return "overlock-" + e.name
}

// createEnvironmentNetwork creates (idempotent) the local Docker bridge network
// for this environment. Masquerade is disabled so WireGuard routing works correctly.
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

// deleteEnvironmentNetwork removes the local Docker bridge network for this environment.
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

// setupWireGuardTunnel establishes a WireGuard tunnel between the local host and
// the remote host, connecting the two Docker bridge networks.
func (e *Environment) setupWireGuardTunnel(ctx context.Context, dockerClient *docker.Client, remote *SSHClient, logger *zap.SugaredLogger) error {
	addrs := computeEnvNetAddrs(e.name)
	keyFile := fmt.Sprintf("/tmp/wg-%s.key", e.name)
	remotePubFile := fmt.Sprintf("/tmp/wg-%s-remote.pub", e.name)

	// Pull the setup image (likely already cached).
	pullReader, err := dockerClient.ImagePull(ctx, wgSetupImage, types.ImagePullOptions{})
	if err != nil {
		logger.Debugf("Failed to pull %s: %v", wgSetupImage, err)
	} else {
		_, _ = io.Copy(io.Discard, pullReader)
		pullReader.Close()
	}

	// Generate local WireGuard key and capture the public key.
	logger.Debug("Generating local WireGuard keys...")
	genScript := fmt.Sprintf(
		"apk add -q wireguard-tools >/dev/null 2>&1; umask 077; wg genkey > %s; wg pubkey < %s",
		keyFile, keyFile,
	)
	localPubkey, err := runPrivilegedScript(ctx, dockerClient, genScript)
	if err != nil {
		return fmt.Errorf("failed to generate local WireGuard keys: %w", err)
	}
	localPubkey = strings.TrimSpace(localPubkey)
	logger.Debugf("Local WireGuard public key: %s", localPubkey)

	// Set up WireGuard on the remote host.
	logger.Debugf("Setting up WireGuard on remote %s...", remote.Host)
	remoteScript := buildRemoteWGScript(addrs, localPubkey, e.name, keyFile, remotePubFile)
	remoteDockerCmd := fmt.Sprintf(
		"docker run --rm -i --privileged --network host -v /tmp:/tmp -v /var/run/docker.sock:/var/run/docker.sock %s sh",
		wgSetupImage,
	)
	remoteOut, err := remote.RunWithStdin(remoteDockerCmd, remoteScript)
	if err != nil {
		return fmt.Errorf("failed to set up WireGuard on remote: %w\noutput: %s", err, remoteOut)
	}

	// The remote script ends with `cat <pubfile>`, so the last 44-char line is the pubkey.
	remotePubkey := extractWGPubkey(remoteOut)
	if remotePubkey == "" {
		return fmt.Errorf("remote WireGuard setup did not produce a public key; output: %s", remoteOut)
	}
	logger.Debugf("Remote WireGuard public key: %s", remotePubkey)

	// Set up WireGuard locally.
	logger.Debug("Setting up WireGuard locally...")
	localScript := buildLocalWGScript(addrs, remotePubkey, remote.Host, e.name, keyFile)
	if _, err := runPrivilegedScript(ctx, dockerClient, localScript); err != nil {
		return fmt.Errorf("failed to set up WireGuard locally: %w", err)
	}

	// Brief pause then verify the tunnel.
	time.Sleep(2 * time.Second)
	pingScript := fmt.Sprintf("ping -c 2 -W 3 %s >/dev/null 2>&1 && echo OK || echo FAIL", addrs.remoteWGAddr)
	pingOut, _ := runPrivilegedScript(ctx, dockerClient, pingScript)
	if strings.Contains(pingOut, "OK") {
		logger.Infof("WireGuard tunnel up: %s <-> %s", addrs.localWGAddr, addrs.remoteWGAddr)
	} else {
		logger.Warnf("WireGuard tunnel ping failed — check that UDP %d is open on the remote host", wgPort)
	}

	return nil
}

// ensureWireGuardTunnel sets up the WireGuard tunnel if the local wg0 interface
// is not already present.
func (e *Environment) ensureWireGuardTunnel(ctx context.Context, dockerClient *docker.Client, remote *SSHClient, logger *zap.SugaredLogger) error {
	checkScript := "apk add -q iproute2 >/dev/null 2>&1; ip link show wg0 >/dev/null 2>&1 && echo TUNNEL_UP || echo TUNNEL_DOWN"
	out, _ := runPrivilegedScript(ctx, dockerClient, checkScript)
	if strings.Contains(out, "TUNNEL_UP") {
		logger.Debug("WireGuard tunnel already up, skipping setup.")
		return nil
	}
	return e.setupWireGuardTunnel(ctx, dockerClient, remote, logger)
}

// teardownWireGuardTunnel removes the WireGuard tunnel on both sides. Best-effort.
func (e *Environment) teardownWireGuardTunnel(ctx context.Context, dockerClient *docker.Client, remote *SSHClient, logger *zap.SugaredLogger) {
	addrs := computeEnvNetAddrs(e.name)
	netName := e.envNetworkName()

	localTeardown := fmt.Sprintf(
		`apk add -q iproute2 iptables nftables >/dev/null 2>&1
nft delete rule ip raw PREROUTING iifname "wg0" ip daddr %s return 2>/dev/null || true
nft delete rule ip nat nat_POST_public_allow oifname "wg0" return 2>/dev/null || true
ip link del wg0 2>/dev/null || true
ip route del %s 2>/dev/null || true
rm -f /tmp/wg-%s.key /tmp/wg-%s-remote.pub`,
		addrs.localDockerSubnet, addrs.remoteDockerSubnet, e.name, e.name,
	)
	if _, err := runPrivilegedScript(ctx, dockerClient, localTeardown); err != nil {
		logger.Warnf("Failed to tear down local WireGuard: %v", err)
	}

	if remote == nil {
		return
	}

	remoteTeardown := fmt.Sprintf(
		`apk add -q iproute2 iptables nftables >/dev/null 2>&1
nft delete rule ip raw PREROUTING iifname "wg0" ip daddr %s return 2>/dev/null || true
ip link del wg0 2>/dev/null || true
ip route del %s 2>/dev/null || true
docker ps -q --filter network=%s | xargs -r docker rm -f 2>/dev/null || true
docker network rm %s 2>/dev/null || true
rm -f /tmp/wg-%s.key /tmp/wg-%s-remote.pub`,
		addrs.remoteDockerSubnet, addrs.localDockerSubnet, netName, netName, e.name, e.name,
	)
	remoteDockerCmd := fmt.Sprintf(
		"docker run --rm -i --privileged --network host -v /tmp:/tmp -v /var/run/docker.sock:/var/run/docker.sock %s sh",
		wgSetupImage,
	)
	if _, err := remote.RunWithStdin(remoteDockerCmd, remoteTeardown); err != nil {
		logger.Warnf("Failed to tear down remote WireGuard: %v", err)
	}
}

// runPrivilegedScript runs a shell script in a privileged Docker container with
// host networking and /tmp mounted. Returns combined stdout output.
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

// extractWGPubkey finds the last line in the output that looks like a WireGuard
// public key (44-character base64 string).
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

// buildRemoteWGScript generates the shell script sent to the remote host to
// configure WireGuard and create the remote Docker bridge network.
func buildRemoteWGScript(addrs envNetAddrs, localPubkey, envName, keyFile, pubFile string) string {
	netName := "overlock-" + envName
	return fmt.Sprintf(`apk add -q wireguard-tools iproute2 iptables nftables >/dev/null 2>&1

umask 077
wg genkey > %s
wg pubkey < %s > %s
chmod 644 %s

ip link del wg0 2>/dev/null || true
ip link add wg0 type wireguard
ip addr add %s/30 dev wg0
wg set wg0 \
    private-key %s \
    listen-port %d \
    peer %s \
        allowed-ips %s/32,%s
ip link set wg0 up
echo 1 > /proc/sys/net/ipv4/ip_forward

docker network rm %s 2>/dev/null || true
docker network create --driver bridge \
    --subnet %s \
    --gateway %s \
    --opt com.docker.network.bridge.enable_ip_masquerade=false \
    --opt com.docker.network.driver.mtu=%s \
    %s

ip route replace %s via %s

BR=$(docker network inspect %s --format "{{.Id}}" | head -c 12)
iptables -I FORWARD -i br-$BR -o wg0 -j ACCEPT 2>/dev/null || true
iptables -I FORWARD -i wg0 -o br-$BR -j ACCEPT 2>/dev/null || true
nft insert rule ip raw PREROUTING iifname wg0 ip daddr %s return 2>/dev/null || true
iptables -t nat -A POSTROUTING -s %s ! -d %s ! -o wg0 -j MASQUERADE 2>/dev/null || true

cat %s`,
		keyFile, keyFile, pubFile, pubFile,
		addrs.remoteWGAddr,
		keyFile,
		wgPort,
		localPubkey,
		addrs.localWGAddr, addrs.localDockerSubnet,
		netName,
		addrs.remoteDockerSubnet, addrs.remoteDockerGW,
		envNetMTU,
		netName,
		addrs.localDockerSubnet, addrs.localWGAddr,
		netName,
		addrs.remoteDockerSubnet,
		addrs.remoteDockerSubnet, addrs.localDockerSubnet,
		pubFile,
	)
}

// buildLocalWGScript generates the shell script run locally to complete the
// WireGuard tunnel setup (after the remote side is already configured).
func buildLocalWGScript(addrs envNetAddrs, remotePubkey, remoteHost, envName, keyFile string) string {
	netName := "overlock-" + envName
	return fmt.Sprintf(`apk add -q wireguard-tools iproute2 iptables nftables >/dev/null 2>&1

ip link del wg0 2>/dev/null || true
ip link add wg0 type wireguard
ip addr add %s/30 dev wg0
wg set wg0 \
    private-key %s \
    peer %s \
        endpoint %s:%d \
        allowed-ips %s/32,%s \
        persistent-keepalive 25
ip link set wg0 up
echo 1 > /proc/sys/net/ipv4/ip_forward

ip route replace %s via %s

BR=$(docker network inspect %s --format "{{.Id}}" | head -c 12)
iptables -I FORWARD -i br-$BR -o wg0 -j ACCEPT 2>/dev/null || true
iptables -I FORWARD -i wg0 -o br-$BR -j ACCEPT 2>/dev/null || true
nft insert rule ip nat nat_POST_public_allow oifname wg0 return 2>/dev/null || true
nft insert rule ip raw PREROUTING iifname wg0 ip daddr %s return 2>/dev/null || true
iptables -t nat -A POSTROUTING -s %s ! -d %s ! -o wg0 -j MASQUERADE 2>/dev/null || true`,
		addrs.localWGAddr,
		keyFile,
		remotePubkey,
		remoteHost, wgPort,
		addrs.remoteWGAddr, addrs.remoteDockerSubnet,
		addrs.remoteDockerSubnet, addrs.remoteWGAddr,
		netName,
		addrs.localDockerSubnet,
		addrs.localDockerSubnet, addrs.remoteDockerSubnet,
	)
}
