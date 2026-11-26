package packages

import (
	"context"
	"testing"

	"github.com/web-seven/overlock/internal/engine"
)

func TestUpgradeVersionMatchesLocalPackages(t *testing.T) {
	p := &Package{}
	deployed := []Package{
		{Url: engine.LocalRegistryDomain() + "/my-provider:v1.2.5"},
		{Url: "xpkg.crossplane.io/other/my-provider:v1.2.9"},
	}
	got, err := p.UpgradeVersion(context.Background(), nil, "my-provider:v1.2.0", deployed)
	if err != nil {
		t.Fatalf("UpgradeVersion() unexpected error: %v", err)
	}
	if got != "my-provider:v1.2.6" {
		t.Errorf("UpgradeVersion() = %q, want my-provider:v1.2.6 (next patch of the local deployed version, short name kept for push)", got)
	}
}

func TestUpgradeVersionKeepsPrefix(t *testing.T) {
	p := &Package{}
	deployed := []Package{{Url: "xpkg.crossplane.io/acme/config:v5.0.3"}}
	for requested, want := range map[string]string{
		"xpkg.crossplane.io/acme/config:v5.0": "xpkg.crossplane.io/acme/config:v5.0.4",
		"xpkg.crossplane.io/acme/config:5.0":  "xpkg.crossplane.io/acme/config:5.0.4",
		"xpkg.crossplane.io/acme/config:v6.1": "xpkg.crossplane.io/acme/config:v6.1.1",
	} {
		got, err := p.UpgradeVersion(context.Background(), nil, requested, deployed)
		if err != nil || got != want {
			t.Errorf("UpgradeVersion(%q) = %q, %v; want %q", requested, got, err, want)
		}
	}
}
