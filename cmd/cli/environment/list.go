package environment

import (
	"context"

	"github.com/pkg/errors"
	"github.com/pterm/pterm"
	"go.uber.org/zap"

	"github.com/web-seven/overlock/pkg/environment"
)

type listCmd struct {
}

func (c *listCmd) Run(ctx context.Context, logger *zap.SugaredLogger) error {
	logger.Info("helo from run list")
	tableData := pterm.TableData{[]string{"NAME", "TYPE"}}
	tableData, err := environment.ListEnvironments(logger, tableData)
	if err != nil {
		return errors.Wrap(err, "failed to list environments")
	}
	if err := pterm.DefaultTable.WithHasHeader().WithData(tableData).Render(); err != nil {
		return errors.Wrap(err, "failed to render table")
	}

	return nil
}
