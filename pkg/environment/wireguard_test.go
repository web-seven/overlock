package environment

import (
	"strings"
	"testing"
)

// TestPeerScriptsAllowTunnelTrafficInRawTable checks that both sides exempt
// wg0 traffic from Docker's raw PREROUTING drop rules with iptables, which
// creates the raw table if no container created it yet.
func TestPeerScriptsAllowTunnelTrafficInRawTable(t *testing.T) {
	addrs := computeRemoteNetAddrs("dev", 0)

	remote := buildRemoteAddPeerScript(addrs, "localpub", "dev", "/tmp/k", "/tmp/p")
	wantRemote := "iptables -t raw -C PREROUTING -i wg0 -d " + addrs.remoteDockerSubnet + " -j RETURN 2>/dev/null || iptables -t raw -I PREROUTING -i wg0 -d " + addrs.remoteDockerSubnet + " -j RETURN"
	if !strings.Contains(remote, wantRemote) {
		t.Errorf("remote script lacks raw exception for %s:\n%s", addrs.remoteDockerSubnet, remote)
	}

	local := buildLocalAddPeerScript(addrs, "remotepub", "192.0.2.10", "dev")
	wantLocal := "iptables -t raw -C PREROUTING -i wg0 -d " + addrs.localDockerSubnet + " -j RETURN 2>/dev/null || iptables -t raw -I PREROUTING -i wg0 -d " + addrs.localDockerSubnet + " -j RETURN"
	if !strings.Contains(local, wantLocal) {
		t.Errorf("local script lacks raw exception for %s:\n%s", addrs.localDockerSubnet, local)
	}

	for name, script := range map[string]string{"remote": remote, "local": local} {
		if strings.Contains(script, "%!") {
			t.Errorf("%s script has formatting errors:\n%s", name, script)
		}
		if strings.Contains(script, "nft insert rule ip raw") {
			t.Errorf("%s script still uses nft insert for the raw table", name)
		}
	}
}
