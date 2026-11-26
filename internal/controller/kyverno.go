package controller

import (
	"context"
	"fmt"
	"strings"
	"time"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/web-seven/overlock/internal/policy"
)

const (
	kyvernoFinalizerPrefix = "kyverno.io/"
	kyvernoStopTimeout     = 2 * time.Minute
	kyvernoStopInterval    = 2 * time.Second
)

var (
	// kyvernoWebhookSelector matches the webhook configurations Kyverno
	// registers at runtime; they are not part of the Helm release.
	kyvernoWebhookSelector = client.MatchingLabels{"webhook.kyverno.io/managed-by": "kyverno"}
	// kyvernoReleaseSelector matches cluster-scoped objects of the release.
	kyvernoReleaseSelector = client.MatchingLabels{"app.kubernetes.io/instance": policy.KyvernoReleaseName}
)

// prepareKyvernoRemoval makes the Kyverno release removable by Helm without
// the chart's delete hooks.
//
// Kyverno puts finalizers on its deployments, service accounts and roles and
// removes them itself after deleting its webhooks. During "helm uninstall" its
// RBAC is deleted at the same time, so a running Kyverno can no longer remove
// the finalizers and the uninstall never completes. The chart solves this, and
// the removal of config maps kept by "config.preserve", with kubectl delete
// hooks whose image no longer exists. The operator does the same work with its
// own client:
//
//  1. scale Kyverno to zero, so it shuts down gracefully (deleting its
//     webhooks) and cannot add finalizers again,
//  2. delete remaining Kyverno webhook configurations,
//  3. remove Kyverno finalizers,
//  4. delete config maps kept by the release and leftover hook objects,
//
// after which Helm uninstalls the release without hooks.
func (r *EnvironmentReconciler) prepareKyvernoRemoval(ctx context.Context) error {
	if err := r.stopKyverno(ctx); err != nil {
		return err
	}
	if err := r.DeleteAllOf(ctx, &admissionregistrationv1.ValidatingWebhookConfiguration{}, kyvernoWebhookSelector); err != nil {
		return fmt.Errorf("failed to delete Kyverno validating webhooks: %w", err)
	}
	if err := r.DeleteAllOf(ctx, &admissionregistrationv1.MutatingWebhookConfiguration{}, kyvernoWebhookSelector); err != nil {
		return fmt.Errorf("failed to delete Kyverno mutating webhooks: %w", err)
	}

	inNamespace := client.InNamespace(policy.KyvernoNamespace)
	for _, list := range []struct {
		list client.ObjectList
		opts []client.ListOption
	}{
		{&appsv1.DeploymentList{}, []client.ListOption{inNamespace}},
		{&corev1.ServiceAccountList{}, []client.ListOption{inNamespace}},
		{&rbacv1.RoleList{}, []client.ListOption{inNamespace}},
		{&rbacv1.RoleBindingList{}, []client.ListOption{inNamespace}},
		{&rbacv1.ClusterRoleList{}, []client.ListOption{kyvernoReleaseSelector}},
		{&rbacv1.ClusterRoleBindingList{}, []client.ListOption{kyvernoReleaseSelector}},
	} {
		if err := r.removeKyvernoFinalizers(ctx, list.list, list.opts...); err != nil {
			return err
		}
	}
	return r.deleteKyvernoLeftovers(ctx)
}

