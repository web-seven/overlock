package configuration

import (
	"context"

	"github.com/pterm/pterm"
)

type applyCmd struct {
	Link string `arg:"" required:"" help:"Link URL to Crossplane configuration to be applied to Environment."`
}

func (c *applyCmd) Run(ctx context.Context, p pterm.TextPrinter) error {
	return nil
}
