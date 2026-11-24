package configuration

import (
	"context"
	"fmt"

	"go.uber.org/zap"
	"k8s.io/client-go/dynamic"

	"github.com/web-seven/overlock/pkg/configuration"
)

type deleteCmd struct {
	ConfigurationURL string `arg:"" required:"" help:"Specifies the URL (or multimple comma separated) of configuration to be deleted from Environment."`
}

func (c *deleteCmd) Run(ctx context.Context, dynamic *dynamic.DynamicClient, logger *zap.SugaredLogger) error {
	if err := configuration.DeleteConfiguration(ctx, c.ConfigurationURL, dynamic, logger); err != nil {
		return fmt.Errorf("failed to delete configuration: %w", err)
	}
	return nil
}
