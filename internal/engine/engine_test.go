package engine

import "testing"

func TestQualifyPackage(t *testing.T) {
	local := LocalRegistryDomain()
	for in, want := range map[string]string{
		"my-provider:v1.0.0":      local + "/my-provider:v1.0.0",
		"team/my-provider:v1.0.0": local + "/team/my-provider:v1.0.0",
		"xpkg.crossplane.io/crossplane-contrib/provider-helm:v1": "xpkg.crossplane.io/crossplane-contrib/provider-helm:v1",
		"localhost:5000/team/provider:v1":                        "localhost:5000/team/provider:v1",
	} {
		got, err := QualifyPackage(in)
		if err != nil || got != want {
			t.Errorf("QualifyPackage(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
}
