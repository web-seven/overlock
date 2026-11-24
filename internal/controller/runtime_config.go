package controller

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"

	"github.com/web-seven/overlock/api/v1alpha1"
)

const componentRuntimeConfig = "runtime-config"

var runtimeConfigGVK = schema.GroupVersionKind{Group: "pkg.crossplane.io", Version: "v1beta1", Kind: "DeploymentRuntimeConfig"}

// reconcileRuntimeConfig applies spec.scheduling to the default
// DeploymentRuntimeConfig, so provider and function runtimes run on the same
// nodes as Crossplane. It returns nil when no scheduling is specified.
func (r *EnvironmentReconciler) reconcileRuntimeConfig(ctx context.Context, env *v1alpha1.Environment) *v1alpha1.ComponentStatus {
	if env.Spec.Scheduling == nil {
		return nil
	}
	st := v1alpha1.ComponentStatus{Name: componentRuntimeConfig}
	nodeSelector, tolerations := schedulingParams(env.Spec.Scheduling)

	drc := &unstructured.Unstructured{}
	drc.SetGroupVersionKind(runtimeConfigGVK)
	err := r.Get(ctx, types.NamespacedName{Name: "default"}, drc)
	switch {
	case apierrors.IsNotFound(err):
		drc = &unstructured.Unstructured{}
		drc.SetGroupVersionKind(runtimeConfigGVK)
		drc.SetName("default")
		if err := setRuntimeScheduling(drc, nodeSelector, tolerations); err != nil {
			return ptr(failed(st, err))
		}
		if err := r.Create(ctx, drc); err != nil {
			return ptr(failed(st, fmt.Errorf("failed to create default DeploymentRuntimeConfig: %w", err)))
		}
	case err != nil:
		return ptr(failed(st, fmt.Errorf("failed to get default DeploymentRuntimeConfig: %w", err)))
	default:
		desired := drc.DeepCopy()
		if err := setRuntimeScheduling(desired, nodeSelector, tolerations); err != nil {
			return ptr(failed(st, err))
		}
		if !equality.Semantic.DeepEqual(desired.Object, drc.Object) {
			drc = desired
			if err := r.Update(ctx, drc); err != nil {
				return ptr(failed(st, fmt.Errorf("failed to update default DeploymentRuntimeConfig: %w", err)))
			}
		}
	}

	st.State = v1alpha1.ComponentInstalled
	st.Ready = true
	return &st
}

// setRuntimeScheduling sets the node selector and tolerations of the package
// runtime pods, keeping the rest of the deployment template.
func setRuntimeScheduling(drc *unstructured.Unstructured, nodeSelector map[string]interface{}, tolerations []interface{}) error {
	spec := []string{"spec", "deploymentTemplate", "spec"}
	if _, found, _ := unstructured.NestedMap(drc.Object, append(spec, "selector")...); !found {
		if err := unstructured.SetNestedMap(drc.Object, map[string]any{}, append(spec, "selector")...); err != nil {
			return err
		}
	}
	pod := append(spec, "template", "spec")
	if _, found, _ := unstructured.NestedSlice(drc.Object, append(pod, "containers")...); !found {
		if err := unstructured.SetNestedSlice(drc.Object, []any{map[string]any{"name": "package-runtime"}}, append(pod, "containers")...); err != nil {
			return err
		}
	}
	if err := unstructured.SetNestedMap(drc.Object, nodeSelector, append(pod, "nodeSelector")...); err != nil {
		return err
	}
	return unstructured.SetNestedSlice(drc.Object, tolerations, append(pod, "tolerations")...)
}
