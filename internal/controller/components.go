package controller

import (
	"fmt"

	"github.com/web-seven/overlock/api/v1alpha1"
	"github.com/web-seven/overlock/internal/certmanager"
	"github.com/web-seven/overlock/internal/chart"
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
	// UninstallWithoutHooks skips the chart's delete hooks; the operator
	// does their work itself (see prepareKyvernoRemoval).
	UninstallWithoutHooks bool
	// MinVersion is the oldest chart version the operator installs.
	MinVersion string
}

// helmComponents returns the Helm components of spec in installation order:
// cert-manager first, then Crossplane, then Kyverno, whose admission webhooks
// could otherwise interfere with the other installations. Removal runs in
// reverse order.
func helmComponents(spec v1alpha1.EnvironmentSpec) []helmComponent {
	comps := []helmComponent{
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
			Name:       componentCrossplane,
			Chart:      engine.ChartName,
			RepoURL:    engine.RepoUrl,
			Release:    engine.ReleaseName,
			Namespace:  namespace.Namespace,
			Version:    orDefault(spec.Crossplane.Version, engine.Version),
			MinVersion: engine.MinVersion,
			Values:     crossplaneValues(spec.Crossplane),
			Enabled:    true,
			Policy:     policyOrDefault(spec.Crossplane.Policy),
			CRD:        "providers.pkg.crossplane.io",
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
			// Releases installed before config.preserve=false run a kubectl
			// post-delete hook whose image no longer exists.
			UninstallWithoutHooks: true,
		},
	}

	if spec.Scheduling != nil {
		nodeSelector, tolerations := schedulingParams(spec.Scheduling)
		scopes := []chart.Chart{chart.CertManagerChart{}, chart.CrossplaneChart{}, chart.KyvernoChart{}}
		for i := range comps {
			comps[i].Values = mergeValues(comps[i].Values, scopes[i].ScopeParams(nodeSelector, tolerations))
		}
	}
	return comps
}

// schedulingParams converts scheduling to the generic form used in chart values.
func schedulingParams(s *v1alpha1.SchedulingSpec) (map[string]interface{}, []interface{}) {
	nodeSelector := map[string]interface{}{}
	for k, v := range s.NodeSelector {
		nodeSelector[k] = v
	}
	tolerations := []interface{}{}
	for _, t := range s.Tolerations {
		tol := map[string]interface{}{}
		if t.Key != "" {
			tol["key"] = t.Key
		}
		if t.Operator != "" {
			tol["operator"] = string(t.Operator)
		}
		if t.Value != "" {
			tol["value"] = t.Value
		}
		if t.Effect != "" {
			tol["effect"] = string(t.Effect)
		}
		if t.TolerationSeconds != nil {
			tol["tolerationSeconds"] = *t.TolerationSeconds
		}
		tolerations = append(tolerations, tol)
	}
	return nodeSelector, tolerations
}

// mergeValues deep-merges src into a copy of dst; values of src win.
func mergeValues(dst, src map[string]any) map[string]any {
	out := make(map[string]any, len(dst)+len(src))
	for k, v := range dst {
		out[k] = v
	}
	for k, v := range src {
		if sm, ok := v.(map[string]any); ok {
			if dm, ok := out[k].(map[string]any); ok {
				out[k] = mergeValues(dm, sm)
				continue
			}
		}
		out[k] = v
	}
	return out
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
	values := map[string]any{"args": args}
	if spec.RegistryCABundle != nil {
		values["registryCaBundleConfig"] = map[string]any{
			"name": spec.RegistryCABundle.Name,
			"key":  spec.RegistryCABundle.Key,
		}
	}
	return values
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

// Chart is a Helm chart the operator installs for an Environment.
type Chart struct {
	Name      string
	Chart     string
	RepoURL   string
	Release   string
	Namespace string
	Version   string
	Values    map[string]any
}

// DefaultCharts returns the Helm charts, with versions and values, the
// operator installs for an Environment with a default specification. It is
// used to bundle their images into the Overlock K3s image.
func DefaultCharts() []Chart {
	var charts []Chart
	for _, c := range helmComponents(v1alpha1.EnvironmentSpec{}) {
		charts = append(charts, Chart{
			Name:      c.Name,
			Chart:     c.Chart,
			RepoURL:   c.RepoURL,
			Release:   c.Release,
			Namespace: c.Namespace,
			Version:   c.Version,
			Values:    c.Values,
		})
	}
	return charts
}
