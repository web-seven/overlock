package controller

import (
	"context"
	"errors"
	"testing"

	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/release"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/web-seven/overlock/api/v1alpha1"
)

// fakeReleases is an in-memory Releases implementation.
type fakeReleases struct {
	releases map[string]*release.Release
	values   map[string]map[string]any
	calls    []string
	failOn   string
	// client, when set, receives the CRD of each installed component, as a
	// real chart installation would create it.
	client client.Client
}

func newFakeReleases() *fakeReleases {
	return &fakeReleases{releases: map[string]*release.Release{}, values: map[string]map[string]any{}}
}

func (f *fakeReleases) Get(c helmComponent) (*release.Release, error) {
	return f.releases[c.Name], nil
}

func (f *fakeReleases) set(c helmComponent, labels map[string]string) {
	if f.values != nil {
		f.values[c.Name] = c.Values
	}
	f.releases[c.Name] = &release.Release{
		Name:   c.Release,
		Labels: labels,
		Info:   &release.Info{Status: release.StatusDeployed},
		Chart:  &chart.Chart{Metadata: &chart.Metadata{Version: c.Version}},
	}
}

func (f *fakeReleases) Install(c helmComponent, labels map[string]string) error {
	f.calls = append(f.calls, "install "+c.Name)
	if f.failOn == c.Name {
		return errors.New("boom")
	}
	f.set(c, labels)
	if f.client != nil {
		if err := f.client.Create(context.Background(), establishedCRD(c.CRD)); client.IgnoreAlreadyExists(err) != nil {
			return err
		}
	}
	return nil
}

func (f *fakeReleases) Upgrade(c helmComponent, labels map[string]string) error {
	f.calls = append(f.calls, "upgrade "+c.Name)
	if f.failOn == c.Name {
		return errors.New("boom")
	}
	f.set(c, labels)
	return nil
}

func (f *fakeReleases) Uninstall(c helmComponent) error {
	f.calls = append(f.calls, "uninstall "+c.Name)
	delete(f.releases, c.Name)
	return nil
}

func establishedCRD(name string) *apiextensionsv1.CustomResourceDefinition {
	return &apiextensionsv1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Status: apiextensionsv1.CustomResourceDefinitionStatus{Conditions: []apiextensionsv1.CustomResourceDefinitionCondition{
			{Type: apiextensionsv1.Established, Status: apiextensionsv1.ConditionTrue},
		}},
	}
}

func newScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	s := runtime.NewScheme()
	for _, add := range []func(*runtime.Scheme) error{clientgoscheme.AddToScheme, apiextensionsv1.AddToScheme, v1alpha1.AddToScheme} {
		if err := add(s); err != nil {
			t.Fatalf("failed to build scheme: %v", err)
		}
	}
	s.AddKnownTypeWithName(runtimeConfigGVK, &unstructured.Unstructured{})
	s.AddKnownTypeWithName(imageConfigGVK, &unstructured.Unstructured{})
	for _, gvk := range []schema.GroupVersionKind{certificateGVK, issuerGVK} {
		s.AddKnownTypeWithName(gvk, &unstructured.Unstructured{})
		s.AddKnownTypeWithName(gvk.GroupVersion().WithKind(gvk.Kind+"List"), &unstructured.UnstructuredList{})
	}
	s.AddKnownTypeWithName(imageConfigGVK.GroupVersion().WithKind("ImageConfigList"), &unstructured.UnstructuredList{})
	for _, k := range packageKinds {
		s.AddKnownTypeWithName(k.gvk(), &unstructured.Unstructured{})
		s.AddKnownTypeWithName(k.gvk().GroupVersion().WithKind(k.Kind+"List"), &unstructured.UnstructuredList{})
	}
	return s
}

func newTestReconciler(t *testing.T, releases *fakeReleases, objs ...client.Object) *EnvironmentReconciler {
	t.Helper()
	s := newScheme(t)
	c := fake.NewClientBuilder().WithScheme(s).WithObjects(objs...).WithStatusSubresource(&v1alpha1.Environment{}).Build()
	releases.client = c
	return &EnvironmentReconciler{Client: c, Scheme: s, Releases: releases}
}

func reconcile(t *testing.T, r *EnvironmentReconciler, name string) (ctrl.Result, *v1alpha1.Environment) {
	t.Helper()
	res, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: name}})
	if err != nil {
		t.Fatalf("Reconcile() unexpected error: %v", err)
	}
	env := &v1alpha1.Environment{}
	if err := r.Get(context.Background(), types.NamespacedName{Name: name}, env); err != nil {
		if client.IgnoreNotFound(err) == nil {
			return res, nil
		}
		t.Fatalf("Get() unexpected error: %v", err)
	}
	return res, env
}

