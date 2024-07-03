package policy

import (
	"context"
	"net/url"

	"github.com/kndpio/kndp/internal/install/helm"
	"k8s.io/client-go/rest"
)

const (
	nginxChartName    = "kyverno"
	nginxChartVersion = "3.2.5"
	nginxReleaseName  = "kyverno"
	nginxRepoUrl      = "https://kyverno.github.io/kyverno/"
	nginxNamespace    = "kyverno"
)

var (
	chartValues = map[string]interface{}{
		"cleanupController": map[string]interface{}{
			"enabled": "false",
		},
		"features": map[string]interface{}{
			"admissionReports": map[string]interface{}{
				"enabled": "false",
			},

			"aggregateReports": map[string]interface{}{
				"enabled": "false",
			},
			"policyReports": map[string]interface{}{
				"enabled": "false",
			},
		},
	}
)

func AddKyvernoPolicyConroller(ctx context.Context, config *rest.Config) error {
	repoURL, err := url.Parse(nginxRepoUrl)
	if err != nil {
		return err
	}

	manager, err := helm.NewManager(config, nginxChartName, repoURL, nginxReleaseName,
		helm.InstallerModifierFn(helm.WithNamespace(nginxNamespace)),
		helm.InstallerModifierFn(helm.WithUpgradeInstall(true)),
		helm.InstallerModifierFn(helm.WithCreateNamespace(true)),
	)
	if err != nil {
		return err
	}

	manager.Upgrade(nginxChartVersion, chartValues)
	if err != nil {
		return err
	}

	return nil
}
