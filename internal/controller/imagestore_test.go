package controller

import (
	"context"
	"reflect"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"

	"github.com/web-seven/overlock/api/v1alpha1"
	"github.com/web-seven/overlock/internal/namespace"
)

func storeEnv(enabled bool) *v1alpha1.Environment {
	return &v1alpha1.Environment{
		ObjectMeta: metav1.ObjectMeta{Name: "dev", Generation: 1},
		Spec: v1alpha1.EnvironmentSpec{
			ImageStore: &v1alpha1.ImageStoreSpec{Enabled: enabled},
			Providers:  []string{providerHelm},
		},
	}
}

func issuedSecret() *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: imageStoreSecret, Namespace: namespace.Namespace, Labels: imageStoreLabels("dev")},
		Data:       map[string][]byte{"ca.crt": []byte("CA"), "tls.crt": []byte("CA"), "tls.key": []byte("KEY")},
	}
}

func TestImageStore(t *testing.T) {
	releases := newFakeReleases()
	r := newTestReconciler(t, releases, storeEnv(true), issuedSecret(), establishedCRD("certificates.cert-manager.io"))
	r.ImageStoreImage = "ghcr.io/web-seven/overlock-operator:test"
	ctx := context.Background()

	// The fake cluster has no cert-manager deployments, so cert-manager is
	// ready from its CRD: the certificate is requested before Crossplane.
	_, got := reconcile(t, r, "dev")

	cert := &unstructured.Unstructured{}
	cert.SetGroupVersionKind(certificateGVK)
	if err := r.Get(ctx, types.NamespacedName{Namespace: namespace.Namespace, Name: imageStoreName}, cert); err != nil {
		t.Fatalf("certificate not created: %v", err)
	}
	dns, _, _ := unstructured.NestedStringSlice(cert.Object, "spec", "dnsNames")
	if !reflect.DeepEqual(dns, []string{"registry", "registry.overlock", "registry.overlock.svc", "registry.overlock.svc.cluster.local"}) {
		t.Errorf("certificate dnsNames = %v", dns)
	}

	cm := &corev1.ConfigMap{}
	if err := r.Get(ctx, types.NamespacedName{Namespace: namespace.Namespace, Name: ImageStoreCA}, cm); err != nil || cm.Data["ca.crt"] != "CA" {
		t.Fatalf("CA config map = %v, %v", cm.Data, err)
	}
	if ca := releases.values[componentCrossplane]["registryCaBundleConfig"]; !reflect.DeepEqual(ca, map[string]any{"name": ImageStoreCA, "key": "ca.crt"}) {
		t.Errorf("crossplane registryCaBundleConfig = %v", ca)
	}

	ds := &appsv1.DaemonSet{}
	if err := r.Get(ctx, types.NamespacedName{Namespace: namespace.Namespace, Name: imageStoreName}, ds); err != nil {
		t.Fatalf("daemon set not created: %v", err)
	}
	c := ds.Spec.Template.Spec.Containers[0]
	if c.Image != r.ImageStoreImage || ds.Spec.Template.Spec.Volumes[0].HostPath.Path != defaultSocketPath {
		t.Errorf("image store container = %s, socket %s", c.Image, ds.Spec.Template.Spec.Volumes[0].HostPath.Path)
	}
	svc := &corev1.Service{}
	if err := r.Get(ctx, types.NamespacedName{Namespace: namespace.Namespace, Name: imageStoreService}, svc); err != nil || !isOwnedBy(svc.Labels, "dev") {
		t.Fatalf("service = %v, %v", svc.Labels, err)
	}

	if st := component(got, componentImageStore); st.State != v1alpha1.ComponentInstalled || st.Ready {
		t.Errorf("image store status = %+v, want installed, not ready", st)
	}
	if st := component(got, "provider/"+providerHelm); st.State != v1alpha1.ComponentPending || st.Message != "waiting for the image store" {
		t.Errorf("provider status = %+v, want pending on the image store", st)
	}

	// Ready on all nodes: packages are installed.
	ds.Status.DesiredNumberScheduled, ds.Status.NumberReady = 1, 1
	if err := r.Status().Update(ctx, ds); err != nil {
		t.Fatal(err)
	}
	_, got = reconcile(t, r, "dev")
	if st := component(got, componentImageStore); !st.Ready {
		t.Errorf("image store status = %+v, want ready", st)
	}
	if st := component(got, "provider/crossplane-contrib-provider-helm"); st.State != v1alpha1.ComponentInstalled {
		t.Errorf("provider status = %+v, want installed", st)
	}

	// Disabling removes the owned objects.
	got.Spec.ImageStore.Enabled = false
	if err := r.Update(ctx, got); err != nil {
		t.Fatal(err)
	}
	reconcile(t, r, "dev")
	if err := r.Get(ctx, types.NamespacedName{Namespace: namespace.Namespace, Name: imageStoreName}, &appsv1.DaemonSet{}); err == nil {
		t.Error("daemon set not removed")
	}
	if err := r.Get(ctx, types.NamespacedName{Namespace: namespace.Namespace, Name: imageStoreService}, &corev1.Service{}); err == nil {
		t.Error("service not removed")
	}
	if err := r.Get(ctx, types.NamespacedName{Namespace: namespace.Namespace, Name: ImageStoreCA}, &corev1.ConfigMap{}); err == nil {
		t.Error("CA config map not removed")
	}
}

func TestImageStoreRefusesForeignService(t *testing.T) {
	foreign := &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: imageStoreService, Namespace: namespace.Namespace}}
	r := newTestReconciler(t, newFakeReleases(), storeEnv(true), foreign)
	r.ImageStoreImage = "img"

	_, got := reconcile(t, r, "dev")

	if st := component(got, componentImageStore); st.State != v1alpha1.ComponentFailed {
		t.Errorf("image store status = %+v, want Failed on the foreign registry service", st)
	}
	svc := &corev1.Service{}
	if err := r.Get(context.Background(), types.NamespacedName{Namespace: namespace.Namespace, Name: imageStoreService}, svc); err != nil || isOwnedBy(svc.Labels, "dev") {
		t.Errorf("foreign service was modified: %v %v", svc.Labels, err)
	}
}