// stopKyverno scales the Kyverno deployments to zero and waits until their
// pods are gone, or the timeout passes.
func (r *EnvironmentReconciler) stopKyverno(ctx context.Context) error {
	deployments := &appsv1.DeploymentList{}
	if err := r.List(ctx, deployments, client.InNamespace(policy.KyvernoNamespace)); err != nil {
		return fmt.Errorf("failed to list Kyverno deployments: %w", err)
	}
	for i := range deployments.Items {
		d := &deployments.Items[i]
		if d.Spec.Replicas != nil && *d.Spec.Replicas == 0 {
			continue
		}
		patch := client.MergeFrom(d.DeepCopy())
		zero := int32(0)
		d.Spec.Replicas = &zero
		if err := r.Patch(ctx, d, patch); client.IgnoreNotFound(err) != nil {
			return fmt.Errorf("failed to scale down %s: %w", d.Name, err)
		}
	}

	err := wait.PollUntilContextTimeout(ctx, kyvernoStopInterval, kyvernoStopTimeout, true, func(ctx context.Context) (bool, error) {
		pods := &corev1.PodList{}
		if err := r.List(ctx, pods, client.InNamespace(policy.KyvernoNamespace), kyvernoReleaseSelector); err != nil {
			return false, err
		}
		for _, p := range pods.Items {
			if _, isHook := p.Annotations["helm.sh/hook"]; !isHook && p.Labels["job-name"] == "" {
				return false, nil
			}
		}
		return true, nil
	})
	if err != nil && !wait.Interrupted(err) {
		return fmt.Errorf("failed to wait for Kyverno to stop: %w", err)
	}
	// On timeout the remaining steps still make the release removable.
	return nil
}

// removeKyvernoFinalizers removes Kyverno finalizers from the listed objects.
func (r *EnvironmentReconciler) removeKyvernoFinalizers(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	if err := r.List(ctx, list, opts...); err != nil {
		return fmt.Errorf("failed to list %T: %w", list, err)
	}
	items, err := meta.ExtractList(list)
	if err != nil {
		return err
	}
	for _, item := range items {
		obj, ok := item.(client.Object)
		if !ok {
			continue
		}
		var keep []string
		for _, f := range obj.GetFinalizers() {
			if !strings.HasPrefix(f, kyvernoFinalizerPrefix) {
				keep = append(keep, f)
			}
		}
		if len(keep) == len(obj.GetFinalizers()) {
			continue
		}
		patch := client.MergeFrom(obj.DeepCopyObject().(client.Object))
		obj.SetFinalizers(keep)
		if err := r.Patch(ctx, obj, patch); client.IgnoreNotFound(err) != nil {
			return fmt.Errorf("failed to remove finalizers of %s: %w", obj.GetName(), err)
		}
	}
	return nil
}

// deleteKyvernoLeftovers deletes resources of the Kyverno release that Helm
// keeps on uninstall: config maps with the "keep" resource policy and objects
// of hooks that never completed.
func (r *EnvironmentReconciler) deleteKyvernoLeftovers(ctx context.Context) error {
	configMaps := &corev1.ConfigMapList{}
	if err := r.List(ctx, configMaps, client.InNamespace(policy.KyvernoNamespace)); err != nil {
		return fmt.Errorf("failed to list Kyverno config maps: %w", err)
	}
	for i := range configMaps.Items {
		cm := &configMaps.Items[i]
		if cm.Annotations["helm.sh/resource-policy"] != "keep" || cm.Annotations["meta.helm.sh/release-name"] != policy.KyvernoReleaseName {
			continue
		}
		if err := r.Delete(ctx, cm); client.IgnoreNotFound(err) != nil {
			return fmt.Errorf("failed to delete config map %s: %w", cm.Name, err)
		}
	}

	for _, list := range []client.ObjectList{&batchv1.JobList{}, &corev1.ServiceAccountList{}, &rbacv1.RoleList{}, &rbacv1.RoleBindingList{}} {
		if err := r.List(ctx, list, client.InNamespace(policy.KyvernoNamespace)); err != nil {
			return fmt.Errorf("failed to list %T: %w", list, err)
		}
		items, err := meta.ExtractList(list)
		if err != nil {
			return err
		}
		for _, item := range items {
			obj, ok := item.(client.Object)
			if !ok {
				continue
			}
			if _, isHook := obj.GetAnnotations()["helm.sh/hook"]; !isHook {
				continue
			}
			if err := r.Delete(ctx, obj, client.PropagationPolicy(metav1.DeletePropagationBackground)); client.IgnoreNotFound(err) != nil {
				return fmt.Errorf("failed to delete hook object %s: %w", obj.GetName(), err)
			}
		}
	}
	return nil
}
