package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/web-seven/overlock/internal/engine"
	"go.uber.org/zap"
	"k8s.io/client-go/rest"
)

// DeleteProvider deletes a crossplane provider from current environment
func DeleteProvider(ctx context.Context, configClient *rest.Config, url string, logger *zap.SugaredLogger) error {

	logger.Debug("Preparing engine")
	installer, err := engine.GetEngine(configClient)
	if err != nil {
		return err
	}

	var params map[string]any
	release, err := installer.GetRelease()
	if err == nil {
		params = release.Config
	}

	provider := params["provider"].(map[string]any)
	packages, ok := provider["packages"].([]any)
	if !ok {
		return fmt.Errorf("error extracting packages")
	}
	var newpackages []string
	for _, p := range packages {
		pstr := p.(string)
		if !strings.Contains(pstr, url) {
			newpackages = append(newpackages, pstr)
		}

	}
	provider["packages"] = newpackages
	params["provider"] = provider

	logger.Debug("Installing engine")
	err = engine.InstallEngine(ctx, configClient, params, logger)
	if err != nil {
		return err
	}
	logger.Infof("Provider %s deleted successfully", url)
	return nil
}