func component(env *v1alpha1.Environment, name string) v1alpha1.ComponentStatus {
	for _, c := range env.Status.Components {
		if c.Name == name {
			return c
		}
	}
	return v1alpha1.ComponentStatus{}
}

func getPackage(t *testing.T, r *EnvironmentReconciler, kind, name string) *unstructured.Unstructured {
	t.Helper()
	pkg := &unstructured.Unstructured{}
	pkg.SetGroupVersionKind(packageKind{Kind: kind}.gvk())
	if err := r.Get(context.Background(), types.NamespacedName{Name: name}, pkg); err != nil {
		if client.IgnoreNotFound(err) == nil {
			return nil
		}
		t.Fatalf("Get(%s) unexpected error: %v", name, err)
	}
	return pkg
}

func markHealthy(t *testing.T, r *EnvironmentReconciler, pkg *unstructured.Unstructured) {
	t.Helper()
	conds := []any{
		map[string]any{"type": "Installed", "status": "True"},
		map[string]any{"type": "Healthy", "status": "True"},
	}
	if err := unstructured.SetNestedSlice(pkg.Object, conds, "status", "conditions"); err != nil {
		t.Fatal(err)
	}
	if err := r.Update(context.Background(), pkg); err != nil {
		t.Fatal(err)
	}
}

const providerHelm = "xpkg.upbound.io/crossplane-contrib/provider-helm:v0.19.0"

func TestReconcileInstallsComponentsInOrder(t *testing.T) {
	releases := newFakeReleases()
	env := &v1alpha1.Environment{ObjectMeta: metav1.ObjectMeta{Name: "dev", Generation: 1}}
	r := newTestReconciler(t, releases, env)

	res, got := reconcile(t, r, "dev")

	want := []string{"install cert-manager", "install crossplane", "install kyverno"}
	if len(releases.calls) != 3 || releases.calls[0] != want[0] || releases.calls[1] != want[1] || releases.calls[2] != want[2] {
		t.Fatalf("calls = %v, want %v", releases.calls, want)
	}
	if !controllerutilContains(got.Finalizers, finalizer) {
		t.Errorf("finalizer not added: %v", got.Finalizers)
	}
	cp := component(got, componentCrossplane)
	if cp.State != v1alpha1.ComponentInstalled || !cp.Owned || !cp.Ready {
		t.Errorf("crossplane status = %+v", cp)
	}
	cond := meta.FindStatusCondition(got.Status.Conditions, v1alpha1.ConditionReady)
	if cond == nil || cond.Status != metav1.ConditionTrue || cond.Reason != ReasonAvailable {
		t.Errorf("Ready condition = %+v, want Available", cond)
	}
	if res.RequeueAfter != requeueAvailable {
		t.Errorf("RequeueAfter = %v, want %v", res.RequeueAfter, requeueAvailable)
	}
	if rel := releases.releases[componentCrossplane]; !isOwnedBy(rel.Labels, "dev") || rel.Labels[labelSpecHash] == "" {
		t.Errorf("release labels = %v", rel.Labels)
	}
}

func TestReconcileIsIdempotentAndUpgradesOnChange(t *testing.T) {
	releases := newFakeReleases()
	env := &v1alpha1.Environment{ObjectMeta: metav1.ObjectMeta{Name: "dev", Generation: 1}}
	r := newTestReconciler(t, releases, env)

	reconcile(t, r, "dev")
	releases.calls = nil
	reconcile(t, r, "dev")
	if len(releases.calls) != 0 {
		t.Fatalf("second reconcile calls = %v, want none", releases.calls)
	}

	_, got := reconcile(t, r, "dev")
	got.Spec.Crossplane.MaxReconcileRate = 5
	if err := r.Update(context.Background(), got); err != nil {
		t.Fatal(err)
	}
	reconcile(t, r, "dev")
	if len(releases.calls) != 1 || releases.calls[0] != "upgrade crossplane" {
		t.Fatalf("calls after spec change = %v, want [upgrade crossplane]", releases.calls)
	}
}

