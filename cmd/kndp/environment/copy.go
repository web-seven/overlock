package environment

import (
	"context"

	"github.com/kndpio/kndp/internal/environment"

	"github.com/charmbracelet/log"
)

type copyCmd struct {
	Source      string `arg:"" required:"" help:"Name source of environment."`
	Destination string `arg:"" required:"" help:"Name destination of environment."`
}

func (c *copyCmd) Run(ctx context.Context, logger *log.Logger) error {
	return environment.CopyEnvironment(ctx, logger, c.Source, c.Destination)
}
