package controller

import (
	"context"
	"fmt"

	"github.com/google/go-containerregistry/pkg/name"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/web-seven/overlock/api/v1alpha1"
	"github.com/web-seven/overlock/internal/engine"
)

// packageKind is a kind of Crossplane package.
type packageKind struct {
	Kind string
	// Prefix of the component name in the Environment status.
	Prefix string
}

var packageKinds = []packageKind{
	{Kind: "Provider", Prefix: "provider"},
	{Kind: "Configuration", Prefix: "configuration"},
	{Kind: "Function", Prefix: "function"},
}

func (k packageKind) gvk() schema.GroupVersionKind {
	return schema.GroupVersionKind{Group: "pkg.crossplane.io", Version: "v1", Kind: k.Kind}
}

func (k packageKind) refs(spec v1alpha1.EnvironmentSpec) []string {
	switch k.Kind {
	case "Provider":
		return spec.Providers
	case "Configuration":
		return spec.Configurations
	default:
		return spec.Functions
	}
}

// packageName returns the Kubernetes name of a package reference, the same
// way the CLI names packages: the DNS label of its repository path.
func packageName(ref string) (string, error) {
	r, err := name.ParseReference(ref, name.WithDefaultRegistry(""))
	if err != nil {
		return "", fmt.Errorf("invalid package %q: %w", ref, err)
	}
	return engine.ToDNSLabel(r.Context().RepositoryStr()), nil
}

// reconcilePackages creates or updates the packages of spec and removes
// owned packages that are no longer specified. Packages that exist but are
// not owned by env are left untouched, unless Crossplane is adopted.
func (r *EnvironmentReconciler) reconcilePackages(ctx context.Context, env *v1alpha1.Environment) []v1alpha1.ComponentStatus {
	var statuses []v1alpha1.ComponentStatus
	adopt := env.Spec.Crossplane.Policy == v1alpha1.ManagementAdopt

	for _, kind := range packageKinds {
		desired := map[string]bool{}
		for _, ref := range kind.refs(env.Spec) {
			st := v1alpha1.ComponentStatus{Name: kind.Prefix + "/" + ref}
			pkgName, err := packageName(ref)
			if err != nil {
				statuses = append(statuses, failed(st, err))
				continue
			}
			st.Name = kind.Prefix + "/" + pkgName
			desired[pkgName] = true
			statuses = append(statuses, r.reconcilePackage(ctx, env, kind, pkgName, ref, adopt, st))
		}

		if err := r.removePackages(ctx, env, kind, desired); err != nil {
			statuses = append(statuses, failed(v1alpha1.ComponentStatus{Name: kind.Prefix + "s"}, err))
		}
	}
	return statuses
}

func (r *EnvironmentReconciler) reconcilePackage(ctx context.Context, env *v1alpha1.Environment, kind packageKind, pkgName, ref string, adopt bool, st v1alpha1.ComponentStatus) v1alpha1.ComponentStatus {
	pkg := &unstructured.Unstructured{}
	pkg.SetGroupVersionKind(kind.gvk())
	err := r.Get(ctx, types.NamespacedName{Name: pkgName}, pkg)
	switch {
	case apierrors.IsNotFound(err):
		pkg = &unstructured.Unstructured{}
		pkg.SetGroupVersionKind(kind.gvk())
		pkg.SetName(pkgName)
		pkg.SetLabels(ownerLabels(env.Name))
		if err := unstructured.SetNestedField(pkg.Object, ref, "spec", "package"); err != nil {
			return failed(st, err)
		}
		if err := r.Create(ctx, pkg); err != nil {
			return failed(st, fmt.Errorf("failed to create: %w", err))
		}
	case err != nil:
		return failed(st, fmt.Errorf("failed to get: %w", err))
	case isOwnedBy(pkg.GetLabels(), env.Name) || (adopt && !isOwnedByOther(pkg.GetLabels(), env.Name)):
		current, _, _ := unstructured.NestedString(pkg.Object, "spec", "package")
		if current != ref || !isOwnedBy(pkg.GetLabels(), env.Name) {
			labels := pkg.GetLabels()
			if labels == nil {
				labels = map[string]string{}
			}
			for k, v := range ownerLabels(env.Name) {
				labels[k] = v
			}
			pkg.SetLabels(labels)
			if err := unstructured.SetNestedField(pkg.Object, ref, "spec", "package"); err != nil {
				return failed(st, err)
			}
			if err := r.Update(ctx, pkg); err != nil {
				return failed(st, fmt.Errorf("failed to update: %w", err))
			}
		}
	default:
		st.State = v1alpha1.ComponentExternal
		st.Version, _, _ = unstructured.NestedString(pkg.Object, "spec", "package")
		st.Ready = packageReady(pkg)
		st.Message = "package exists and is not owned by the environment; left untouched"
		return st
	}

	st.State = v1alpha1.ComponentInstalled
	st.Owned = true
	st.Version = ref
	st.Ready = packageReady(pkg)
	if !st.Ready {
		st.Message = "waiting for package to become installed and healthy"
	}
	return st
}

// removePackages deletes packages of kind owned by env that are not desired.
func (r *EnvironmentReconciler) removePackages(ctx context.Context, env *v1alpha1.Environment, kind packageKind, desired map[string]bool) error {
	list := &unstructured.UnstructuredList{}
	list.SetGroupVersionKind(kind.gvk().GroupVersion().WithKind(kind.Kind + "List"))
	if err := r.List(ctx, list, client.MatchingLabels{LabelEnvironment: env.Name}); err != nil {
		return fmt.Errorf("failed to list %s packages: %w", kind.Prefix, err)
	}
	for i := range list.Items {
		pkg := &list.Items[i]
		if desired[pkg.GetName()] {
			continue
		}
		if err := r.Delete(ctx, pkg); client.IgnoreNotFound(err) != nil {
			return fmt.Errorf("failed to delete %s %s: %w", kind.Prefix, pkg.GetName(), err)
		}
	}
	return nil
}

// packagesRemaining reports whether owned packages still exist.
func (r *EnvironmentReconciler) packagesRemaining(ctx context.Context, env *v1alpha1.Environment) (bool, error) {
	for _, kind := range packageKinds {
		list := &unstructured.UnstructuredList{}
		list.SetGroupVersionKind(kind.gvk().GroupVersion().WithKind(kind.Kind + "List"))
		if err := r.List(ctx, list, client.MatchingLabels{LabelEnvironment: env.Name}); err != nil {
			if isNoMatch(err) {
				continue
			}
			return false, err
		}
		if len(list.Items) > 0 {
			return true, nil
		}
	}
	return false, nil
}

// packageReady reports whether a package is installed and healthy.
func packageReady(pkg *unstructured.Unstructured) bool {
	conds, _, _ := unstructured.NestedSlice(pkg.Object, "status", "conditions")
	installed, healthy := false, false
	for _, c := range conds {
		m, ok := c.(map[string]any)
		if !ok || m["status"] != "True" {
			continue
		}
		switch m["type"] {
		case "Installed":
			installed = true
		case "Healthy":
			healthy = true
		}
	}
	return installed && healthy
}