func TestReconcileExistingReleaseNotAdoptedByDefault(t *testing.T) {
	releases := newFakeReleases()
	cp := helmComponents(v1alpha1.EnvironmentSpec{})[1]
	releases.set(cp, nil) // e.g. created by the v1 CLI, without owner labels
	env := &v1alpha1.Environment{ObjectMeta: metav1.ObjectMeta{Name: "dev", Generation: 1}}
	r := newTestReconciler(t, releases, env, establishedCRD("providers.pkg.crossplane.io"))

	_, got := reconcile(t, r, "dev")

	for _, call := range releases.calls {
		if call == "upgrade crossplane" || call == "install crossplane" {
			t.Fatalf("existing crossplane must not be modified, calls = %v", releases.calls)
		}
	}
	st := component(got, componentCrossplane)
	if st.State != v1alpha1.ComponentExternal || st.Owned || !st.Ready {
		t.Errorf("crossplane status = %+v, want ready External", st)
	}
}

func TestReconcileAdoptsExistingRelease(t *testing.T) {
	releases := newFakeReleases()
	cp := helmComponents(v1alpha1.EnvironmentSpec{})[1]
	releases.set(cp, nil)
	env := &v1alpha1.Environment{
		ObjectMeta: metav1.ObjectMeta{Name: "dev", Generation: 1},
		Spec:       v1alpha1.EnvironmentSpec{Crossplane: v1alpha1.CrossplaneSpec{Policy: v1alpha1.ManagementAdopt}},
	}
	r := newTestReconciler(t, releases, env, establishedCRD("providers.pkg.crossplane.io"))

	_, got := reconcile(t, r, "dev")

	st := component(got, componentCrossplane)
	if st.State != v1alpha1.ComponentInstalled || !st.Owned || st.Message != "adopted" {
		t.Errorf("crossplane status = %+v, want adopted", st)
	}
	if !isOwnedBy(releases.releases[componentCrossplane].Labels, "dev") {
		t.Error("adopted release is not labeled as owned")
	}
}

func TestReconcileNeverAdoptsReleaseOfOtherEnvironment(t *testing.T) {
	releases := newFakeReleases()
	cp := helmComponents(v1alpha1.EnvironmentSpec{})[1]
	releases.set(cp, ownerLabels("other"))
	env := &v1alpha1.Environment{
		ObjectMeta: metav1.ObjectMeta{Name: "dev", Generation: 1},
		Spec:       v1alpha1.EnvironmentSpec{Crossplane: v1alpha1.CrossplaneSpec{Policy: v1alpha1.ManagementAdopt}},
	}
	r := newTestReconciler(t, releases, env, establishedCRD("providers.pkg.crossplane.io"))

	_, got := reconcile(t, r, "dev")

	if st := component(got, componentCrossplane); st.State != v1alpha1.ComponentExternal {
		t.Errorf("crossplane status = %+v, want External", st)
	}
	if !isOwnedBy(releases.releases[componentCrossplane].Labels, "other") {
		t.Error("release of another environment was taken over")
	}
}

func TestReconcileDetectsExistingInstallationWithoutRelease(t *testing.T) {
	releases := newFakeReleases()
	env := &v1alpha1.Environment{ObjectMeta: metav1.ObjectMeta{Name: "dev", Generation: 1}}
	r := newTestReconciler(t, releases, env, establishedCRD("certificates.cert-manager.io"))

	_, got := reconcile(t, r, "dev")

	for _, call := range releases.calls {
		if call == "install cert-manager" {
			t.Fatal("cert-manager installed although an existing installation was detected")
		}
	}
	if st := component(got, componentCertManager); st.State != v1alpha1.ComponentExternal || !st.Ready {
		t.Errorf("cert-manager status = %+v, want ready External", st)
	}
}

func TestReconcileExternalPolicyMissing(t *testing.T) {
	releases := newFakeReleases()
	env := &v1alpha1.Environment{
		ObjectMeta: metav1.ObjectMeta{Name: "dev", Generation: 1},
		Spec: v1alpha1.EnvironmentSpec{Tools: v1alpha1.ToolsSpec{
			CertManager: v1alpha1.ToolSpec{Policy: v1alpha1.ManagementExternal},
		}},
	}
	r := newTestReconciler(t, releases, env)

	res, got := reconcile(t, r, "dev")

	if st := component(got, componentCertManager); st.State != v1alpha1.ComponentMissing {
		t.Errorf("cert-manager status = %+v, want Missing", st)
	}
	cond := meta.FindStatusCondition(got.Status.Conditions, v1alpha1.ConditionReady)
	if cond == nil || cond.Reason != ReasonDependencyMissing {
		t.Errorf("Ready condition = %+v, want DependencyMissing", cond)
	}
	if res.RequeueAfter != requeueFailed {
		t.Errorf("RequeueAfter = %v, want %v", res.RequeueAfter, requeueFailed)
	}
}

