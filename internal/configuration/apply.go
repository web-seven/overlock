package configuration

import (
	"strings"

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
		filteredConfigs := []string{}
		linkName, _, _ := strings.Cut(Link, ":")

		for _, packageLink := range configs["packages"].([]interface{}) {
			packageName, _, _ := strings.Cut(packageLink.(string), ":")
			if packageName != linkName {
				filteredConfigs = append(filteredConfigs, packageLink.(string))
			}
		}

		configs["packages"] = append(
			filteredConfigs,
			Link,
		)
		release.Config["configuration"] = configs
	}

	err = installer.Upgrade(engine.Version, release.Config)
	if err != nil {
		logger.Errorf(" %v\n", err)
	} else {
		logger.Infof("Configuration %s applied successfully.", Link)
	}
}
