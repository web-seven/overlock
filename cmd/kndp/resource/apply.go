package resource

import (
	"context"

	"github.com/kndpio/kndp/internal/resources"
	"go.uber.org/zap"

	"k8s.io/client-go/dynamic"
)

type applyCmd struct {
	File string `required:"" type:"path" short:"f" help:"YAML file containing the KNDP resources to apply."`
}

func (c *applyCmd) Run(ctx context.Context, client *dynamic.DynamicClient, logger *zap.Logger) error {
	err := resources.ApplyResources(ctx, client, logger, c.File)
	if err != nil {
		logger.Sugar().Fatal(err)
	} else {
		logger.Sugar().Info("Kndp resources applied successfully!")
	}
	return nil
}
