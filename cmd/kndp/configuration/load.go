package configuration

import (
	"context"

	"k8s.io/client-go/rest"

	"github.com/charmbracelet/log"
)

type loadCmd struct {
	Link string `arg:"" required:"" help:"Link URL (or multiple comma separated) to Crossplane configuration to be loaded from Docker to Environment."`
}

func (c *loadCmd) Run(ctx context.Context, config *rest.Config, logger *log.Logger) error {
	return nil
}
