package environment

import (
	"context"

	"github.com/pterm/pterm"
)

type createCmd struct {
	Name string `arg:"" required:"" help:"Name of environment."`
}

func (c *createCmd) Run(ctx context.Context, p pterm.TextPrinter) error {
	return nil
}
