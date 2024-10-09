package resource

import (
	"context"

	"github.com/web-seven/overlock/internal/resources"
	"go.uber.org/zap"

	"k8s.io/client-go/dynamic"
)

type applyCmd struct {
	File string `required:"" type:"path" short:"f" help:"YAML file containing the Overlock resources to apply."`
}

func (c *applyCmd) Run(ctx context.Context, client *dynamic.DynamicClient, logger *zap.SugaredLogger) error {
	err := resources.ApplyResources(ctx, client, logger, c.File)
	if err != nil {
		logger.Fatal(err)
	} else {
		logger.Info("Kndp resources applied successfully!")
	}
	return nil
}
