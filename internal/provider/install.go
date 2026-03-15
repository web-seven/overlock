package provider

import (
	"fmt"

	"go.uber.org/zap"

	"github.com/web-seven/overlock/internal/engine"

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
		configs, ok := release.Config["provider"].(map[string]interface{})
		if !ok {
			return fmt.Errorf("invalid provider configuration")
		}
		packages, ok := configs["packages"].([]interface{})
		if !ok {
			return fmt.Errorf("invalid packages configuration")
		}
		configs["packages"] = append(packages, provider)
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
