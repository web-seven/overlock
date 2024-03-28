package configuration

import (
	"github.com/charmbracelet/log"

	"k8s.io/client-go/rest"

	"github.com/kndpio/kndp/internal/engine"
)

func ApplyConfiguration(Link string, config *rest.Config, logger *log.Logger) {

	installer, err := engine.GetEngine(config)
	if err != nil {
		logger.Errorf(" %v\n", err)
	}

	release, _ := installer.GetRelease()

	if release.Config == nil {
		release.Config = map[string]interface{}{
			"configuration": map[string]interface{}{
				"packages": []string{Link},
			},
		}
	} else if release.Config["configuration"] == nil {
		release.Config["configuration"] = map[string]interface{}{
			"packages": []string{Link},
		}
	} else {
		configs := release.Config["configuration"].(map[string]interface{})
		configs["packages"] = append(
			configs["packages"].([]interface{}),
			Link,
		)
		release.Config["configuration"] = configs
	}

	err = installer.Upgrade("", release.Config)
	if err != nil {
		logger.Errorf(" %v\n", err)
	}

	logger.Info("Configuration applied successfully.")

}