func TestReconcileRemovesOnlyOwnedDisabledTools(t *testing.T) {
	disabled := false
	releases := newFakeReleases()
	comps := helmComponents(v1alpha1.EnvironmentSpec{})
	releases.set(comps[0], ownerLabels("dev")) // cert-manager owned
	releases.set(comps[2], nil)                // kyverno not owned
	env := &v1alpha1.Environment{
		ObjectMeta: metav1.ObjectMeta{Name: "dev", Generation: 1},
		Spec: v1alpha1.EnvironmentSpec{Tools: v1alpha1.ToolsSpec{
			CertManager: v1alpha1.ToolSpec{Enabled: &disabled},
			Kyverno:     v1alpha1.ToolSpec{Enabled: &disabled},
		}},
	}
	r := newTestReconciler(t, releases, env)

	_, got := reconcile(t, r, "dev")

	if _, ok := releases.releases[componentCertManager]; ok {
		t.Error("owned cert-manager was not removed")
	}
	if _, ok := releases.releases[componentKyverno]; !ok {
		t.Error("kyverno not owned by the environment was removed")
	}
	if st := component(got, componentKyverno); st.State != v1alpha1.ComponentDisabled {
		t.Errorf("kyverno status = %+v, want Disabled", st)
	}
}

func TestReconcileInstallFailure(t *testing.T) {
	releases := newFakeReleases()
	releases.failOn = componentCrossplane
	env := &v1alpha1.Environment{
		ObjectMeta: metav1.ObjectMeta{Name: "dev", Generation: 1},
		Spec:       v1alpha1.EnvironmentSpec{Providers: []string{providerHelm}},
	}
	r := newTestReconciler(t, releases, env)

	res, got := reconcile(t, r, "dev")

	if st := component(got, componentCrossplane); st.State != v1alpha1.ComponentFailed {
		t.Errorf("crossplane status = %+v, want Failed", st)
	}
	if st := component(got, "provider/"+providerHelm); st.State != v1alpha1.ComponentPending {
		t.Errorf("provider status = %+v, want Pending", st)
	}
	cond := meta.FindStatusCondition(got.Status.Conditions, v1alpha1.ConditionReady)
	if cond == nil || cond.Reason != ReasonFailed {
		t.Errorf("Ready condition = %+v, want Failed", cond)
	}
	if res.RequeueAfter != requeueFailed {
		t.Errorf("RequeueAfter = %v, want %v", res.RequeueAfter, requeueFailed)
	}
}

func TestReconcilePackages(t *testing.T) {
	releases := newFakeReleases()
	external := &unstructured.Unstructured{}
	external.SetGroupVersionKind(packageKind{Kind: "Function"}.gvk())
	external.SetName("crossplane-contrib-function-patch-and-transform")
	env := &v1alpha1.Environment{
		ObjectMeta: metav1.ObjectMeta{Name: "dev", Generation: 1},
		Spec: v1alpha1.EnvironmentSpec{
			Providers: []string{providerHelm},
			Functions: []string{"xpkg.upbound.io/crossplane-contrib/function-patch-and-transform:v0.7.0"},
		},
	}
	r := newTestReconciler(t, releases, env, external)

	_, got := reconcile(t, r, "dev")

	pkg := getPackage(t, r, "Provider", "crossplane-contrib-provider-helm")
	if pkg == nil || !isOwnedBy(pkg.GetLabels(), "dev") {
		t.Fatalf("provider not created as owned: %v", pkg)
	}
	if st := component(got, "provider/crossplane-contrib-provider-helm"); st.State != v1alpha1.ComponentInstalled || st.Ready {
		t.Errorf("provider status = %+v, want Installed not ready", st)
	}
	if st := component(got, "function/crossplane-contrib-function-patch-and-transform"); st.State != v1alpha1.ComponentExternal {
		t.Errorf("function status = %+v, want External", st)
	}
	fn := getPackage(t, r, "Function", "crossplane-contrib-function-patch-and-transform")
	if src, _, _ := unstructured.NestedString(fn.Object, "spec", "package"); src != "" || isOwnedBy(fn.GetLabels(), "dev") {
		t.Error("external function was modified")
	}

	markHealthy(t, r, pkg)
	_, got = reconcile(t, r, "dev")
	if st := component(got, "provider/crossplane-contrib-provider-helm"); !st.Ready {
		t.Errorf("provider status = %+v, want ready", st)
	}

	// Removing the provider from the spec deletes the owned package only.
	got.Spec.Providers = nil
	if err := r.Update(context.Background(), got); err != nil {
		t.Fatal(err)
	}
	reconcile(t, r, "dev")
	if getPackage(t, r, "Provider", "crossplane-contrib-provider-helm") != nil {
		t.Error("removed provider still exists")
	}
	if getPackage(t, r, "Function", "crossplane-contrib-function-patch-and-transform") == nil {
		t.Error("external function was deleted")
	}
}

