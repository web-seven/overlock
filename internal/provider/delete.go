package provider

import (
	"context"
	"fmt"
	"strings"

	"go.uber.org/zap"
	"k8s.io/client-go/rest"

	"github.com/web-seven/overlock/internal/engine"
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

	provider, ok := params["provider"].(map[string]any)
	if !ok {
		return fmt.Errorf("invalid provider configuration in params")
	}
	packages, ok := provider["packages"].([]any)
	if !ok {
		return fmt.Errorf("error extracting packages")
	}
	var newpackages []string
	for _, p := range packages {
		pstr, ok := p.(string)
		if !ok {
			continue // Skip non-string entries
		}
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
