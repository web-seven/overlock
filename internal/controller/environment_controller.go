// Package controller contains the reconcilers of the Overlock operator.
package controller

import (
	"context"
	"time"

	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/web-seven/overlock/api/v1alpha1"
	"github.com/web-seven/overlock/pkg/environment"
)

const (
	reasonInstalled     = "Installed"
	reasonInstallFailed = "InstallFailed"

	retryInterval = time.Minute
)

// InstallFunc installs an environment specification into a cluster.
type InstallFunc func(ctx context.Context, restConfig *rest.Config, spec v1alpha1.EnvironmentSpec, opts environment.InstallOptions, logger *zap.SugaredLogger) error

// EnvironmentReconciler installs Crossplane, packages and tools described by
// an Environment into the cluster the operator runs in.
type EnvironmentReconciler struct {
	client.Client
	Scheme     *runtime.Scheme
	RestConfig *rest.Config
	Logger     *zap.SugaredLogger
	Install    InstallFunc
}

// The operator installs Helm charts (Crossplane, Kyverno, cert-manager) whose
// RBAC requirements are cluster-wide, so its service account is additionally
// bound to cluster-admin in config/rbac/cluster_admin_binding.yaml.
//
// +kubebuilder:rbac:groups=overlock.io,resources=environments,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=overlock.io,resources=environments/status,verbs=get;update;patch

// Reconcile installs the Environment specification and reports the result in
// its Ready condition.
func (r *EnvironmentReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := r.Logger.With("environment", req.Name)

	env := &v1alpha1.Environment{}
	if err := r.Get(ctx, req.NamespacedName, env); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if isReconciled(env) {
		return ctrl.Result{}, nil
	}

	logger.Info("Installing environment")
	installErr := r.Install(ctx, r.RestConfig, env.Spec, environment.InstallOptions{}, logger)

	cond := metav1.Condition{
		Type:               v1alpha1.ConditionReady,
		Status:             metav1.ConditionTrue,
		Reason:             reasonInstalled,
		Message:            "Crossplane, packages and tools are installed",
		ObservedGeneration: env.Generation,
	}
	if installErr != nil {
		cond.Status = metav1.ConditionFalse
		cond.Reason = reasonInstallFailed
		cond.Message = installErr.Error()
	}
	meta.SetStatusCondition(&env.Status.Conditions, cond)
	env.Status.ObservedGeneration = env.Generation

	if err := r.Status().Update(ctx, env); err != nil {
		return ctrl.Result{}, err
	}

	if installErr != nil {
		logger.Errorf("Failed to install environment: %v", installErr)
		return ctrl.Result{RequeueAfter: retryInterval}, nil
	}
	logger.Info("Environment installed")
	return ctrl.Result{}, nil
}

// isReconciled reports whether the current generation is already installed.
func isReconciled(env *v1alpha1.Environment) bool {
	cond := meta.FindStatusCondition(env.Status.Conditions, v1alpha1.ConditionReady)
	return cond != nil &&
		cond.Status == metav1.ConditionTrue &&
		cond.ObservedGeneration == env.Generation
}

// SetupWithManager registers the reconciler with the manager.
func (r *EnvironmentReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if r.Install == nil {
		r.Install = environment.Install
	}
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.Environment{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Complete(r)
}
