package environment

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/web-seven/overlock/api/v1alpha1"
)

func writeConfig(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "overlock.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}
	return path
}

func TestEnvironmentSpecFromLegacyOptions(t *testing.T) {
	cfg, err := loadConfig(writeConfig(t, `
engine: kind
providers: [xpkg.upbound.io/crossplane-contrib/provider-helm:v0.19.0]
configurations: [xpkg.upbound.io/devops-toolkit/dot-application:v3.0.31]
functions: [xpkg.upbound.io/crossplane-contrib/function-patch-and-transform:v0.7.0]
`))
	if err != nil {
		t.Fatalf("loadConfig() unexpected error: %v", err)
	}
	cfg.MaxReconcileRate = 3
	cfg.CreateAdminServiceAccount = true
	cfg.AdminServiceAccountName = "admin"

	spec, err := cfg.environmentSpec()
	if err != nil {
		t.Fatalf("environmentSpec() unexpected error: %v", err)
	}

	want := v1alpha1.EnvironmentSpec{
		Crossplane:          v1alpha1.CrossplaneSpec{MaxReconcileRate: 3},
		Providers:           []string{"xpkg.upbound.io/crossplane-contrib/provider-helm:v0.19.0"},
		Configurations:      []string{"xpkg.upbound.io/devops-toolkit/dot-application:v3.0.31"},
		Functions:           []string{"xpkg.upbound.io/crossplane-contrib/function-patch-and-transform:v0.7.0"},
		AdminServiceAccount: &v1alpha1.AdminServiceAccountSpec{Name: "admin"},
	}
	if !reflect.DeepEqual(spec, want) {
		t.Fatalf("environmentSpec() = %+v, want %+v", spec, want)
	}
}

func TestEnvironmentSectionOverridesLegacyOptions(t *testing.T) {
	cfg, err := loadConfig(writeConfig(t, `
engine: k3s-docker
providers: [legacy-provider]
functions: [legacy-function]
environment:
  crossplane:
    maxReconcileRate: 5
    args: [--debug]
  providers: [spec-provider]
  tools:
    kyverno:
      enabled: false
`))
	if err != nil {
		t.Fatalf("loadConfig() unexpected error: %v", err)
	}
	if cfg.Engine != "k3s-docker" {
		t.Fatalf("Engine = %q, want k3s-docker", cfg.Engine)
	}
	cfg.MaxReconcileRate = 1

	spec, err := cfg.environmentSpec()
	if err != nil {
		t.Fatalf("environmentSpec() unexpected error: %v", err)
	}

	if spec.Crossplane.MaxReconcileRate != 5 {
		t.Errorf("MaxReconcileRate = %d, want 5", spec.Crossplane.MaxReconcileRate)
	}
	if !reflect.DeepEqual(spec.Crossplane.Args, []string{"--debug"}) {
		t.Errorf("Args = %v, want [--debug]", spec.Crossplane.Args)
	}
	if !reflect.DeepEqual(spec.Providers, []string{"spec-provider"}) {
		t.Errorf("Providers = %v, want [spec-provider]", spec.Providers)
	}
	if !reflect.DeepEqual(spec.Functions, []string{"legacy-function"}) {
		t.Errorf("Functions = %v, want legacy value kept", spec.Functions)
	}
	if spec.Tools.Kyverno.IsEnabled() {
		t.Error("Kyverno enabled, want disabled")
	}
	if !spec.Tools.CertManager.IsEnabled() {
		t.Error("CertManager disabled, want enabled by default")
	}
}

func TestEnvironmentSectionLayeredMerge(t *testing.T) {
	c := &createCmd{}
	for _, content := range []string{
		"environment:\n  providers: [base]\n  crossplane:\n    maxReconcileRate: 2\n",
		"environment:\n  crossplane:\n    maxReconcileRate: 7\n",
	} {
		cfg, err := loadConfig(writeConfig(t, content))
		if err != nil {
			t.Fatalf("loadConfig() unexpected error: %v", err)
		}
		if err := c.mergeConfig(cfg, nil); err != nil {
			t.Fatalf("mergeConfig() unexpected error: %v", err)
		}
	}

	spec, err := c.environmentSpec()
	if err != nil {
		t.Fatalf("environmentSpec() unexpected error: %v", err)
	}
	if spec.Crossplane.MaxReconcileRate != 7 {
		t.Errorf("MaxReconcileRate = %d, want 7", spec.Crossplane.MaxReconcileRate)
	}
	if !reflect.DeepEqual(spec.Providers, []string{"base"}) {
		t.Errorf("Providers = %v, want [base]", spec.Providers)
	}
}

func TestRenderManifest(t *testing.T) {
	disabled := false
	out, err := renderManifest("dev", v1alpha1.EnvironmentSpec{
		Crossplane: v1alpha1.CrossplaneSpec{MaxReconcileRate: 1},
		Providers:  []string{"provider-helm"},
		Tools:      v1alpha1.ToolsSpec{CertManager: v1alpha1.ToolSpec{Enabled: &disabled}},
	})
	if err != nil {
		t.Fatalf("renderManifest() unexpected error: %v", err)
	}

	want := `apiVersion: overlock.io/v1alpha1
kind: Environment
metadata:
  name: dev
spec:
  crossplane:
    maxReconcileRate: 1
  providers:
  - provider-helm
  tools:
    certManager:
      enabled: false
    kyverno: {}
`
	if string(out) != want {
		t.Fatalf("renderManifest() =\n%s\nwant\n%s", out, want)
	}
	if strings.Contains(string(out), "engine") || strings.Contains(string(out), "nodes") {
		t.Fatal("manifest must not contain CLI-specific settings")
	}
}
