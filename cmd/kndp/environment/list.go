package environment

import (
	"context"

	"github.com/kndpio/kndp/internal/environment"

	"github.com/charmbracelet/log"
	"github.com/pterm/pterm"
)

type listCmd struct {
}

func (c *listCmd) Run(ctx context.Context, logger *log.Logger) error {

	tableData := pterm.TableData{{"NAME", "TYPE"}}
	tableData = environment.ListEnvironments(logger, tableData)
	pterm.DefaultTable.WithHasHeader().WithData(tableData).Render()

	return nil
}
