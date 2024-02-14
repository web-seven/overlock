package configuration

import (
	"fmt"
	"net/url"

	"k8s.io/client-go/rest"

	"github.com/kndpio/kndp/internal/install"
	"github.com/kndpio/kndp/internal/install/helm"
	"github.com/pterm/pterm"
)

func GetCrossplaneChart(config *rest.Config) install.Manager {
	chartName := "crossplane"

	repoURL, err := url.Parse("https://charts.crossplane.io/stable")
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	}
	installer, err := helm.NewManager(config, chartName, repoURL)
	installer.GetCurrentVersion()
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	}
	return installer
}

func ApplyConfiguration(Link string, config *rest.Config) {

	parameters := map[string]interface{}{
		"configuration": map[string]interface{}{
			"packages": []string{Link},
		},
	}

	installer := GetCrossplaneChart(config)

	err := installer.Upgrade("", parameters)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	}

	pterm.Success.Println("KNDP configuration applied successfully.")

}
