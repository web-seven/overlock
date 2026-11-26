package registry

import (
	"reflect"
	"strings"
	"testing"

	"github.com/web-seven/overlock/api/v1alpha1"
	"github.com/web-seven/overlock/internal/engine"
)

func TestRewritePrefix(t *testing.T) {
	for prefix, want := range map[string]string{
		"xpkg.crossplane.io/crossplane-contrib/":    "registry.local/crossplane-contrib/",
		"xpkg.crossplane.io/":                       "registry.local/",
		"registry.example.com:5000/team/provider-x": "registry.local/team/provider-x",
	} {
		got, err := rewritePrefix(prefix, "registry.local")
		if err != nil || got != want {
			t.Errorf("rewritePrefix(%q) = %q, %v; want %q", prefix, got, err, want)
		}
	}
	for _, prefix := range []string{"crossplane-contrib/", "", "/x"} {
		if _, err := rewritePrefix(prefix, "registry.local"); err == nil {
			t.Errorf("rewritePrefix(%q) accepted a prefix without registry host", prefix)
		}
	}
}

func TestLocalRegistryRules(t *testing.T) {
	reg := NewLocal()
	reg.SetRewrites([]string{"xpkg.crossplane.io/crossplane-contrib/"})

	rules, err := reg.environmentRules()
	if err != nil {
		t.Fatalf("environmentRules() unexpected error: %v", err)
	}
	if len(rules) != 1 {
		t.Fatalf("rules = %+v, want one rewrite rule (no auth rule for local)", rules)
	}
	r := rules[0]
	if !strings.HasPrefix(r.Name, "registry-local-rewrite-") || r.PullSecret != "" ||
		!reflect.DeepEqual(r.Prefixes, []string{"xpkg.crossplane.io/crossplane-contrib/"}) ||
		r.RewritePrefix != engine.LocalRegistryDomain()+"/crossplane-contrib/" {
		t.Errorf("unexpected local rewrite rule: %+v", r)
	}
}

func TestRemoteRegistryRules(t *testing.T) {
	reg := New("https://ghcr.io", "user", "pass", "user@example.com")
	reg.Name = "registry-abc"
	reg.SetRewrites([]string{"xpkg.crossplane.io/acme/"})

	rules, err := reg.environmentRules()
	if err != nil {
		t.Fatalf("environmentRules() unexpected error: %v", err)
	}
	want := []v1alpha1.RegistrySpec{
		{Name: "registry-abc", Prefixes: []string{"ghcr.io/"}, PullSecret: "registry-abc"},
		{Name: rules[1].Name, Prefixes: []string{"xpkg.crossplane.io/acme/"}, RewritePrefix: "ghcr.io/acme/", PullSecret: "registry-abc"},
	}
	if !reflect.DeepEqual(rules, want) {
		t.Errorf("rules = %+v, want %+v", rules, want)
	}
}

func TestOtherRules(t *testing.T) {
	reg := Registry{Name: "registry-abc"}
	rules := []v1alpha1.RegistrySpec{
		{Name: "registry-abc"},
		{Name: "registry-abc-rewrite-1234abcd"},
		{Name: "registry-abcd"},
		{Name: "user-rule"},
	}
	var names []string
	for _, r := range reg.otherRules(rules) {
		names = append(names, r.Name)
	}
	if !reflect.DeepEqual(names, []string{"registry-abcd", "user-rule"}) {
		t.Errorf("otherRules() = %v, want rules of other registries and users kept", names)
	}
}

func TestLocalPushName(t *testing.T) {
	local := engine.LocalRegistryDomain() + "/"
	rules := []v1alpha1.RegistrySpec{
		{Name: "acme", Prefixes: []string{"xpkg.crossplane.io/acme/"}, RewritePrefix: local + "acme/"},
		{Name: "acme-dev", Prefixes: []string{"xpkg.crossplane.io/acme/dev/"}, RewritePrefix: local + "acme-dev/"},
		{Name: "mirror", Prefixes: []string{"ghcr.io/team/"}, RewritePrefix: "mirror.example.com/team/"},
		{Name: "auth", Prefixes: []string{"ghcr.io/"}, PullSecret: "ghcr"},
	}
	for name, want := range map[string]string{
		"myconfig:5.0":                                 "myconfig:5.0",
		"acme/myconfig:5.0":                            "acme/myconfig:5.0",
		local + "acme/myconfig:5.0":                    "acme/myconfig:5.0",
		"xpkg.crossplane.io/acme/myconfig:5.0":         "acme/myconfig:5.0",
		"xpkg.crossplane.io/acme/dev/myconfig:5.0":     "acme-dev/myconfig:5.0",
		"xpkg.crossplane.io/acme/nested/provider:v1.0": "acme/nested/provider:v1.0",
	} {
		got, err := localPushName(name, rules)
		if err != nil || got != want {
			t.Errorf("localPushName(%q) = %q, %v; want %q", name, got, err, want)
		}
	}

	for name, hint := range map[string]string{
		"xpkg.crossplane.io/other/config:v1": "--rewrite xpkg.crossplane.io/other/",
		"ghcr.io/team/config:v1":             "--rewrite ghcr.io/team/",
		"ghcr.io/config:v1":                  "--rewrite ghcr.io/",
	} {
		_, err := localPushName(name, rules)
		if err == nil || !strings.Contains(err.Error(), hint) {
			t.Errorf("localPushName(%q) error = %v, want hint %q (no local rewrite)", name, err, hint)
		}
	}
}
