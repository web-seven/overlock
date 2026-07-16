package environment

import "testing"

func TestK3sImage(t *testing.T) {
	tests := []struct {
		name    string
		version string
		want    string
		wantErr bool
	}{
		{name: "default when empty", version: "", want: k3sDockerImageRepo + ":" + k3sDockerDefaultVersion},
		{name: "upstream release format", version: "v1.33.11+k3s1", want: "rancher/k3s:v1.33.11-k3s1"},
		{name: "docker tag format", version: "v1.34.7-k3s1", want: "rancher/k3s:v1.34.7-k3s1"},
		{name: "surrounding whitespace", version: " v1.35.4+k3s1 ", want: "rancher/k3s:v1.35.4-k3s1"},
		{name: "missing v prefix", version: "1.33.11+k3s1", wantErr: true},
		{name: "missing k3s revision", version: "v1.33.11", wantErr: true},
		{name: "crossplane version", version: "1.20.4", wantErr: true},
		{name: "arbitrary tag", version: "latest", wantErr: true},
		{name: "injection attempt", version: "v1.33.11+k3s1 --evil", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &Environment{k3sVersion: tt.version}
			got, err := e.k3sImage()
			if tt.wantErr {
				if err == nil {
					t.Fatalf("k3sImage(%q) expected error, got %q", tt.version, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("k3sImage(%q) unexpected error: %v", tt.version, err)
			}
			if got != tt.want {
				t.Fatalf("k3sImage(%q) = %q, want %q", tt.version, got, tt.want)
			}
		})
	}
}
