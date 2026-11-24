package bootstrap

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// FieldOwner is the server-side apply field manager of the CLI.
	FieldOwner = "overlock"

	crdName       = "environments.overlock.io"
	crdTimeout    = time.Minute
	pollInterval  = 2 * time.Second
	imagePullHint = "check the operator image with: kubectl -n " + Namespace + " describe pods"
)

// InstallOperator installs or upgrades the operator with server-side apply and
// waits until the Environment CRD is established.
func InstallOperator(ctx context.Context, c client.Client, opts Options, logger *zap.SugaredLogger) error {
	objs, err := Manifests(opts)
	if err != nil {
		return err
	}
	logger.Debugf("Installing Overlock operator %s", opts.Image)
	for _, obj := range objs {
		if err := c.Patch(ctx, obj, client.Apply, client.FieldOwner(FieldOwner), client.ForceOwnership); err != nil {
			return fmt.Errorf("failed to apply %s %s: %w", obj.GetKind(), obj.GetName(), err)
		}
	}
	return waitCRDEstablished(ctx, c)
}

func waitCRDEstablished(ctx context.Context, c client.Client) error {
	err := wait.PollUntilContextTimeout(ctx, pollInterval, crdTimeout, true, func(ctx context.Context) (bool, error) {
		crd := &apiextensionsv1.CustomResourceDefinition{}
		if err := c.Get(ctx, types.NamespacedName{Name: crdName}, crd); err != nil {
			return false, client.IgnoreNotFound(err)
		}
		for _, cond := range crd.Status.Conditions {
			if cond.Type == apiextensionsv1.Established && cond.Status == apiextensionsv1.ConditionTrue {
				return true, nil
			}
		}
		return false, nil
	})
	if err != nil {
		return fmt.Errorf("CRD %s is not established: %w", crdName, err)
	}
	return nil
}
