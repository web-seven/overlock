package configuration

import (
	"context"

	"github.com/kndpio/kndp/internal/configuration"

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"

	"github.com/charmbracelet/log"
)

type applyCmd struct {
	Link string `arg:"" required:"" help:"Link URL to Crossplane configuration to be applied to Environment."`
}

func (c *applyCmd) Run(ctx context.Context, config *rest.Config, dynamicClient *dynamic.DynamicClient, logger *log.Logger) error {
	configuration.ApplyConfiguration(c.Link, config, logger)
	return nil
}
