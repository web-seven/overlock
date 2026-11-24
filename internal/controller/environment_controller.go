// Package controller contains the reconcilers of the Overlock operator.
package controller

import (
	"context"
	"fmt"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/web-seven/overlock/api/v1alpha1"
)

// Reasons of the Ready condition.
const (
	ReasonAvailable         = "Available"
	ReasonProgressing       = "Progressing"
	ReasonDependencyMissing = "DependencyMissing"
	ReasonFailed            = "Failed"
	ReasonDeleting          = "Deleting"
)

// Requeue intervals.
const (
	requeueProgressing = 15 * time.Second
	requeueFailed      = time.Minute
	requeueAvailable   = 10 * time.Minute
)

// EnvironmentReconciler installs and reconciles Crossplane, its packages and
// tools described by an Environment in the cluster the operator runs in.
type EnvironmentReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Releases Releases
}

// The operator installs Helm charts (Crossplane, Kyverno, cert-manager) whose
// RBAC requirements are cluster-wide, so its service account is additionally
// bound to cluster-admin in config/rbac/cluster_admin_binding.yaml.
//
// +kubebuilder:rbac:groups=overlock.io,resources=environments,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=overlock.io,resources=environments/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=overlock.io,resources=environments/finalizers,verbs=update

// Reconcile brings the cluster to the state described by an Environment.
func (r *EnvironmentReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	env := &v1alpha1.Environment{}
	if err := r.Get(ctx, req.NamespacedName, env); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if !env.DeletionTimestamp.IsZero() {
		return r.finalize(ctx, env)
	}

	if controllerutil.AddFinalizer(env, finalizer) {
		if err := r.Update(ctx, env); err != nil {
			return ctrl.Result{}, err
		}
	}

	components := r.reconcileComponents(ctx, env)
	cond := readyCondition(env.Generation, components)

	env.Status.Components = components
	env.Status.ObservedGeneration = env.Generation
	meta.SetStatusCondition(&env.Status.Conditions, cond)
	if err := r.Status().Update(ctx, env); err != nil {
		return ctrl.Result{}, err
	}

	logger.Info("Reconciled environment", "ready", cond.Status, "reason", cond.Reason)
	switch cond.Reason {
	case ReasonAvailable:
		return ctrl.Result{RequeueAfter: requeueAvailable}, nil
	case ReasonProgressing:
		return ctrl.Result{RequeueAfter: requeueProgressing}, nil
	default:
		return ctrl.Result{RequeueAfter: requeueFailed}, nil
	}
}

// reconcileComponents reconciles all components in dependency order and
// returns their statuses. Packages are reconciled only once Crossplane is
// ready, because they need its CRDs and package manager.
func (r *EnvironmentReconciler) reconcileComponents(ctx context.Context, env *v1alpha1.Environment) []v1alpha1.ComponentStatus {
	var statuses []v1alpha1.ComponentStatus
	crossplaneReady := false
	for _, c := range helmComponents(env.Spec) {
		st := r.reconcileHelmComponent(ctx, env, c)
		if c.Name == componentCrossplane {
			crossplaneReady = st.Ready
		}
		statuses = append(statuses, st)
	}

	if crossplaneReady {
		statuses = append(statuses, r.reconcilePackages(ctx, env)...)
	} else {
		for _, kind := range packageKinds {
			for _, ref := range kind.refs(env.Spec) {
				statuses = append(statuses, v1alpha1.ComponentStatus{
					Name:    kind.Prefix + "/" + ref,
					State:   v1alpha1.ComponentPending,
					Message: "waiting for Crossplane to become ready",
				})
			}
		}
	}

	if st := r.reconcileAdminServiceAccount(ctx, env); st != nil {
		statuses = append(statuses, *st)
	}
	return statuses
}

