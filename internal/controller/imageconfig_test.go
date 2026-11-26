package controller

import (
	"context"
	"reflect"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"

	"github.com/web-seven/overlock/api/v1alpha1"
)

func getImageConfig(t *testing.T, r *EnvironmentReconciler, name string) *unstructured.Unstructured {
	t.Helper()
	ic := &unstructured.Unstructured{}
	ic.SetGroupVersionKind(imageConfigGVK)
	if err := r.Get(context.Background(), types.NamespacedName{Name: name}, ic); err != nil {
		return nil
	}
	return ic
}

func TestReconcileImageConfigs(t *testing.T) {
	releases := newFakeReleases()
	external := &unstructured.Unstructured{}
	external.SetGroupVersionKind(imageConfigGVK)
	external.SetName("overlock-team")
	external.Object["spec"] = map[string]any{"matchImages": []any{map[string]any{"prefix": "ghcr.io/team/"}}}

	env := &v1alpha1.Environment{
		ObjectMeta: metav1.ObjectMeta{Name: "dev", Generation: 1},
		Spec: v1alpha1.EnvironmentSpec{
			Registries: []v1alpha1.RegistrySpec{
				{
					Name:          "local",
					Prefixes:      []string{"xpkg.crossplane.io/crossplane-contrib/"},
					RewritePrefix: "registry.overlock.svc.cluster.local/crossplane-contrib/",
				},
				{Name: "private", Prefixes: []string{"ghcr.io/acme/"}, PullSecret: "acme"},
				{Name: "team", Prefixes: []string{"ghcr.io/other/"}},
			},
			Providers: []string{providerHelm},
		},
	}
	r := newTestReconciler(t, releases, env, external)

	_, got := reconcile(t, r, "dev")

	local := getImageConfig(t, r, "overlock-local")
	if local == nil || !isOwnedBy(local.GetLabels(), "dev") {
		t.Fatalf("ImageConfig overlock-local not created as owned: %v", local)
	}
	wantLocal := map[string]any{
		"matchImages":  []any{map[string]any{"type": "Prefix", "prefix": "xpkg.crossplane.io/crossplane-contrib/"}},
		"rewriteImage": map[string]any{"prefix": "registry.overlock.svc.cluster.local/crossplane-contrib/"},
	}
	if !reflect.DeepEqual(local.Object["spec"], wantLocal) {
		t.Errorf("overlock-local spec = %v, want %v", local.Object["spec"], wantLocal)
	}
	private := getImageConfig(t, r, "overlock-private")
	if name, _, _ := unstructured.NestedString(private.Object, "spec", "registry", "authentication", "pullSecretRef", "name"); name != "acme" {
		t.Errorf("overlock-private pullSecretRef = %q, want acme", name)
	}
	if st := component(got, "registry/team"); st.State != v1alpha1.ComponentExternal {
		t.Errorf("registry/team status = %+v, want External", st)
	}
	if spec := getImageConfig(t, r, "overlock-team").Object["spec"]; !reflect.DeepEqual(spec, external.Object["spec"]) {
		t.Errorf("external ImageConfig changed: %v", spec)
	}
	if st := component(got, "provider/crossplane-contrib-provider-helm"); st.State != v1alpha1.ComponentInstalled {
		t.Errorf("provider status = %+v, want Installed after registry rules", st)
	}

	// Changing and removing rules updates and deletes owned ImageConfigs only.
	got.Spec.Registries = []v1alpha1.RegistrySpec{{Name: "local", Prefixes: []string{"xpkg.crossplane.io/"}, RewritePrefix: "registry.local/"}}
	if err := r.Update(context.Background(), got); err != nil {
		t.Fatal(err)
	}
	reconcile(t, r, "dev")
	if p, _, _ := unstructured.NestedString(getImageConfig(t, r, "overlock-local").Object, "spec", "rewriteImage", "prefix"); p != "registry.local/" {
		t.Errorf("overlock-local rewrite prefix = %q, want registry.local/", p)
	}
	if getImageConfig(t, r, "overlock-private") != nil {
		t.Error("removed rule still has an ImageConfig")
	}
	if getImageConfig(t, r, "overlock-team") == nil {
		t.Error("external ImageConfig was deleted")
	}
}

func TestCrossplaneMinVersion(t *testing.T) {
	releases := newFakeReleases()
	env := &v1alpha1.Environment{
		ObjectMeta: metav1.ObjectMeta{Name: "dev", Generation: 1},
		Spec:       v1alpha1.EnvironmentSpec{Crossplane: v1alpha1.CrossplaneSpec{Version: "1.20.15"}},
	}
	r := newTestReconciler(t, releases, env)

	_, got := reconcile(t, r, "dev")

	st := component(got, componentCrossplane)
	if st.State != v1alpha1.ComponentFailed || st.Message != "crossplane version 1.20.15 is not supported, at least 2.0.0 is required" {
		t.Errorf("crossplane status = %+v", st)
	}
	for _, call := range releases.calls {
		if call == "install crossplane" {
			t.Error("unsupported Crossplane version was installed")
		}
	}
}

func TestCrossplaneValuesCABundle(t *testing.T) {
	values := crossplaneValues(v1alpha1.CrossplaneSpec{RegistryCABundle: &v1alpha1.ConfigMapKey{Name: "registry-ca", Key: "ca.crt"}})
	want := map[string]any{"name": "registry-ca", "key": "ca.crt"}
	if !reflect.DeepEqual(values["registryCaBundleConfig"], want) {
		t.Errorf("registryCaBundleConfig = %v, want %v", values["registryCaBundleConfig"], want)
	}
}

func TestImageStoreCacheRules(t *testing.T) {
	spec := v1alpha1.EnvironmentSpec{ImageStore: &v1alpha1.ImageStoreSpec{Enabled: true, Cache: []string{"xpkg.crossplane.io/"}}}
	rules := cacheRules(spec)
	if len(rules) != 1 || rules[0].RewritePrefix != "registry.overlock.svc.cluster.local/xpkg.crossplane.io/" ||
		!reflect.DeepEqual(rules[0].Prefixes, []string{"xpkg.crossplane.io/"}) {
		t.Fatalf("cacheRules() = %+v", rules)
	}
	spec.ImageStore.Enabled = false
	if rules := cacheRules(spec); len(rules) != 0 {
		t.Errorf("cache rules without image store = %+v", rules)
	}
}
