package function

import (
	"context"

	"github.com/pterm/pterm"
	"go.uber.org/zap"

	"github.com/web-seven/overlock/internal/function"

	"k8s.io/client-go/dynamic"
)

type listCmd struct {
}

func (listCmd) Run(ctx context.Context, dynamicClient *dynamic.DynamicClient, logger *zap.SugaredLogger) error {
	functions := function.GetFunctions(ctx, dynamicClient)
	table := pterm.TableData{[]string{"NAME", "PACKAGE"}}
	for _, conf := range functions {
		table = append(table, []string{conf.Name, conf.Spec.Package})
	}
	pterm.DefaultTable.WithHasHeader().WithData(table).Render()
	return nil
}
