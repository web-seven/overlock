package configuration

import (
	"github.com/charmbracelet/log"

	"k8s.io/client-go/rest"

	"github.com/kndpio/kndp/internal/engine"
)

func ApplyConfiguration(Link string, config *rest.Config, logger *log.Logger) {

	parameters := map[string]interface{}{
		"configuration": map[string]interface{}{
			"packages": []string{Link},
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

	logger.Info("KNDP configuration applied successfully.")

}
