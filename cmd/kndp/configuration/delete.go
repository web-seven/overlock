package configuration

import (
	"context"

	"github.com/kndpio/kndp/internal/configuration"
	"go.uber.org/zap"
	"k8s.io/client-go/dynamic"
)

type deleteCmd struct {
	ConfigurationURL string `arg:"" required:"" help:"Specifies the URL (or multimple comma separated) of configuration to be deleted from Environment."`
}

func (c *deleteCmd) Run(ctx context.Context, dynamic *dynamic.DynamicClient, logger *zap.SugaredLogger) error {
	return configuration.DeleteConfiguration(ctx, c.ConfigurationURL, dynamic, logger)
}
