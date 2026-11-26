package controller

import (
	"context"
	"fmt"

	"github.com/Masterminds/semver/v3"
	"helm.sh/helm/v3/pkg/release"
	appsv1 "k8s.io/api/apps/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/web-seven/overlock/api/v1alpha1"
)

// reconcileHelmComponent brings a Helm component to its desired state and
// returns its observed status. It only installs, upgrades or removes
// releases owned by env, or releases it was explicitly told to adopt.
func (r *EnvironmentReconciler) reconcileHelmComponent(ctx context.Context, env *v1alpha1.Environment, c helmComponent) v1alpha1.ComponentStatus {
	st := v1alpha1.ComponentStatus{Name: c.Name}

	rel, err := r.Releases.Get(c)
	if err != nil {
		return failed(st, fmt.Errorf("failed to get release: %w", err))
	}
	owned := rel != nil && isOwnedBy(rel.Labels, env.Name)

	if !c.Enabled {
		st.State = v1alpha1.ComponentDisabled
		if !owned {
			if rel != nil {
				st.Message = "disabled; existing release is not owned and was left untouched"
			}
			return st
		}
		if err := r.uninstall(ctx, c); err != nil {
			return failed(st, fmt.Errorf("failed to remove: %w", err))
		}
		st.Message = "removed"
		return st
	}

	// External: never install or modify, only observe.
	if c.Policy == v1alpha1.ManagementExternal {
		return r.observeExternal(ctx, c, rel, st)
	}

	if err := checkMinVersion(c); err != nil {
		return failed(st, err)
	}

	labels := ownerLabels(env.Name)
	labels[labelSpecHash] = specHash(c.Version, c.Values)

	switch {
	case owned:
		if rel.Labels[labelSpecHash] != labels[labelSpecHash] || rel.Info == nil || rel.Info.Status != release.StatusDeployed {
			if err := r.Releases.Upgrade(c, labels); err != nil {
				return failed(st, fmt.Errorf("failed to upgrade: %w", err))
			}
		}
	case rel != nil:
		if c.Policy != v1alpha1.ManagementAdopt || isOwnedByOther(rel.Labels, env.Name) {
			return r.observeExternal(ctx, c, rel, st)
		}
		if err := r.Releases.Upgrade(c, labels); err != nil {
			return failed(st, fmt.Errorf("failed to adopt: %w", err))
		}
		st.Message = "adopted"
	default:
		exists, err := r.crdExists(ctx, c.CRD)
		if err != nil {
			return failed(st, err)
		}
		if exists {
			// Installed by other means than the expected Helm release; it
			// cannot be adopted and is used as-is.
			return r.observeExternal(ctx, c, nil, st)
		}
		if err := r.Releases.Install(c, labels); err != nil {
			return failed(st, fmt.Errorf("failed to install: %w", err))
		}
	}

	st.State = v1alpha1.ComponentInstalled
	st.Owned = true
	st.Version = c.Version
	st.Ready, err = r.deploymentsReady(ctx, c)
	if err != nil {
		return failed(st, err)
	}
	if !st.Ready && st.Message == "" {
		st.Message = "waiting for deployments to become available"
	}
	return st
}

// observeExternal reports a component managed outside of the Environment.
func (r *EnvironmentReconciler) observeExternal(ctx context.Context, c helmComponent, rel *release.Release, st v1alpha1.ComponentStatus) v1alpha1.ComponentStatus {
	exists, err := r.crdExists(ctx, c.CRD)
	if err != nil {
		return failed(st, err)
	}
	if rel == nil && !exists {
		st.State = v1alpha1.ComponentMissing
		st.Message = fmt.Sprintf("policy is %s but %s is not installed", c.Policy, c.Name)
		return st
	}
	st.State = v1alpha1.ComponentExternal
	st.Ready = exists
	if rel != nil && rel.Chart != nil && rel.Chart.Metadata != nil {
		st.Version = rel.Chart.Metadata.Version
	}
	st.Message = "managed outside of the environment"
	if !exists {
		st.Message = fmt.Sprintf("waiting for CRD %s", c.CRD)
	}
	return st
}

// uninstall removes the release of c, preparing components that cannot be
// removed by Helm alone.
func (r *EnvironmentReconciler) uninstall(ctx context.Context, c helmComponent) error {
	if c.Name == componentKyverno {
		if err := r.prepareKyvernoRemoval(ctx); err != nil {
			return err
		}
	}
	return r.Releases.Uninstall(c)
}

// removeHelmComponent uninstalls c when its release is owned by env.
func (r *EnvironmentReconciler) removeHelmComponent(ctx context.Context, env *v1alpha1.Environment, c helmComponent) error {
	rel, err := r.Releases.Get(c)
	if err != nil {
		return err
	}
	if rel == nil || !isOwnedBy(rel.Labels, env.Name) {
		return nil
	}
	return r.uninstall(ctx, c)
}

// crdExists reports whether the named CRD exists and is established.
func (r *EnvironmentReconciler) crdExists(ctx context.Context, name string) (bool, error) {
	crd := &apiextensionsv1.CustomResourceDefinition{}
	if err := r.Get(ctx, types.NamespacedName{Name: name}, crd); err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("failed to get CRD %s: %w", name, err)
	}
	for _, cond := range crd.Status.Conditions {
		if cond.Type == apiextensionsv1.Established {
			return cond.Status == apiextensionsv1.ConditionTrue, nil
		}
	}
	return false, nil
}

// deploymentsReady reports whether all deployments of the release of c are
// available.
func (r *EnvironmentReconciler) deploymentsReady(ctx context.Context, c helmComponent) (bool, error) {
	list := &appsv1.DeploymentList{}
	if err := r.List(ctx, list, client.InNamespace(c.Namespace), client.MatchingLabels{"app.kubernetes.io/instance": c.Release}); err != nil {
		return false, fmt.Errorf("failed to list deployments of %s: %w", c.Name, err)
	}
	if len(list.Items) == 0 {
		return r.crdExists(ctx, c.CRD)
	}
	for _, d := range list.Items {
		if !deploymentAvailable(&d) {
			return false, nil
		}
	}
	return true, nil
}

func deploymentAvailable(d *appsv1.Deployment) bool {
	if d.Status.ObservedGeneration < d.Generation {
		return false
	}
	for _, cond := range d.Status.Conditions {
		if cond.Type == appsv1.DeploymentAvailable {
			return cond.Status == "True"
		}
	}
	return false
}

func failed(st v1alpha1.ComponentStatus, err error) v1alpha1.ComponentStatus {
	st.State = v1alpha1.ComponentFailed
	st.Ready = false
	st.Message = err.Error()
	return st
}

// checkMinVersion rejects chart versions older than the component supports.
func checkMinVersion(c helmComponent) error {
	if c.MinVersion == "" {
		return nil
	}
	v, err := semver.NewVersion(c.Version)
	if err != nil {
		return fmt.Errorf("invalid %s version %q: %w", c.Name, c.Version, err)
	}
	if v.LessThan(semver.MustParse(c.MinVersion)) {
		return fmt.Errorf("%s version %s is not supported, at least %s is required", c.Name, c.Version, c.MinVersion)
	}
	return nil
}
