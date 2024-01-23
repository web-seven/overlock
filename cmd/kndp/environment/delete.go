package environment

import (
	"context"

	"github.com/pterm/pterm"
)

type deleteCmd struct {
	Name string `arg:"" required:"" help:"Name of environment."`
}

func (c *deleteCmd) Run(ctx context.Context, p pterm.TextPrinter) error {
	return nil
}
