package policy

import (
	"context"
	"net/url"

	"github.com/kndpio/kndp/internal/install/helm"
	"k8s.io/client-go/rest"
)

const (
	kyvernoChartName    = "kyverno"
	kyvernoChartVersion = "3.2.5"
	kyvernoReleaseName  = "kyverno"
	kyvernoRepoUrl      = "https://kyverno.github.io/kyverno/"
	kyvernoNamespace    = "kyverno"
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
	repoURL, err := url.Parse(kyvernoRepoUrl)
	if err != nil {
		return err
	}

	manager, err := helm.NewManager(config, kyvernoChartName, repoURL, kyvernoReleaseName,
		helm.InstallerModifierFn(helm.WithNamespace(kyvernoNamespace)),
		helm.InstallerModifierFn(helm.WithUpgradeInstall(true)),
		helm.InstallerModifierFn(helm.WithCreateNamespace(true)),
	)
	if err != nil {
		return err
	}

	manager.Upgrade(kyvernoChartVersion, chartValues)
	if err != nil {
		return err
	}

	return nil
}
