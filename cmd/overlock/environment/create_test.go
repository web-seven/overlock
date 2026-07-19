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

func TestLoadConfigName(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "overlock.yaml")
	data := []byte(`
name: my-env
engine: k3s-docker
`)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := loadConfig(path)
	if err != nil {
		t.Fatalf("loadConfig() unexpected error: %v", err)
	}

	if cfg.Name != "my-env" {
		t.Fatalf("Name = %q, want %q", cfg.Name, "my-env")
	}
}

func TestCreateNodeRequiresName(t *testing.T) {
	err := createNode(nil, nil, "", nil, nil, "", "", 0, "", nil, nil)
	if err == nil {
		t.Fatal("createNode() expected error for missing node name, got nil")
	}
}

func TestCreateNodeRemoteRequiresUserPortKey(t *testing.T) {
	cases := []struct {
		name string
		user string
		port int
		key  string
	}{
		{name: "missing all", user: "", port: 0, key: ""},
		{name: "missing user", user: "", port: 2222, key: "~/.ssh/id_rsa"},
		{name: "missing port", user: "root", port: 0, key: "~/.ssh/id_rsa"},
		{name: "missing key", user: "root", port: 2222, key: ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := createNode(nil, nil, "worker-1", nil, nil, "10.0.0.5", tc.user, tc.port, tc.key, nil, nil)
			if err == nil {
				t.Fatal("createNode() expected error for incomplete remote SSH configuration, got nil")
			}
		})
	}
}
