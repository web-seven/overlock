package environment

import (
	"context"
)

type moveCmd struct {
	Source      string `arg:"" required:"" help:"Name source of environment."`
	Destination string `arg:"" required:"" help:"Name destination of environment."`
}

func (c *moveCmd) Run(ctx context.Context) error {
	return nil
}
