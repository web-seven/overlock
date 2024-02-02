package configuration

import (
	"fmt"
	"net/url"

	"k8s.io/client-go/rest"

	"github.com/kndpio/kndp/internal/install/helm"
	"github.com/pterm/pterm"
)

func ApplyConfiguration(Link string, config *rest.Config) {
	chartName := "crossplane"

	repoURL, err := url.Parse("https://charts.crossplane.io/stable")
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	}

	installer, err := helm.NewManager(config, chartName, repoURL)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	}

	parameters := map[string]interface{}{
		"configuration": map[string]interface{}{
			"packages": []string{Link},
		},
	}

	err = installer.Upgrade("", parameters)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	}

	pterm.Success.Println("KNDP configuration applied successfully.")

}
