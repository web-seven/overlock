package provider

import (
	"github.com/web-seven/overlock/internal/engine"
	"go.uber.org/zap"

	"k8s.io/client-go/rest"
)

func InstallProvider(provider string, config *rest.Config, logger *zap.SugaredLogger) error {

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

	version, err := installer.GetCurrentVersion()
	if err != nil {
		return err
	}

	err = installer.Upgrade(version, release.Config)
	if err != nil {
		return err
	}

	logger.Info("Overlock provider installed successfully.")
	return nil
}
