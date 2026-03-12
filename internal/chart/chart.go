package chart

import (
	"context"
	"fmt"
	"net/url"

	"go.uber.org/zap"
	"k8s.io/client-go/rest"

	"github.com/web-seven/overlock/internal/install"
	"github.com/web-seven/overlock/internal/install/helm"
)

// Chart abstracts a Helm chart that participates in environment setup
// and node scope management (nodeSelector / tolerations).
type Chart interface {
	Install(ctx context.Context, restConfig *rest.Config, scopeParams map[string]any, logger *zap.SugaredLogger) error
	ScopeParams(nodeSelector map[string]interface{}, tolerations []interface{}) map[string]any
	Apply(restConfig *rest.Config, nodeSelector map[string]interface{}, tolerations []interface{}, logger *zap.SugaredLogger) error
	Remove(restConfig *rest.Config, logger *zap.SugaredLogger) error
}

// EngineScopeSelector returns the standard engine scope nodeSelector and tolerations.
func EngineScopeSelector() (map[string]interface{}, []interface{}) {
	nodeSelector := map[string]interface{}{
		"overlock.io/scope": "engine",
	}
	tolerations := []interface{}{
		map[string]interface{}{
			"key":      "overlock.io/scope",
			"operator": "Equal",
			"value":    "engine",
			"effect":   "NoSchedule",
		},
	}
	return nodeSelector, tolerations
}

// EngineCharts returns the set of charts managed for engine-scope scheduling.
func EngineCharts() []Chart {
	return []Chart{CrossplaneChart{}, KyvernoChart{}, CertManagerChart{}}
}

// chartDef holds metadata required to create a Helm manager for a chart.
type chartDef struct {
	name      string
	repoURL   string
	relName   string
	namespace string
}

// helmManager creates a Helm manager for upgrading an existing release.
func (c chartDef) helmManager(restConfig *rest.Config, reuseValues bool) (install.Manager, error) {
	repoURL, err := url.Parse(c.repoURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse repo URL %q: %w", c.repoURL, err)
	}
	return helm.NewManager(
		restConfig,
		c.name,
		repoURL,
		c.relName,
		helm.InstallerModifierFn(helm.Wait()),
		helm.InstallerModifierFn(helm.WithNamespace(c.namespace)),
		helm.InstallerModifierFn(helm.WithReuseValues(reuseValues)),
		helm.InstallerModifierFn(helm.WithUpgradeInstall(false)),
		helm.InstallerModifierFn(helm.WithCreateNamespace(false)),
	)
}

// applyValues upgrades the chart with nodeSelector and tolerations merged
// on top of existing values.
func (c chartDef) applyValues(restConfig *rest.Config, params map[string]any, logger *zap.SugaredLogger) error {
	mgr, err := c.helmManager(restConfig, true)
	if err != nil {
		return fmt.Errorf("failed to create Helm manager for %q: %w", c.relName, err)
	}
	version, err := mgr.GetCurrentVersion()
	if err != nil {
		return fmt.Errorf("failed to get current version for %q: %w", c.relName, err)
	}
	if err := mgr.Upgrade(version, params); err != nil {
		return fmt.Errorf("failed to upgrade %q with node scope values: %w", c.relName, err)
	}
	logger.Infof("Updated %q Helm release with node scope values.", c.relName)
	return nil
}

// removeValues reads the current release config and strips the specified
// keys, then upgrades without reuseValues so stale values are not merged back.
func (c chartDef) removeValues(restConfig *rest.Config, keys []string, logger *zap.SugaredLogger) error {
	readMgr, err := c.helmManager(restConfig, true)
	if err != nil {
		return fmt.Errorf("failed to create Helm manager for %q: %w", c.relName, err)
	}
	rel, err := readMgr.GetRelease()
	if err != nil {
		logger.Warnf("Could not find release %q, skipping: %v", c.relName, err)
		return nil
	}

	cleanCfg := make(map[string]any, len(rel.Config))
	for k, v := range rel.Config {
		cleanCfg[k] = v
	}
	for _, key := range keys {
		delete(cleanCfg, key)
	}

	version := rel.Chart.Metadata.Version

	upgMgr, err := c.helmManager(restConfig, false)
	if err != nil {
		return fmt.Errorf("failed to create Helm manager for %q: %w", c.relName, err)
	}
	if err := upgMgr.Upgrade(version, cleanCfg); err != nil {
		return fmt.Errorf("failed to upgrade %q: %w", c.relName, err)
	}
	logger.Infof("Removed node scope from %q Helm release.", c.relName)
	return nil
}