func TestReconcileAdminServiceAccount(t *testing.T) {
	releases := newFakeReleases()
	env := &v1alpha1.Environment{
		ObjectMeta: metav1.ObjectMeta{Name: "dev", Generation: 1},
		Spec:       v1alpha1.EnvironmentSpec{AdminServiceAccount: &v1alpha1.AdminServiceAccountSpec{}},
	}
	r := newTestReconciler(t, releases, env)

	_, got := reconcile(t, r, "dev")
	if st := component(got, componentAdminServiceAccount); st.State != v1alpha1.ComponentInstalled || !st.Ready {
		t.Fatalf("admin service account status = %+v", st)
	}

	got.Spec.AdminServiceAccount = nil
	if err := r.Update(context.Background(), got); err != nil {
		t.Fatal(err)
	}
	_, got = reconcile(t, r, "dev")
	if st := component(got, componentAdminServiceAccount); st.Name != "" {
		t.Errorf("admin service account still reported: %+v", st)
	}
	sas := &unstructured.UnstructuredList{}
	sas.SetAPIVersion("v1")
	sas.SetKind("ServiceAccountList")
	if err := r.List(context.Background(), sas, client.MatchingLabels{LabelEnvironment: "dev"}); err != nil || len(sas.Items) != 0 {
		t.Errorf("owned service accounts left: %d, err %v", len(sas.Items), err)
	}
}

func TestFinalizeOrphanKeepsComponents(t *testing.T) {
	releases := newFakeReleases()
	env := &v1alpha1.Environment{ObjectMeta: metav1.ObjectMeta{Name: "dev", Generation: 1}}
	r := newTestReconciler(t, releases, env)
	reconcile(t, r, "dev")
	releases.calls = nil

	if err := r.Delete(context.Background(), env); err != nil {
		t.Fatal(err)
	}
	_, got := reconcile(t, r, "dev")

	if got != nil {
		t.Errorf("environment still exists with finalizers %v", got.Finalizers)
	}
	if len(releases.calls) != 0 || len(releases.releases) != 3 {
		t.Errorf("orphan deletion changed releases: calls %v", releases.calls)
	}
}

func TestFinalizeDeleteRemovesOwnedComponents(t *testing.T) {
	releases := newFakeReleases()
	comps := helmComponents(v1alpha1.EnvironmentSpec{})
	releases.set(comps[0], nil) // cert-manager installed by someone else
	env := &v1alpha1.Environment{
		ObjectMeta: metav1.ObjectMeta{Name: "dev", Generation: 1},
		Spec: v1alpha1.EnvironmentSpec{
			DeletionPolicy: v1alpha1.DeletionDelete,
			Providers:      []string{providerHelm},
		},
	}
	r := newTestReconciler(t, releases, env)
	reconcile(t, r, "dev")
	if getPackage(t, r, "Provider", "crossplane-contrib-provider-helm") == nil {
		t.Fatal("provider not created")
	}
	releases.calls = nil

	if err := r.Delete(context.Background(), env); err != nil {
		t.Fatal(err)
	}
	_, got := reconcile(t, r, "dev")

	if got != nil {
		t.Fatalf("environment still exists: %+v", got.Status.Conditions)
	}
	if getPackage(t, r, "Provider", "crossplane-contrib-provider-helm") != nil {
		t.Error("owned provider not removed")
	}
	want := []string{"uninstall kyverno", "uninstall crossplane"}
	if len(releases.calls) != 2 || releases.calls[0] != want[0] || releases.calls[1] != want[1] {
		t.Errorf("calls = %v, want %v (reverse order, cert-manager not owned)", releases.calls, want)
	}
	if _, ok := releases.releases[componentCertManager]; !ok {
		t.Error("cert-manager not owned by the environment was removed")
	}
}

