package controller

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/web-seven/overlock/api/v1alpha1"
)

const (
	componentRegistryPrefix = "registry/"
	imageConfigPrefix       = "overlock-"
)

var imageConfigGVK = schema.GroupVersionKind{Group: "pkg.crossplane.io", Version: "v1beta1", Kind: "ImageConfig"}

// imageConfigName returns the ImageConfig name of a registry rule.
func imageConfigName(reg v1alpha1.RegistrySpec) string {
	return imageConfigPrefix + reg.Name
}

// imageConfigSpec returns the ImageConfig spec of a registry rule.
func imageConfigSpec(reg v1alpha1.RegistrySpec) map[string]any {
	match := make([]any, 0, len(reg.Prefixes))
	for _, p := range reg.Prefixes {
		match = append(match, map[string]any{"type": "Prefix", "prefix": p})
	}
	spec := map[string]any{"matchImages": match}
	if reg.RewritePrefix != "" {
		spec["rewriteImage"] = map[string]any{"prefix": reg.RewritePrefix}
	}
	if reg.PullSecret != "" {
		spec["registry"] = map[string]any{
			"authentication": map[string]any{
				"pullSecretRef": map[string]any{"name": reg.PullSecret},
			},
		}
	}
	return spec
}

// reconcileImageConfigs creates or updates an ImageConfig for each registry
// rule and removes owned ImageConfigs of rules that are no longer specified.
// ImageConfigs that exist but are not owned by env are left untouched.
func (r *EnvironmentReconciler) reconcileImageConfigs(ctx context.Context, env *v1alpha1.Environment) []v1alpha1.ComponentStatus {
	var statuses []v1alpha1.ComponentStatus
	desired := map[string]bool{}
	for _, reg := range env.Spec.Registries {
		name := imageConfigName(reg)
		desired[name] = true
		statuses = append(statuses, r.reconcileImageConfig(ctx, env, name, reg))
	}

	list := &unstructured.UnstructuredList{}
	list.SetGroupVersionKind(imageConfigGVK.GroupVersion().WithKind("ImageConfigList"))
	if err := r.List(ctx, list, client.MatchingLabels{LabelEnvironment: env.Name}); err != nil {
		return append(statuses, failed(v1alpha1.ComponentStatus{Name: componentRegistryPrefix + "*"}, fmt.Errorf("failed to list ImageConfigs: %w", err)))
	}
	for i := range list.Items {
		ic := &list.Items[i]
		if desired[ic.GetName()] {
			continue
		}
		if err := r.Delete(ctx, ic); client.IgnoreNotFound(err) != nil {
			statuses = append(statuses, failed(v1alpha1.ComponentStatus{Name: componentRegistryPrefix + ic.GetName()}, fmt.Errorf("failed to delete: %w", err)))
		}
	}
	return statuses
}

func (r *EnvironmentReconciler) reconcileImageConfig(ctx context.Context, env *v1alpha1.Environment, name string, reg v1alpha1.RegistrySpec) v1alpha1.ComponentStatus {
	st := v1alpha1.ComponentStatus{Name: componentRegistryPrefix + reg.Name}
	spec := imageConfigSpec(reg)

	ic := &unstructured.Unstructured{}
	ic.SetGroupVersionKind(imageConfigGVK)
	err := r.Get(ctx, types.NamespacedName{Name: name}, ic)
	switch {
	case apierrors.IsNotFound(err):
		ic = &unstructured.Unstructured{}
		ic.SetGroupVersionKind(imageConfigGVK)
		ic.SetName(name)
		ic.SetLabels(ownerLabels(env.Name))
		ic.Object["spec"] = spec
		if err := r.Create(ctx, ic); err != nil {
			return failed(st, fmt.Errorf("failed to create ImageConfig %s: %w", name, err))
		}
	case err != nil:
		return failed(st, fmt.Errorf("failed to get ImageConfig %s: %w", name, err))
	case !isOwnedBy(ic.GetLabels(), env.Name):
		st.State = v1alpha1.ComponentExternal
		st.Ready = true
		st.Message = fmt.Sprintf("ImageConfig %s exists and is not owned by the environment; left untouched", name)
		return st
	default:
		current, _, _ := unstructured.NestedMap(ic.Object, "spec")
		if !equality.Semantic.DeepEqual(normalize(current), normalize(spec)) {
			ic.Object["spec"] = spec
			if err := r.Update(ctx, ic); err != nil {
				return failed(st, fmt.Errorf("failed to update ImageConfig %s: %w", name, err))
			}
		}
	}

	st.State = v1alpha1.ComponentInstalled
	st.Owned = true
	st.Ready = true
	st.Message = "ImageConfig " + name
	return st
}

// normalize converts a value to its JSON form, so values built in Go and read
// from the API server compare equal.
func normalize(v any) any {
	u := &unstructured.Unstructured{Object: map[string]any{"v": v}}
	data, err := u.MarshalJSON()
	if err != nil {
		return v
	}
	out := &unstructured.Unstructured{}
	if err := out.UnmarshalJSON(data); err != nil {
		return v
	}
	return out.Object["v"]
}

// removeImageConfigs deletes the ImageConfigs owned by env.
func (r *EnvironmentReconciler) removeImageConfigs(ctx context.Context, env *v1alpha1.Environment) error {
	list := &unstructured.UnstructuredList{}
	list.SetGroupVersionKind(imageConfigGVK.GroupVersion().WithKind("ImageConfigList"))
	if err := r.List(ctx, list, client.MatchingLabels{LabelEnvironment: env.Name}); err != nil {
		if isNoMatch(err) {
			return nil
		}
		return fmt.Errorf("failed to list ImageConfigs: %w", err)
	}
	for i := range list.Items {
		if err := r.Delete(ctx, &list.Items[i]); client.IgnoreNotFound(err) != nil {
			return fmt.Errorf("failed to delete ImageConfig %s: %w", list.Items[i].GetName(), err)
		}
	}
	return nil
}
