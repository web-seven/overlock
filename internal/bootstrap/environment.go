package bootstrap

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/web-seven/overlock/api/v1alpha1"
)

// operatorStartTimeout is how long WaitReady waits for the operator to report
// any status before checking its pods for image or scheduling problems.
const operatorStartTimeout = time.Minute

// ApplyEnvironment submits an Environment. With overwrite, its spec is
// server-side applied; otherwise an existing Environment is left unchanged.
// It reports whether the Environment was created or changed.
func ApplyEnvironment(ctx context.Context, c client.Client, name string, spec v1alpha1.EnvironmentSpec, overwrite bool) (bool, error) {
	if !overwrite {
		existing := &v1alpha1.Environment{}
		err := c.Get(ctx, types.NamespacedName{Name: name}, existing)
		if err == nil {
			return false, nil
		}
		if !apierrors.IsNotFound(err) {
			return false, fmt.Errorf("failed to get environment %s: %w", name, err)
		}
	}

	env := &v1alpha1.Environment{
		TypeMeta:   metav1.TypeMeta{APIVersion: v1alpha1.GroupVersion.String(), Kind: v1alpha1.EnvironmentKind},
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec:       spec,
	}
	obj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(env)
	if err != nil {
		return false, err
	}
	u := &unstructured.Unstructured{Object: obj}
	delete(u.Object, "status")
	unstructured.RemoveNestedField(u.Object, "metadata", "creationTimestamp")
	if err := c.Patch(ctx, u, client.Apply, client.FieldOwner(FieldOwner), client.ForceOwnership); err != nil {
		return false, fmt.Errorf("failed to apply environment %s: %w", name, err)
	}
	return true, nil
}

// WaitReady waits until the operator reports the Environment as ready,
// logging component progress. It fails early when the operator cannot start.
func WaitReady(ctx context.Context, c client.Client, name string, timeout time.Duration, logger *zap.SugaredLogger) error {
	reported := map[string]string{}
	start := time.Now()
	var last *metav1.Condition

	err := wait.PollUntilContextTimeout(ctx, pollInterval, timeout, true, func(ctx context.Context) (bool, error) {
		env := &v1alpha1.Environment{}
		if err := c.Get(ctx, types.NamespacedName{Name: name}, env); err != nil {
			return false, fmt.Errorf("failed to get environment %s: %w", name, err)
		}

		cond := meta.FindStatusCondition(env.Status.Conditions, v1alpha1.ConditionReady)
		if cond == nil || env.Status.ObservedGeneration < env.Generation {
			if time.Since(start) > operatorStartTimeout {
				if err := operatorPodError(ctx, c); err != nil {
					return false, err
				}
			}
			return false, nil
		}
		last = cond

		reportComponents(env.Status.Components, reported, logger)
		return cond.Status == metav1.ConditionTrue, nil
	})
	if err == nil {
		return nil
	}
	if wait.Interrupted(err) {
		if last == nil {
			return fmt.Errorf("environment %s has no status after %s; the operator may not be running, %s", name, timeout, imagePullHint)
		}
		return fmt.Errorf("environment %s is not ready after %s: %s: %s", name, timeout, last.Reason, last.Message)
	}
	return err
}

// reportComponents logs components whose state changed since the last call.
func reportComponents(components []v1alpha1.ComponentStatus, reported map[string]string, logger *zap.SugaredLogger) {
	sorted := append([]v1alpha1.ComponentStatus(nil), components...)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].Name < sorted[j].Name })
	for _, st := range sorted {
		line := describe(st)
		if reported[st.Name] == line {
			continue
		}
		reported[st.Name] = line
		switch st.State {
		case v1alpha1.ComponentFailed, v1alpha1.ComponentMissing:
			logger.Warnf("%s: %s", st.Name, line)
		default:
			logger.Infof("%s: %s", st.Name, line)
		}
	}
}

func describe(st v1alpha1.ComponentStatus) string {
	parts := []string{string(st.State)}
	if st.State != v1alpha1.ComponentDisabled {
		if st.Ready {
			parts = append(parts, "ready")
		} else {
			parts = append(parts, "not ready")
		}
	}
	if st.Version != "" {
		parts = append(parts, st.Version)
	}
	line := strings.Join(parts, ", ")
	if st.Message != "" {
		line += " (" + st.Message + ")"
	}
	return line
}

// operatorPodError returns an error when the operator pod cannot start, e.g.
// because its image cannot be pulled.
func operatorPodError(ctx context.Context, c client.Client) error {
	pods := &corev1.PodList{}
	if c.List(ctx, pods, client.InNamespace(Namespace), client.MatchingLabels{"app.kubernetes.io/name": DeploymentName}) != nil {
		// Pod problems are only a hint; keep waiting for the environment.
		return nil
	}
	for _, pod := range pods.Items {
		for _, cs := range pod.Status.ContainerStatuses {
			if w := cs.State.Waiting; w != nil {
				switch w.Reason {
				case "ErrImagePull", "ImagePullBackOff", "InvalidImageName", "CrashLoopBackOff":
					return fmt.Errorf("operator pod %s cannot start: %s: %s", pod.Name, w.Reason, w.Message)
				}
			}
		}
	}
	return nil
}

// UpdateEnvironment applies mutate to the specification of the Environment of
// the cluster and saves it, retrying on conflicts. The cluster must have
// exactly one Environment.
func UpdateEnvironment(ctx context.Context, c client.Client, mutate func(*v1alpha1.EnvironmentSpec)) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		list := &v1alpha1.EnvironmentList{}
		if err := c.List(ctx, list); err != nil {
			if meta.IsNoMatchError(err) {
				return errNoEnvironment
			}
			return fmt.Errorf("failed to list environments: %w", err)
		}
		switch len(list.Items) {
		case 0:
			return errNoEnvironment
		case 1:
		default:
			return fmt.Errorf("the cluster has %d Environments, expected one", len(list.Items))
		}
		env := &list.Items[0]
		mutate(&env.Spec)
		return c.Update(ctx, env)
	})
}

var errNoEnvironment = fmt.Errorf("no Overlock Environment found in the cluster; create it with \"overlock env create\" or the Overlock K3s image")
