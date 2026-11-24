package controller

import (
	"fmt"

	"github.com/web-seven/overlock/api/v1alpha1"
	"github.com/web-seven/overlock/internal/certmanager"
	"github.com/web-seven/overlock/internal/engine"
	"github.com/web-seven/overlock/internal/namespace"
	"github.com/web-seven/overlock/internal/policy"
)

// Component names reported in the Environment status.
const (
	componentCertManager         = "cert-manager"
	componentCrossplane          = "crossplane"
	componentKyverno             = "kyverno"
	componentAdminServiceAccount = "admin-service-account"
)

// helmComponent is a component installed as a Helm release.
type helmComponent struct {
	Name      string
	Chart     string
	RepoURL   string
	Release   string
	Namespace string
	Version   string
	Values    map[string]any
	Enabled   bool
	Policy    v1alpha1.ManagementPolicy
	// CRD identifies an existing installation that was not made by a Helm
	// release with the expected name.
	CRD string
}

// helmComponents returns the Helm components of spec in installation order:
// cert-manager first, then Crossplane, then Kyverno, whose admission webhooks
// could otherwise interfere with the other installations. Removal runs in
// reverse order.
func helmComponents(spec v1alpha1.EnvironmentSpec) []helmComponent {
	return []helmComponent{
		{
			Name:      componentCertManager,
			Chart:     certmanager.ChartName,
			RepoURL:   certmanager.RepoURL,
			Release:   certmanager.ReleaseName,
			Namespace: certmanager.Namespace,
			Version:   orDefault(spec.Tools.CertManager.Version, certmanager.ChartVersion),
			Values:    certmanager.Values(),
			Enabled:   spec.Tools.CertManager.IsEnabled(),
			Policy:    policyOrDefault(spec.Tools.CertManager.Policy),
			CRD:       "certificates.cert-manager.io",
		},
		{
			Name:      componentCrossplane,
			Chart:     engine.ChartName,
			RepoURL:   engine.RepoUrl,
			Release:   engine.ReleaseName,
			Namespace: namespace.Namespace,
			Version:   orDefault(spec.Crossplane.Version, engine.Version),
			Values:    crossplaneValues(spec.Crossplane),
			Enabled:   true,
			Policy:    policyOrDefault(spec.Crossplane.Policy),
			CRD:       "providers.pkg.crossplane.io",
		},
		{
			Name:      componentKyverno,
			Chart:     policy.KyvernoChartName,
			RepoURL:   policy.KyvernoRepoURL,
			Release:   policy.KyvernoReleaseName,
			Namespace: policy.KyvernoNamespace,
			Version:   orDefault(spec.Tools.Kyverno.Version, policy.KyvernoChartVersion),
			Values:    policy.KyvernoValues(),
			Enabled:   spec.Tools.Kyverno.IsEnabled(),
			Policy:    policyOrDefault(spec.Tools.Kyverno.Policy),
			CRD:       "clusterpolicies.kyverno.io",
		},
	}
}

// crossplaneValues returns the Crossplane chart values. Packages are not part
// of the values: they are managed as Crossplane package resources.
func crossplaneValues(spec v1alpha1.CrossplaneSpec) map[string]any {
	args := make([]any, 0, len(spec.Args)+1)
	for _, a := range spec.Args {
		args = append(args, a)
	}
	if spec.MaxReconcileRate > 0 {
		args = append(args, fmt.Sprintf("--max-reconcile-rate=%d", spec.MaxReconcileRate))
	}
	return map[string]any{"args": args}
}

func orDefault(v, def string) string {
	if v == "" {
		return def
	}
	return v
}

func policyOrDefault(p v1alpha1.ManagementPolicy) v1alpha1.ManagementPolicy {
	if p == "" {
		return v1alpha1.ManagementAuto
	}
	return p
}
