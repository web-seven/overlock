package environment

import (
	"context"
	"fmt"

	"go.uber.org/zap"
	"k8s.io/client-go/rest"

	"github.com/web-seven/overlock/api/v1alpha1"
	"github.com/web-seven/overlock/internal/chart"
	"github.com/web-seven/overlock/internal/kube"
	"github.com/web-seven/overlock/internal/namespace"
)

// InstallOptions holds cluster-specific settings for Install that are not part
// of the operator-compatible EnvironmentSpec.
type InstallOptions struct {
	// EngineScope schedules engine components on nodes labeled with the
	// engine scope (used by the k3s-docker engine).
	EngineScope bool
}

// Install installs Crossplane, its packages and the enabled tools described by
// spec into the cluster reachable through restConfig. It is shared by the CLI
// (after creating a cluster) and the operator (in its own cluster).
func Install(ctx context.Context, restConfig *rest.Config, spec v1alpha1.EnvironmentSpec, opts InstallOptions, logger *zap.SugaredLogger) error {
	var nodeSelector map[string]interface{}
	if opts.EngineScope {
		nodeSelector, _ = chart.EngineScopeSelector()
	}

	for _, ch := range Charts(spec) {
		var scopeParams map[string]any
		if nodeSelector != nil {
			scopeParams = ch.ScopeParams(nodeSelector, []interface{}{})
		}
		if err := ch.Install(ctx, restConfig, scopeParams, logger); err != nil {
			return fmt.Errorf("failed to install chart: %w", err)
		}
	}

	// Patch DeploymentRuntimeConfig for provider/function scheduling.
	if opts.EngineScope {
		if err := chart.PatchDefaultRuntimeConfig(restConfig, nodeSelector, []interface{}{}, logger); err != nil {
			logger.Warnf("Failed to patch DeploymentRuntimeConfig: %v", err)
		}
	}

	if spec.AdminServiceAccount != nil {
		logger.Debug("Creating admin service account")
		if _, err := kube.CreateAdminServiceAccount(ctx, restConfig, spec.AdminServiceAccount.Name, namespace.Namespace, logger); err != nil {
			logger.Errorf("Failed to create admin service account: %v", err)
			return err
		}
		logger.Debug("Done")
	}

	return nil
}

// Charts returns the Helm charts to install for spec, in installation order.
func Charts(spec v1alpha1.EnvironmentSpec) []chart.Chart {
	args := append([]string{}, spec.Crossplane.Args...)
	if spec.Crossplane.MaxReconcileRate > 0 {
		args = append(args, fmt.Sprintf("--max-reconcile-rate=%d", spec.Crossplane.MaxReconcileRate))
	}

	charts := []chart.Chart{
		chart.CrossplaneChart{
			Configurations: spec.Configurations,
			Providers:      spec.Providers,
			Functions:      spec.Functions,
			Args:           args,
		},
	}
	if spec.Tools.Kyverno.IsEnabled() {
		charts = append(charts, chart.KyvernoChart{})
	}
	if spec.Tools.CertManager.IsEnabled() {
		charts = append(charts, chart.CertManagerChart{})
	}
	return charts
}
