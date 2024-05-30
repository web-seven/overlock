package environment

import (
	"context"

	"github.com/charmbracelet/log"
	"github.com/kndpio/kndp/internal/environment"
)

type stopCmd struct {
	Name string `arg:"" required:"" help:"Name of environment."`
}

func (c *stopCmd) Run(ctx context.Context, logger *log.Logger) error {
	return environment.Stop(ctx, c.Name, logger)
}
