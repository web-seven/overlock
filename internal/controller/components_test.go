package controller

import (
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"

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

func TestHelmComponentsScheduling(t *testing.T) {
	spec := v1alpha1.EnvironmentSpec{Scheduling: &v1alpha1.SchedulingSpec{
		NodeSelector: map[string]string{"overlock.app/scope": "engine"},
		Tolerations:  []corev1.Toleration{{Key: "dedicated", Operator: corev1.TolerationOpEqual, Value: "engine", Effect: corev1.TaintEffectNoSchedule}},
	}}
	comps := helmComponents(spec)
	wantSelector := map[string]interface{}{"overlock.app/scope": "engine"}
	wantTolerations := []interface{}{map[string]interface{}{"key": "dedicated", "operator": "Equal", "value": "engine", "effect": "NoSchedule"}}

	cp := comps[1].Values
	if !reflect.DeepEqual(cp["nodeSelector"], wantSelector) || !reflect.DeepEqual(cp["tolerations"], wantTolerations) {
		t.Errorf("crossplane values = %v", cp)
	}
	if _, ok := cp["args"]; !ok {
		t.Error("crossplane args lost after merge")
	}

	kyverno := comps[2].Values
	admission, _ := kyverno["admissionController"].(map[string]any)
	if !reflect.DeepEqual(admission["nodeSelector"], wantSelector) {
		t.Errorf("kyverno admissionController = %v", admission)
	}
	if cleanup, _ := kyverno["cleanupController"].(map[string]interface{}); cleanup["enabled"] != false {
		t.Errorf("kyverno base values lost: cleanupController = %v", kyverno["cleanupController"])
	}

	if specHash(comps[1].Version, comps[1].Values) == specHash(helmComponents(v1alpha1.EnvironmentSpec{})[1].Version, helmComponents(v1alpha1.EnvironmentSpec{})[1].Values) {
		t.Error("scheduling does not change the release hash")
	}
}

func TestMergeValues(t *testing.T) {
	dst := map[string]any{"a": map[string]any{"x": 1, "y": 2}, "b": 1}
	src := map[string]any{"a": map[string]any{"y": 3, "z": 4}, "c": 5}
	got := mergeValues(dst, src)
	want := map[string]any{"a": map[string]any{"x": 1, "y": 3, "z": 4}, "b": 1, "c": 5}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("mergeValues() = %v, want %v", got, want)
	}
	if dst["a"].(map[string]any)["y"] != 2 {
		t.Fatal("mergeValues() modified dst")
	}
}
