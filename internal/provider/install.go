package provider

import (
	"github.com/charmbracelet/log"
	"github.com/kndpio/kndp/internal/engine"

	"k8s.io/client-go/rest"
)

func InstallProvider(provider string, config *rest.Config, logger *log.Logger) error {

	installer, err := engine.GetEngine(config)
	if err != nil {
		return err
	}

	release, _ := installer.GetRelease()

	if release.Config == nil {
		release.Config = map[string]interface{}{
			"provider": map[string]interface{}{
				"packages": []string{provider},
			},
		}
	} else if release.Config["provider"] == nil {
		release.Config["provider"] = map[string]interface{}{
			"packages": []string{provider},
		}
	} else {
		configs := release.Config["provider"].(map[string]interface{})
		configs["packages"] = append(
			configs["packages"].([]interface{}),
			provider,
		)
		release.Config["provider"] = configs
	}

	err = installer.Upgrade(engine.Version, release.Config)
	if err != nil {
		return err
	}

	logger.Info("KNDP provider installed successfully.")
	return nil
}
