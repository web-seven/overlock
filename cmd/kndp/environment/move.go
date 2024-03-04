package environment

import (
	"context"

	"github.com/kndpio/kndp/internal/environment"

	"github.com/charmbracelet/log"
)

type moveCmd struct {
	Source      string `arg:"" required:"" help:"Name source of environment."`
	Destination string `arg:"" required:"" help:"Name destination of environment."`
}

func (c *moveCmd) Run(ctx context.Context, logger *log.Logger) error {

	environment.MoveKndpResources(ctx, logger, c.Source, c.Destination)
	return nil
}
