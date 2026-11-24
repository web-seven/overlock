package bootstrap

import (
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestImage(t *testing.T) {
	for version, want := range map[string]string{
		"":            ImageRepository + ":latest",
		"development": ImageRepository + ":latest",
		"1.2.3":       ImageRepository + ":1.2.3",
	} {
		if got := Image(version); got != want {
			t.Errorf("Image(%q) = %q, want %q", version, got, want)
		}
	}
}

func TestManifests(t *testing.T) {
	objs, err := Manifests(Options{Image: "example.com/operator:test", NodeSelector: map[string]string{"overlock.app/scope": "engine"}})
	if err != nil {
		t.Fatalf("Manifests() unexpected error: %v", err)
	}

	kinds := map[string]*unstructured.Unstructured{}
	for _, o := range objs {
		kinds[o.GetKind()+"/"+o.GetName()] = o
	}
	for _, want := range []string{
		"Namespace/" + Namespace,
		"CustomResourceDefinition/environments.overlock.app",
		"ServiceAccount/overlock-operator",
		"ClusterRole/overlock-operator",
		"ClusterRoleBinding/overlock-operator",
		"ClusterRoleBinding/overlock-operator-cluster-admin",
		"Deployment/" + DeploymentName,
	} {
		if kinds[want] == nil {
			t.Errorf("missing %s in %v", want, keys(kinds))
		}
	}

	deploy := kinds["Deployment/"+DeploymentName]
	if deploy.GetNamespace() != Namespace {
		t.Errorf("deployment namespace = %q, want %q", deploy.GetNamespace(), Namespace)
	}
	containers, _, _ := unstructured.NestedSlice(deploy.Object, "spec", "template", "spec", "containers")
	if image := containers[0].(map[string]any)["image"]; image != "example.com/operator:test" {
		t.Errorf("image = %v", image)
	}
	sel, _, _ := unstructured.NestedStringMap(deploy.Object, "spec", "template", "spec", "nodeSelector")
	if sel["overlock.app/scope"] != "engine" {
		t.Errorf("nodeSelector = %v", sel)
	}
	if _, found, _ := unstructured.NestedFieldNoCopy(deploy.Object, "status"); found {
		t.Error("deployment contains status")
	}
}

func keys(m map[string]*unstructured.Unstructured) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
