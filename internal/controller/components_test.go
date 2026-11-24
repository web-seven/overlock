package controller

import (
	"reflect"
	"testing"

	"github.com/web-seven/overlock/api/v1alpha1"
)

func TestHelmComponentsDefaults(t *testing.T) {
	comps := helmComponents(v1alpha1.EnvironmentSpec{})
	names := []string{comps[0].Name, comps[1].Name, comps[2].Name}
	if !reflect.DeepEqual(names, []string{componentCertManager, componentCrossplane, componentKyverno}) {
		t.Fatalf("order = %v", names)
	}
	for _, c := range comps {
		if !c.Enabled || c.Policy != v1alpha1.ManagementAuto || c.Version == "" || c.CRD == "" {
			t.Errorf("unexpected defaults for %s: %+v", c.Name, c)
		}
	}
}

func TestCrossplaneValues(t *testing.T) {
	got := crossplaneValues(v1alpha1.CrossplaneSpec{MaxReconcileRate: 3, Args: []string{"--debug"}})
	want := map[string]any{"args": []any{"--debug", "--max-reconcile-rate=3"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("crossplaneValues() = %v, want %v", got, want)
	}
}

func TestSpecHashChangesWithInput(t *testing.T) {
	a := specHash("1.0.0", map[string]any{"args": []any{}})
	if a != specHash("1.0.0", map[string]any{"args": []any{}}) {
		t.Fatal("specHash is not stable")
	}
	if a == specHash("1.1.0", map[string]any{"args": []any{}}) {
		t.Fatal("specHash ignores version")
	}
	if a == specHash("1.0.0", map[string]any{"args": []any{"--debug"}}) {
		t.Fatal("specHash ignores values")
	}
}

func TestPackageName(t *testing.T) {
	tests := map[string]string{
		"xpkg.upbound.io/crossplane-contrib/provider-helm:v0.19.0":       "crossplane-contrib-provider-helm",
		"xpkg.upbound.io/crossplane-contrib/provider-helm@sha256:" + sha: "crossplane-contrib-provider-helm",
		"registry.local:5000/team/configuration-app:latest":              "team-configuration-app",
	}
	for ref, want := range tests {
		got, err := packageName(ref)
		if err != nil || got != want {
			t.Errorf("packageName(%q) = %q, %v; want %q", ref, got, err, want)
		}
	}
	if _, err := packageName("INVALID REF"); err == nil {
		t.Error("packageName() accepted an invalid reference")
	}
}

const sha = "0000000000000000000000000000000000000000000000000000000000000000"
