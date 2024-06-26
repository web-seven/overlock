package environment

import (
	"context"

	"github.com/charmbracelet/log"
	"github.com/kndpio/kndp/internal/engine"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

type upgradeCmd struct {
	Context string `arg:"" required:"" help:"Kubernetes context where engine will be upgraded."`
}

func (c *upgradeCmd) Run(ctx context.Context, logger *log.Logger) error {
	configClient, err := config.GetConfigWithContext(c.Context)
	if err != nil {
		logger.Fatal(err)
	}
	err = engine.InstallEngine(ctx, configClient, nil)
	if err != nil {
		logger.Fatal(err)
	}
	logger.Info("Environment upgraded successfully.")
	return nil
}
