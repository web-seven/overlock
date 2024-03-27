package provider

import (
	"github.com/charmbracelet/log"
	"github.com/kndpio/kndp/internal/engine"

	"k8s.io/client-go/rest"
)

func InstallProvider(provider string, config *rest.Config, logger *log.Logger) {

	parameters := map[string]interface{}{
		"provider": map[string]interface{}{
			"packages": []string{provider},
		},
	}

	installer, err := engine.GetEngine(config)
	if err != nil {
		logger.Errorf(" %v\n", err)
	}

	err = installer.Upgrade("", parameters)
	if err != nil {
		logger.Errorf(" %v\n", err)
	}

	logger.Info("KNDP provider installed successfully.")

}
