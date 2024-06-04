package environment

import (
	"context"

	"github.com/charmbracelet/log"
	"github.com/kndpio/kndp/internal/environment"
)

type startCmd struct {
	Name string `arg:"" required:"" help:"Name of environment."`
}

func (c *startCmd) Run(ctx context.Context, logger *log.Logger) error {
	return environment.Start(ctx, c.Name, logger)
}
