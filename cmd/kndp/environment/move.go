package environment

import (
	"context"

	"github.com/pterm/pterm"
)

type moveCmd struct {
	Source      string `arg:"" required:"" help:"Name source of environment."`
	Destination string `arg:"" required:"" help:"Name destination of environment."`
}

func (c *moveCmd) Run(ctx context.Context, p pterm.TextPrinter) error {
	return nil
}
