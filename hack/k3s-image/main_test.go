package main

import (
	"bytes"
	"reflect"
	"strings"
	"testing"

	"helm.sh/helm/v3/pkg/release"
)

func TestNormalize(t *testing.T) {
	for in, want := range map[string]string{
		"busybox":                               "docker.io/library/busybox:latest",
		"busybox:1.36":                          "docker.io/library/busybox:1.36",
		"alpine/k8s:1.32.3":                     "docker.io/alpine/k8s:1.32.3",
		"docker.io/alpine/k8s:1.32.3":           "docker.io/alpine/k8s:1.32.3",
		"quay.io/jetstack/cert-manager:v1.16.2": "quay.io/jetstack/cert-manager:v1.16.2",
		"localhost/img":                         "localhost/img:latest",
		"registry.local:5000/team/app":          "registry.local:5000/team/app:latest",
		"ghcr.io/org/app@sha256:abc":            "ghcr.io/org/app@sha256:abc",
	} {
		if got := normalize(in); got != want {
			t.Errorf("normalize(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestManifestImages(t *testing.T) {
	manifest := `
apiVersion: apps/v1
kind: Deployment
spec:
  template:
    spec:
      initContainers:
        - image: init:1
      containers:
        - image: quay.io/app:2
---
apiVersion: batch/v1
kind: CronJob
spec:
  jobTemplate:
    spec:
      template:
        spec:
          containers:
            - image: cron:3
---
apiVersion: v1
kind: ConfigMap
data:
  image: not-an-image
`
	got, err := manifestImages(manifest)
	if err != nil {
		t.Fatalf("manifestImages() unexpected error: %v", err)
	}
	want := []string{"docker.io/library/init:1", "quay.io/app:2", "docker.io/library/cron:3"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("manifestImages() = %v, want %v", got, want)
	}
}

func TestIsInstallHook(t *testing.T) {
	for _, e := range []release.HookEvent{release.HookPreInstall, release.HookPostInstall} {
		if !isInstallHook(&release.Hook{Events: []release.HookEvent{release.HookPostUpgrade, e}}) {
			t.Errorf("%s hook not bundled", e)
		}
	}
	for _, e := range []release.HookEvent{release.HookTest, release.HookPostUpgrade, release.HookPreDelete, release.HookPostDelete} {
		if isInstallHook(&release.Hook{Events: []release.HookEvent{e}}) {
			t.Errorf("%s hook bundled", e)
		}
	}
}

func TestRunManifestsAndVersion(t *testing.T) {
	var out bytes.Buffer
	if err := run([]string{"manifests", "--operator-image", "example.com/operator:1"}, &out); err != nil {
		t.Fatalf("manifests: %v", err)
	}
	if !strings.Contains(out.String(), "image: example.com/operator:1") || !strings.Contains(out.String(), "kind: CustomResourceDefinition") {
		t.Errorf("unexpected manifests:\n%s", out.String())
	}

	out.Reset()
	if err := run([]string{"k3s-version"}, &out); err != nil || !strings.HasPrefix(out.String(), "v1.") {
		t.Errorf("k3s-version = %q, %v", out.String(), err)
	}

	if err := run([]string{"images"}, &out); err == nil {
		t.Error("images without --operator-image succeeded")
	}
}
