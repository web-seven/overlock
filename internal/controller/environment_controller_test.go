package controller

import (
	"context"
	"errors"
	"testing"

	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/web-seven/overlock/api/v1alpha1"
	"github.com/web-seven/overlock/pkg/environment"
)

func newReconciler(t *testing.T, env *v1alpha1.Environment, install InstallFunc) *EnvironmentReconciler {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := v1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme() unexpected error: %v", err)
	}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(env).WithStatusSubresource(env).Build()
	return &EnvironmentReconciler{Client: c, Scheme: scheme, Logger: zap.NewNop().Sugar(), Install: install}
}

func reconcileEnv(t *testing.T, r *EnvironmentReconciler, name string) (ctrl.Result, *v1alpha1.Environment) {
	t.Helper()
	res, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: name}})
	if err != nil {
		t.Fatalf("Reconcile() unexpected error: %v", err)
	}
	got := &v1alpha1.Environment{}
	if err := r.Get(context.Background(), types.NamespacedName{Name: name}, got); err != nil {
		t.Fatalf("Get() unexpected error: %v", err)
	}
	return res, got
}

func TestReconcileInstallsSpec(t *testing.T) {
	env := &v1alpha1.Environment{
		ObjectMeta: metav1.ObjectMeta{Name: "dev", Generation: 1},
		Spec:       v1alpha1.EnvironmentSpec{Providers: []string{"provider-helm"}},
	}
	calls := 0
	r := newReconciler(t, env, func(_ context.Context, _ *rest.Config, spec v1alpha1.EnvironmentSpec, _ environment.InstallOptions, _ *zap.SugaredLogger) error {
		calls++
		if len(spec.Providers) != 1 || spec.Providers[0] != "provider-helm" {
			t.Errorf("Install() spec.Providers = %v", spec.Providers)
		}
		return nil
	})

	res, got := reconcileEnv(t, r, "dev")
	if res.RequeueAfter != 0 {
		t.Errorf("RequeueAfter = %v, want 0", res.RequeueAfter)
	}
	if !meta.IsStatusConditionTrue(got.Status.Conditions, v1alpha1.ConditionReady) {
		t.Errorf("Ready condition not true: %+v", got.Status.Conditions)
	}
	if got.Status.ObservedGeneration != got.Generation {
		t.Errorf("ObservedGeneration = %d, want %d", got.Status.ObservedGeneration, got.Generation)
	}

	// The same generation is not installed twice.
	reconcileEnv(t, r, "dev")
	if calls != 1 {
		t.Errorf("Install() called %d times, want 1", calls)
	}
}

func TestReconcileInstallFailure(t *testing.T) {
	env := &v1alpha1.Environment{ObjectMeta: metav1.ObjectMeta{Name: "dev", Generation: 1}}
	r := newReconciler(t, env, func(context.Context, *rest.Config, v1alpha1.EnvironmentSpec, environment.InstallOptions, *zap.SugaredLogger) error {
		return errors.New("boom")
	})

	res, got := reconcileEnv(t, r, "dev")
	if res.RequeueAfter != retryInterval {
		t.Errorf("RequeueAfter = %v, want %v", res.RequeueAfter, retryInterval)
	}
	cond := meta.FindStatusCondition(got.Status.Conditions, v1alpha1.ConditionReady)
	if cond == nil || cond.Status != metav1.ConditionFalse || cond.Reason != reasonInstallFailed || cond.Message != "boom" {
		t.Errorf("unexpected Ready condition: %+v", cond)
	}
}

func TestReconcileNotFound(t *testing.T) {
	env := &v1alpha1.Environment{ObjectMeta: metav1.ObjectMeta{Name: "other"}}
	r := newReconciler(t, env, nil)
	res, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: "missing"}})
	if err != nil || res.RequeueAfter != 0 {
		t.Fatalf("Reconcile() = %v, %v; want empty result and nil error", res, err)
	}
}