func TestReconcileNotFound(t *testing.T) {
	r := newTestReconciler(t, newFakeReleases())
	res, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: "missing"}})
	if err != nil || res.RequeueAfter != 0 {
		t.Fatalf("Reconcile() = %v, %v; want empty result and nil error", res, err)
	}
}

func controllerutilContains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

func TestReconcileWaitsForDeployments(t *testing.T) {
	releases := newFakeReleases()
	cp := helmComponents(v1alpha1.EnvironmentSpec{})[1]
	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "crossplane", Namespace: cp.Namespace, Labels: map[string]string{"app.kubernetes.io/instance": cp.Release}},
		Status: appsv1.DeploymentStatus{Conditions: []appsv1.DeploymentCondition{
			{Type: appsv1.DeploymentAvailable, Status: corev1.ConditionFalse},
		}},
	}
	env := &v1alpha1.Environment{
		ObjectMeta: metav1.ObjectMeta{Name: "dev", Generation: 1},
		Spec:       v1alpha1.EnvironmentSpec{Providers: []string{providerHelm}},
	}
	r := newTestReconciler(t, releases, env, deploy)

	res, got := reconcile(t, r, "dev")

	if st := component(got, componentCrossplane); st.Ready || st.State != v1alpha1.ComponentInstalled {
		t.Errorf("crossplane status = %+v, want installed and not ready", st)
	}
	if st := component(got, "provider/"+providerHelm); st.State != v1alpha1.ComponentPending {
		t.Errorf("provider status = %+v, want Pending", st)
	}
	if res.RequeueAfter != requeueProgressing {
		t.Errorf("RequeueAfter = %v, want %v", res.RequeueAfter, requeueProgressing)
	}

	deploy.Status.Conditions[0].Status = corev1.ConditionTrue
	if err := r.Status().Update(context.Background(), deploy); err != nil {
		t.Fatal(err)
	}
	_, got = reconcile(t, r, "dev")
	if st := component(got, componentCrossplane); !st.Ready {
		t.Errorf("crossplane status = %+v, want ready", st)
	}
	if st := component(got, "provider/crossplane-contrib-provider-helm"); st.State != v1alpha1.ComponentInstalled {
		t.Errorf("provider status = %+v, want Installed", st)
	}
}

func TestReconcileRuntimeConfigScheduling(t *testing.T) {
	releases := newFakeReleases()
	drc := &unstructured.Unstructured{}
	drc.SetGroupVersionKind(runtimeConfigGVK)
	drc.SetName("default")
	if err := unstructured.SetNestedField(drc.Object, "custom-sa", "spec", "serviceAccountTemplate", "metadata", "name"); err != nil {
		t.Fatal(err)
	}
	env := &v1alpha1.Environment{
		ObjectMeta: metav1.ObjectMeta{Name: "dev", Generation: 1},
		Spec: v1alpha1.EnvironmentSpec{Scheduling: &v1alpha1.SchedulingSpec{
			NodeSelector: map[string]string{"overlock.app/scope": "engine"},
		}},
	}
	r := newTestReconciler(t, releases, env, drc)

	_, got := reconcile(t, r, "dev")

	if st := component(got, componentRuntimeConfig); st.State != v1alpha1.ComponentInstalled || !st.Ready {
		t.Errorf("runtime config status = %+v", st)
	}
	updated := &unstructured.Unstructured{}
	updated.SetGroupVersionKind(runtimeConfigGVK)
	if err := r.Get(context.Background(), types.NamespacedName{Name: "default"}, updated); err != nil {
		t.Fatal(err)
	}
	sel, _, _ := unstructured.NestedStringMap(updated.Object, "spec", "deploymentTemplate", "spec", "template", "spec", "nodeSelector")
	if sel["overlock.app/scope"] != "engine" {
		t.Errorf("nodeSelector = %v", sel)
	}
	if sa, _, _ := unstructured.NestedString(updated.Object, "spec", "serviceAccountTemplate", "metadata", "name"); sa != "custom-sa" {
		t.Errorf("unrelated runtime config fields lost: serviceAccountTemplate = %q", sa)
	}

	// Without scheduling the runtime config is not managed.
	got.Spec.Scheduling = nil
	if err := r.Update(context.Background(), got); err != nil {
		t.Fatal(err)
	}
	_, got = reconcile(t, r, "dev")
	if st := component(got, componentRuntimeConfig); st.Name != "" {
		t.Errorf("runtime config reported without scheduling: %+v", st)
	}
}
