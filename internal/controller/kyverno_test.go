package controller

import (
	"context"
	"reflect"
	"testing"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/web-seven/overlock/api/v1alpha1"
	"github.com/web-seven/overlock/internal/policy"
)

func TestDisablingKyvernoPreparesRemoval(t *testing.T) {
	disabled := false
	three := int32(3)
	releases := newFakeReleases()
	releases.set(helmComponents(v1alpha1.EnvironmentSpec{})[2], ownerLabels("dev"))

	kyvernoLabels := map[string]string{"webhook.kyverno.io/managed-by": "kyverno"}
	objs := []client.Object{
		&v1alpha1.Environment{
			ObjectMeta: metav1.ObjectMeta{Name: "dev", Generation: 1},
			Spec:       v1alpha1.EnvironmentSpec{Tools: v1alpha1.ToolsSpec{Kyverno: v1alpha1.ToolSpec{Enabled: &disabled}}},
		},
		&admissionregistrationv1.ValidatingWebhookConfiguration{ObjectMeta: metav1.ObjectMeta{Name: "kyverno-resource-validating-webhook-cfg", Labels: kyvernoLabels}},
		&admissionregistrationv1.MutatingWebhookConfiguration{ObjectMeta: metav1.ObjectMeta{Name: "kyverno-resource-mutating-webhook-cfg", Labels: kyvernoLabels}},
		&admissionregistrationv1.ValidatingWebhookConfiguration{ObjectMeta: metav1.ObjectMeta{Name: "cert-manager-webhook"}},
		&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{
			Name: "kyverno", Namespace: policy.KyvernoNamespace,
			Annotations: map[string]string{"helm.sh/resource-policy": "keep", "meta.helm.sh/release-name": "kyverno"},
		}},
		&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "user-settings", Namespace: policy.KyvernoNamespace}},
		&batchv1.Job{ObjectMeta: metav1.ObjectMeta{
			Name: "kyverno-remove-configmap", Namespace: policy.KyvernoNamespace,
			Annotations: map[string]string{"helm.sh/hook": "post-delete"},
		}},
		&batchv1.Job{ObjectMeta: metav1.ObjectMeta{Name: "user-job", Namespace: policy.KyvernoNamespace}},
		&appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name: "kyverno-admission-controller", Namespace: policy.KyvernoNamespace,
				Finalizers: []string{"kyverno.io/webhooks", "kyverno.io/exceptionwebhooks", "example.com/keep"},
			},
			Spec: appsv1.DeploymentSpec{Replicas: &three},
		},
		&corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{
			Name: "kyverno-admission-controller", Namespace: policy.KyvernoNamespace,
			Finalizers: []string{"kyverno.io/webhooks"},
		}},
		&rbacv1.Role{ObjectMeta: metav1.ObjectMeta{
			Name: "kyverno:admission-controller", Namespace: policy.KyvernoNamespace,
			Finalizers: []string{"kyverno.io/webhooks"},
		}},
		&corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{
			Name: "kyverno-remove-configmap", Namespace: policy.KyvernoNamespace,
			Annotations: map[string]string{"helm.sh/hook": "post-delete"},
		}},
		&rbacv1.RoleBinding{ObjectMeta: metav1.ObjectMeta{
			Name: "kyverno:remove-configmap", Namespace: policy.KyvernoNamespace,
			Annotations: map[string]string{"helm.sh/hook": "post-delete"},
		}},
		&corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: "user-sa", Namespace: policy.KyvernoNamespace}},
	}
	r := newTestReconciler(t, releases, objs...)

	_, got := reconcile(t, r, "dev")

	if st := component(got, componentKyverno); st.State != v1alpha1.ComponentDisabled || st.Message != "removed" {
		t.Fatalf("kyverno status = %+v, want removed", st)
	}
	if _, ok := releases.releases[componentKyverno]; ok {
		t.Error("kyverno release not uninstalled")
	}

	ctx := context.Background()
	vwc := &admissionregistrationv1.ValidatingWebhookConfigurationList{}
	if err := r.List(ctx, vwc); err != nil {
		t.Fatal(err)
	}
	if len(vwc.Items) != 1 || vwc.Items[0].Name != "cert-manager-webhook" {
		t.Errorf("validating webhooks left = %v, want only cert-manager-webhook", vwc.Items)
	}
	mwc := &admissionregistrationv1.MutatingWebhookConfigurationList{}
	if err := r.List(ctx, mwc); err != nil || len(mwc.Items) != 0 {
		t.Errorf("mutating webhooks left = %v, %v", mwc.Items, err)
	}

	cms := &corev1.ConfigMapList{}
	if err := r.List(ctx, cms, client.InNamespace(policy.KyvernoNamespace)); err != nil {
		t.Fatal(err)
	}
	if len(cms.Items) != 1 || cms.Items[0].Name != "user-settings" {
		t.Errorf("config maps left = %v, want only user-settings", cms.Items)
	}
	jobs := &batchv1.JobList{}
	if err := r.List(ctx, jobs, client.InNamespace(policy.KyvernoNamespace)); err != nil {
		t.Fatal(err)
	}
	if len(jobs.Items) != 1 || jobs.Items[0].Name != "user-job" {
		t.Errorf("jobs left = %v, want only user-job", jobs.Items)
	}

	d := &appsv1.Deployment{}
	if err := r.Get(ctx, types.NamespacedName{Namespace: policy.KyvernoNamespace, Name: "kyverno-admission-controller"}, d); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(d.Finalizers, []string{"example.com/keep"}) {
		t.Errorf("finalizers = %v, want only non-Kyverno ones", d.Finalizers)
	}
	if d.Spec.Replicas == nil || *d.Spec.Replicas != 0 {
		t.Errorf("replicas = %v, want 0", d.Spec.Replicas)
	}

	sa := &corev1.ServiceAccount{}
	if err := r.Get(ctx, types.NamespacedName{Namespace: policy.KyvernoNamespace, Name: "kyverno-admission-controller"}, sa); err != nil {
		t.Fatal(err)
	}
	if len(sa.Finalizers) != 0 {
		t.Errorf("service account finalizers = %v", sa.Finalizers)
	}
	role := &rbacv1.Role{}
	if err := r.Get(ctx, types.NamespacedName{Namespace: policy.KyvernoNamespace, Name: "kyverno:admission-controller"}, role); err != nil {
		t.Fatal(err)
	}
	if len(role.Finalizers) != 0 {
		t.Errorf("role finalizers = %v", role.Finalizers)
	}

	sas := &corev1.ServiceAccountList{}
	if err := r.List(ctx, sas, client.InNamespace(policy.KyvernoNamespace)); err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, s := range sas.Items {
		names = append(names, s.Name)
	}
	if !reflect.DeepEqual(names, []string{"kyverno-admission-controller", "user-sa"}) {
		t.Errorf("service accounts = %v, want hook service account deleted", names)
	}
	rbs := &rbacv1.RoleBindingList{}
	if err := r.List(ctx, rbs, client.InNamespace(policy.KyvernoNamespace)); err != nil || len(rbs.Items) != 0 {
		t.Errorf("role bindings left = %v, %v", rbs.Items, err)
	}
}

func TestOnlyKyvernoUninstallsWithoutHooks(t *testing.T) {
	for _, c := range helmComponents(v1alpha1.EnvironmentSpec{}) {
		if c.UninstallWithoutHooks != (c.Name == componentKyverno) {
			t.Errorf("%s UninstallWithoutHooks = %v", c.Name, c.UninstallWithoutHooks)
		}
	}
}
