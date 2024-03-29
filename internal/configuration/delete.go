package configuration

import (
	"github.com/charmbracelet/log"

	"k8s.io/client-go/rest"

	"github.com/kndpio/kndp/internal/engine"
)

func DeleteConfiguration(url string, config *rest.Config, logger *log.Logger) error {

	installer, err := engine.GetEngine(config)
	if err != nil {
		logger.Errorf(" %v\n", err)
	}

	release, _ := installer.GetRelease()

	if release.Config == nil || release.Config["configuration"] == nil {
		logger.Warn("Not found any applied configuration.")
	} else {
		configs := release.Config["configuration"].(map[string]interface{})
		oldPackages := configs["packages"].([]interface{})

		newPackages := []interface{}{}
		for _, config := range oldPackages {
			if config != url {
				newPackages = append(
					newPackages,
					config,
				)
			}
		}
		release.Config["configuration"] = map[string]interface{}{
			"packages": newPackages,
		}
		if len(oldPackages) == len(newPackages) {
			logger.Warn("Configuration URL not found applied configurations.")
			return nil
		}
	}

	err = installer.Upgrade("", release.Config)
	if err != nil {
		return err
	}

	logger.Info("Configuration removed successfully.")
	return nil
}
