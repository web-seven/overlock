package controller

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/web-seven/overlock/api/v1alpha1"
	"github.com/web-seven/overlock/internal/kube"
	"github.com/web-seven/overlock/internal/namespace"
)

// reconcileAdminServiceAccount creates the admin service account and its
// cluster-admin binding, and removes owned ones that are no longer specified.
func (r *EnvironmentReconciler) reconcileAdminServiceAccount(ctx context.Context, env *v1alpha1.Environment) *v1alpha1.ComponentStatus {
	desired := ""
	if env.Spec.AdminServiceAccount != nil {
		desired = env.Spec.AdminServiceAccount.Name
		if desired == "" {
			desired = kube.DefaultAdminServiceAccountName
		}
	}

	if err := r.removeAdminServiceAccounts(ctx, env, desired); err != nil {
		st := failed(v1alpha1.ComponentStatus{Name: componentAdminServiceAccount}, err)
		return &st
	}
	if desired == "" {
		return nil
	}

	st := v1alpha1.ComponentStatus{Name: componentAdminServiceAccount}
	labels := ownerLabels(env.Name)
	labels[labelComponent] = componentAdminServiceAccount

	sa := &corev1.ServiceAccount{}
	err := r.Get(ctx, types.NamespacedName{Namespace: namespace.Namespace, Name: desired}, sa)
	switch {
	case apierrors.IsNotFound(err):
		if err := r.ensureNamespace(ctx, namespace.Namespace); err != nil {
			return ptr(failed(st, err))
		}
		sa = &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: desired, Namespace: namespace.Namespace, Labels: labels}}
		if err := r.Create(ctx, sa); err != nil {
			return ptr(failed(st, fmt.Errorf("failed to create service account: %w", err)))
		}
	case err != nil:
		return ptr(failed(st, fmt.Errorf("failed to get service account: %w", err)))
	case !isOwnedBy(sa.Labels, env.Name):
		st.State = v1alpha1.ComponentExternal
		st.Ready = true
		st.Message = "service account exists and is not owned by the environment; left untouched"
		return &st
	}

	crb := &rbacv1.ClusterRoleBinding{}
	crbName := desired + "-cluster-admin"
	err = r.Get(ctx, types.NamespacedName{Name: crbName}, crb)
	switch {
	case apierrors.IsNotFound(err):
		crb = &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{Name: crbName, Labels: labels},
			Subjects:   []rbacv1.Subject{{Kind: rbacv1.ServiceAccountKind, Name: desired, Namespace: namespace.Namespace}},
			RoleRef:    rbacv1.RoleRef{APIGroup: rbacv1.GroupName, Kind: "ClusterRole", Name: kube.ClusterAdminRole},
		}
		if err := r.Create(ctx, crb); err != nil {
			return ptr(failed(st, fmt.Errorf("failed to create cluster role binding: %w", err)))
		}
	case err != nil:
		return ptr(failed(st, fmt.Errorf("failed to get cluster role binding: %w", err)))
	}

	st.State = v1alpha1.ComponentInstalled
	st.Owned = true
	st.Ready = true
	return &st
}

// removeAdminServiceAccounts deletes owned admin service accounts and
// bindings other than keep. An empty keep removes all of them.
func (r *EnvironmentReconciler) removeAdminServiceAccounts(ctx context.Context, env *v1alpha1.Environment, keep string) error {
	selector := client.MatchingLabels{LabelEnvironment: env.Name, labelComponent: componentAdminServiceAccount}

	sas := &corev1.ServiceAccountList{}
	if err := r.List(ctx, sas, selector); err != nil {
		return fmt.Errorf("failed to list admin service accounts: %w", err)
	}
	for i := range sas.Items {
		if sas.Items[i].Name == keep {
			continue
		}
		if err := r.Delete(ctx, &sas.Items[i]); client.IgnoreNotFound(err) != nil {
			return fmt.Errorf("failed to delete service account %s: %w", sas.Items[i].Name, err)
		}
	}

	crbs := &rbacv1.ClusterRoleBindingList{}
	if err := r.List(ctx, crbs, selector); err != nil {
		return fmt.Errorf("failed to list admin cluster role bindings: %w", err)
	}
	for i := range crbs.Items {
		if keep != "" && crbs.Items[i].Name == keep+"-cluster-admin" {
			continue
		}
		if err := r.Delete(ctx, &crbs.Items[i]); client.IgnoreNotFound(err) != nil {
			return fmt.Errorf("failed to delete cluster role binding %s: %w", crbs.Items[i].Name, err)
		}
	}
	return nil
}

func (r *EnvironmentReconciler) ensureNamespace(ctx context.Context, name string) error {
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: name}}
	if err := r.Create(ctx, ns); err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("failed to create namespace %s: %w", name, err)
	}
	return nil
}

func ptr(st v1alpha1.ComponentStatus) *v1alpha1.ComponentStatus {
	return &st
}
