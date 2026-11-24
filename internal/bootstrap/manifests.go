// Package bootstrap installs the Overlock operator into a cluster and submits
// Environment resources to it.
package bootstrap

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path"
	"sort"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	utilyaml "k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/kustomize/api/krusty"
	"sigs.k8s.io/kustomize/kyaml/filesys"

	"github.com/web-seven/overlock/config"
)

const (
	// ImageRepository is the repository of the operator image.
	ImageRepository = "ghcr.io/web-seven/overlock-operator"
	// Namespace the operator runs in.
	Namespace = "overlock-system"
	// DeploymentName is the name of the operator deployment.
	DeploymentName = "overlock-operator"

	managerContainer = "manager"
	developmentTag   = "latest"
)

// Image returns the operator image matching a CLI version. Development builds
// use the latest image.
func Image(version string) string {
	if version == "" || version == "development" {
		return ImageRepository + ":" + developmentTag
	}
	return ImageRepository + ":" + version
}

// Options customize the operator deployment.
type Options struct {
	// Image of the operator.
	Image string
	// NodeSelector of the operator pod.
	NodeSelector map[string]string
}

// Manifests renders the operator manifests: namespace, CRDs, RBAC and
// deployment, with the image and node selector of opts.
func Manifests(opts Options) ([]*unstructured.Unstructured, error) {
	memFS := filesys.MakeFsInMemory()
	err := fs.WalkDir(config.FS, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, err := config.FS.ReadFile(p)
		if err != nil {
			return err
		}
		return memFS.WriteFile(path.Join("/", p), data)
	})
	if err != nil {
		return nil, fmt.Errorf("failed to load operator manifests: %w", err)
	}

	resMap, err := krusty.MakeKustomizer(krusty.MakeDefaultOptions()).Run(memFS, "/default")
	if err != nil {
		return nil, fmt.Errorf("failed to render operator manifests: %w", err)
	}
	data, err := resMap.AsYaml()
	if err != nil {
		return nil, fmt.Errorf("failed to render operator manifests: %w", err)
	}

	objs, err := decode(data)
	if err != nil {
		return nil, err
	}
	sortForApply(objs)
	for _, obj := range objs {
		if obj.GetKind() == "Deployment" && obj.GetName() == DeploymentName {
			if err := customizeDeployment(obj, opts); err != nil {
				return nil, err
			}
		}
	}
	return objs, nil
}

// customizeDeployment sets the image and node selector of the operator.
func customizeDeployment(obj *unstructured.Unstructured, opts Options) error {
	deploy := &appsv1.Deployment{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, deploy); err != nil {
		return fmt.Errorf("invalid operator deployment: %w", err)
	}
	found := false
	for i := range deploy.Spec.Template.Spec.Containers {
		if deploy.Spec.Template.Spec.Containers[i].Name == managerContainer {
			if opts.Image != "" {
				deploy.Spec.Template.Spec.Containers[i].Image = opts.Image
			}
			found = true
		}
	}
	if !found {
		return fmt.Errorf("operator deployment has no %q container", managerContainer)
	}
	if len(opts.NodeSelector) > 0 {
		deploy.Spec.Template.Spec.NodeSelector = opts.NodeSelector
	}
	u, err := runtime.DefaultUnstructuredConverter.ToUnstructured(deploy)
	if err != nil {
		return err
	}
	// The converter adds empty status and creationTimestamp fields that must
	// not be applied.
	delete(u, "status")
	unstructured.RemoveNestedField(u, "metadata", "creationTimestamp")
	unstructured.RemoveNestedField(u, "spec", "template", "metadata", "creationTimestamp")
	obj.Object = u
	return nil
}

// applyOrder lists kinds that other resources depend on, in the order they
// must be applied: namespaced resources need their Namespace, bindings their
// ServiceAccount and roles, and the Deployment all of them.
var applyOrder = []string{
	"Namespace",
	"CustomResourceDefinition",
	"ServiceAccount",
	"ClusterRole",
	"Role",
	"ClusterRoleBinding",
	"RoleBinding",
}

// sortForApply orders objects so dependencies are applied first. Kinds not in
// applyOrder, such as the Deployment, keep their relative order at the end.
func sortForApply(objs []*unstructured.Unstructured) {
	rank := func(kind string) int {
		for i, k := range applyOrder {
			if k == kind {
				return i
			}
		}
		return len(applyOrder)
	}
	sort.SliceStable(objs, func(i, j int) bool {
		return rank(objs[i].GetKind()) < rank(objs[j].GetKind())
	})
}

func decode(data []byte) ([]*unstructured.Unstructured, error) {
	var objs []*unstructured.Unstructured
	dec := utilyaml.NewYAMLOrJSONDecoder(bytes.NewReader(data), 4096)
	for {
		obj := &unstructured.Unstructured{}
		if err := dec.Decode(&obj.Object); err != nil {
			if errors.Is(err, io.EOF) {
				return objs, nil
			}
			return nil, fmt.Errorf("failed to decode operator manifests: %w", err)
		}
		if len(obj.Object) == 0 || strings.TrimSpace(obj.GetKind()) == "" {
			continue
		}
		objs = append(objs, obj)
	}
}
