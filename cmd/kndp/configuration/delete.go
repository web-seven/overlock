package configuration

import (
	"context"

	"github.com/kndpio/kndp/internal/configuration"
	"k8s.io/client-go/dynamic"

	"github.com/charmbracelet/log"
)

type deleteCmd struct {
	ConfigurationURL string `arg:"" required:"" help:"Specifies the URL of configuration to be deleted from Environment."`
}

func (c *deleteCmd) Run(ctx context.Context, dynamic *dynamic.DynamicClient, logger *log.Logger) error {
	return configuration.DeleteConfiguration(ctx, c.ConfigurationURL, dynamic, logger)
}