// readyCondition summarizes component statuses into the Ready condition.
func readyCondition(generation int64, components []v1alpha1.ComponentStatus) metav1.Condition {
	var failedMsgs, missingMsgs, waiting []string
	for _, c := range components {
		switch {
		case c.State == v1alpha1.ComponentFailed:
			failedMsgs = append(failedMsgs, c.Name+": "+c.Message)
		case c.State == v1alpha1.ComponentMissing:
			missingMsgs = append(missingMsgs, c.Name+": "+c.Message)
		case c.State != v1alpha1.ComponentDisabled && !c.Ready:
			waiting = append(waiting, c.Name)
		}
	}

	cond := metav1.Condition{
		Type:               v1alpha1.ConditionReady,
		Status:             metav1.ConditionFalse,
		ObservedGeneration: generation,
	}
	switch {
	case len(failedMsgs) > 0:
		cond.Reason = ReasonFailed
		cond.Message = strings.Join(failedMsgs, "; ")
	case len(missingMsgs) > 0:
		cond.Reason = ReasonDependencyMissing
		cond.Message = strings.Join(missingMsgs, "; ")
	case len(waiting) > 0:
		cond.Reason = ReasonProgressing
		cond.Message = "waiting for " + strings.Join(waiting, ", ")
	default:
		cond.Status = metav1.ConditionTrue
		cond.Reason = ReasonAvailable
		cond.Message = "all components are ready"
	}
	return cond
}

// finalize applies the deletion policy and releases the Environment. Only
// components owned by the Environment are removed, and only with the Delete
// policy; everything else stays in the cluster.
func (r *EnvironmentReconciler) finalize(ctx context.Context, env *v1alpha1.Environment) (ctrl.Result, error) {
	if !controllerutil.ContainsFinalizer(env, finalizer) {
		return ctrl.Result{}, nil
	}

	if env.Spec.DeletionPolicy == v1alpha1.DeletionDelete {
		done, err := r.removeOwned(ctx, env)
		if err != nil || !done {
			msg := "waiting for owned packages to be removed"
			if err != nil {
				msg = err.Error()
			}
			meta.SetStatusCondition(&env.Status.Conditions, metav1.Condition{
				Type:               v1alpha1.ConditionReady,
				Status:             metav1.ConditionFalse,
				Reason:             ReasonDeleting,
				Message:            msg,
				ObservedGeneration: env.Generation,
			})
			if uerr := r.Status().Update(ctx, env); uerr != nil {
				return ctrl.Result{}, uerr
			}
			if err != nil {
				return ctrl.Result{RequeueAfter: requeueFailed}, nil
			}
			return ctrl.Result{RequeueAfter: requeueProgressing}, nil
		}
	}

	controllerutil.RemoveFinalizer(env, finalizer)
	return ctrl.Result{}, r.Update(ctx, env)
}

// removeOwned removes owned packages, then the admin service account, then
// owned Helm releases in reverse installation order. It reports false while
// packages are still being removed, so Crossplane is not uninstalled before
// its packages are gone.
func (r *EnvironmentReconciler) removeOwned(ctx context.Context, env *v1alpha1.Environment) (bool, error) {
	for _, kind := range packageKinds {
		if err := r.removePackages(ctx, env, kind, nil); err != nil && !isNoMatch(err) {
			return false, err
		}
	}
	remaining, err := r.packagesRemaining(ctx, env)
	if err != nil || remaining {
		return false, err
	}

	if err := r.removeAdminServiceAccounts(ctx, env, ""); err != nil {
		return false, err
	}

	components := helmComponents(env.Spec)
	for i := len(components) - 1; i >= 0; i-- {
		if err := r.removeHelmComponent(env, components[i]); err != nil {
			return false, fmt.Errorf("failed to remove %s: %w", components[i].Name, err)
		}
	}
	return true, nil
}

func isNoMatch(err error) bool {
	for e := err; e != nil; {
		if meta.IsNoMatchError(e) {
			return true
		}
		u, ok := e.(interface{ Unwrap() error })
		if !ok {
			return false
		}
		e = u.Unwrap()
	}
	return false
}

// SetupWithManager registers the reconciler with the manager. Helm
// operations are long-running, so environments are reconciled one at a time.
func (r *EnvironmentReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.Environment{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Complete(r)
}
