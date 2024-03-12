package configuration

import (
	"net/url"

	"github.com/charmbracelet/log"

	"k8s.io/client-go/rest"

	"github.com/kndpio/kndp/internal/install"
	"github.com/kndpio/kndp/internal/install/helm"
)

const ReleaseName = "kndp-crossplane"

func GetManager(config *rest.Config, logger *log.Logger) install.Manager {
	chartName := "crossplane"

	repoURL, err := url.Parse("https://charts.crossplane.io/stable")
	if err != nil {
		logger.Errorf(" %v\n", err)
	}
	installer, err := helm.NewManager(config, chartName, repoURL, ReleaseName, helm.WithReuseValues(true))
	installer.GetCurrentVersion()
	if err != nil {
		logger.Errorf(" %v\n", err)
	}
	return installer
}

func ApplyConfiguration(Link string, config *rest.Config, logger *log.Logger) {

	parameters := map[string]interface{}{
		"configuration": map[string]interface{}{
			"packages": []string{Link},
		},
	}

	installer := GetManager(config, logger)

	err := installer.Upgrade("", parameters)
	if err != nil {
		logger.Errorf(" %v\n", err)
	}

	logger.Info("KNDP configuration applied successfully.")

}
