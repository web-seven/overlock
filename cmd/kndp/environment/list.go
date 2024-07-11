package environment

import (
	"context"

	"github.com/kndpio/kndp/internal/environment"

	"github.com/pterm/pterm"
	"go.uber.org/zap"
)

type listCmd struct {
}

func (c *listCmd) Run(ctx context.Context, logger *zap.SugaredLogger) error {
	logger.Info("helo from run list")
	tableData := pterm.TableData{{"NAME", "TYPE"}}
	tableData = environment.ListEnvironments(logger, tableData)
	pterm.DefaultTable.WithHasHeader().WithData(tableData).Render()

	return nil
}
