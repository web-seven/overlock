package bootstrap

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/web-seven/overlock/api/v1alpha1"
)

func newFakeClient(t *testing.T, objs ...client.Object) client.Client {
	t.Helper()
	s := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(s); err != nil {
		t.Fatal(err)
	}
	if err := v1alpha1.AddToScheme(s); err != nil {
		t.Fatal(err)
	}
	return fake.NewClientBuilder().WithScheme(s).WithObjects(objs...).WithStatusSubresource(&v1alpha1.Environment{}).Build()
}

func TestApplyEnvironmentKeepsExisting(t *testing.T) {
	existing := &v1alpha1.Environment{
		ObjectMeta: metav1.ObjectMeta{Name: "dev"},
		Spec:       v1alpha1.EnvironmentSpec{Providers: []string{"keep"}},
	}
	c := newFakeClient(t, existing)

	applied, err := ApplyEnvironment(context.Background(), c, "dev", v1alpha1.EnvironmentSpec{}, false)
	if err != nil || applied {
		t.Fatalf("ApplyEnvironment() = %v, %v; want false, nil", applied, err)
	}
	got := &v1alpha1.Environment{}
	if err := c.Get(context.Background(), client.ObjectKey{Name: "dev"}, got); err != nil {
		t.Fatal(err)
	}
	if len(got.Spec.Providers) != 1 || got.Spec.Providers[0] != "keep" {
		t.Errorf("existing spec changed: %+v", got.Spec)
	}
}

func readyEnvironment(components ...v1alpha1.ComponentStatus) *v1alpha1.Environment {
	return &v1alpha1.Environment{
		ObjectMeta: metav1.ObjectMeta{Name: "dev", Generation: 1},
		Status: v1alpha1.EnvironmentStatus{
			ObservedGeneration: 1,
			Conditions: []metav1.Condition{{
				Type: v1alpha1.ConditionReady, Status: metav1.ConditionTrue, Reason: "Available",
				LastTransitionTime: metav1.Now(),
			}},
			Components: components,
		},
	}
}

func TestWaitReadyReportsComponents(t *testing.T) {
	core, logs := observer.New(zap.InfoLevel)
	c := newFakeClient(t, readyEnvironment(
		v1alpha1.ComponentStatus{Name: "crossplane", State: v1alpha1.ComponentInstalled, Ready: true, Version: "1.19.0"},
		v1alpha1.ComponentStatus{Name: "kyverno", State: v1alpha1.ComponentDisabled},
	))

	if err := WaitReady(context.Background(), c, "dev", 5*time.Second, zap.New(core).Sugar()); err != nil {
		t.Fatalf("WaitReady() unexpected error: %v", err)
	}
	var lines []string
	for _, e := range logs.All() {
		lines = append(lines, e.Message)
	}
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "crossplane: Installed, ready, 1.19.0") || !strings.Contains(joined, "kyverno: Disabled") {
		t.Errorf("unexpected progress output:\n%s", joined)
	}
}

func TestWaitReadyTimeout(t *testing.T) {
	env := readyEnvironment()
	env.Status.Conditions[0].Status = metav1.ConditionFalse
	env.Status.Conditions[0].Reason = "DependencyMissing"
	env.Status.Conditions[0].Message = "cert-manager: not installed"
	c := newFakeClient(t, env)

	err := WaitReady(context.Background(), c, "dev", 3*time.Second, zap.NewNop().Sugar())
	if err == nil || !strings.Contains(err.Error(), "DependencyMissing: cert-manager: not installed") {
		t.Fatalf("WaitReady() error = %v", err)
	}
}

func TestOperatorPodError(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "overlock-operator-x", Namespace: Namespace, Labels: map[string]string{"app.kubernetes.io/name": DeploymentName}},
		Status: corev1.PodStatus{ContainerStatuses: []corev1.ContainerStatus{{
			Name:  "manager",
			State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "ImagePullBackOff", Message: "not found"}},
		}}},
	}
	err := operatorPodError(context.Background(), newFakeClient(t, pod))
	if err == nil || !strings.Contains(err.Error(), "ImagePullBackOff") {
		t.Fatalf("operatorPodError() = %v", err)
	}
	if err := operatorPodError(context.Background(), newFakeClient(t)); err != nil {
		t.Fatalf("operatorPodError() without pods = %v", err)
	}
}

func TestUpdateEnvironment(t *testing.T) {
	c := newFakeClient(t, &v1alpha1.Environment{ObjectMeta: metav1.ObjectMeta{Name: "dev"}})
	err := UpdateEnvironment(context.Background(), c, func(spec *v1alpha1.EnvironmentSpec) {
		spec.Registries = append(spec.Registries, v1alpha1.RegistrySpec{Name: "local", Prefixes: []string{"xpkg.crossplane.io/"}})
	})
	if err != nil {
		t.Fatalf("UpdateEnvironment() unexpected error: %v", err)
	}
	got := &v1alpha1.Environment{}
	if err := c.Get(context.Background(), client.ObjectKey{Name: "dev"}, got); err != nil {
		t.Fatal(err)
	}
	if len(got.Spec.Registries) != 1 || got.Spec.Registries[0].Name != "local" {
		t.Errorf("registries = %+v", got.Spec.Registries)
	}

	if err := UpdateEnvironment(context.Background(), newFakeClient(t), func(*v1alpha1.EnvironmentSpec) {}); !errors.Is(err, errNoEnvironment) {
		t.Errorf("UpdateEnvironment() without Environment = %v, want errNoEnvironment", err)
	}
}
