package environment

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfigNodes(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "overlock.yaml")
	data := []byte(`
engine: k3s-docker
nodes:
  - name: worker-1
    host: 10.0.0.5
    user: root
    key: ~/.ssh/id_rsa
    port: 2222
    scopes: [engine, workloads]
    taints: [dedicated:gpu]
  - name: local-1
    scopes: [workloads]
`)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := loadConfig(path)
	if err != nil {
		t.Fatalf("loadConfig() unexpected error: %v", err)
	}

	if cfg.Engine != "k3s-docker" {
		t.Fatalf("Engine = %q, want %q", cfg.Engine, "k3s-docker")
	}

	if len(cfg.Nodes) != 2 {
		t.Fatalf("len(Nodes) = %d, want 2", len(cfg.Nodes))
	}

	remote := cfg.Nodes[0]
	if remote.Name != "worker-1" || remote.Host != "10.0.0.5" || remote.User != "root" ||
		remote.Key != "~/.ssh/id_rsa" || remote.Port != 2222 {
		t.Fatalf("unexpected remote node config: %+v", remote)
	}
	if len(remote.Scopes) != 2 || remote.Scopes[0] != "engine" || remote.Scopes[1] != "workloads" {
		t.Fatalf("unexpected remote node scopes: %v", remote.Scopes)
	}
	if len(remote.Taints) != 1 || remote.Taints[0] != "dedicated:gpu" {
		t.Fatalf("unexpected remote node taints: %v", remote.Taints)
	}

	local := cfg.Nodes[1]
	if local.Name != "local-1" || local.Host != "" {
		t.Fatalf("unexpected local node config: %+v", local)
	}
	if len(local.Scopes) != 1 || local.Scopes[0] != "workloads" {
		t.Fatalf("unexpected local node scopes: %v", local.Scopes)
	}
}

func TestCreateNodeRequiresName(t *testing.T) {
	c := &createCmd{createOptions: createOptions{Engine: "k3s-docker"}}
	c.Name = "test-env"

	err := c.createNode(nil, NodeConfig{}, nil)
	if err == nil {
		t.Fatal("createNode() expected error for missing node name, got nil")
	}
}
