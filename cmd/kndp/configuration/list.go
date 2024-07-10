package configuration

import (
	"context"

	"github.com/kndpio/kndp/internal/configuration"
	"github.com/pterm/pterm"
	"go.uber.org/zap"

	"k8s.io/client-go/dynamic"
)

type listCmd struct {
}

func (listCmd) Run(ctx context.Context, dynamicClient *dynamic.DynamicClient, logger *zap.Logger) error {
	configurations := configuration.GetConfigurations(ctx, dynamicClient)
	table := pterm.TableData{{"NAME", "PACKAGE"}}
	for _, conf := range configurations {
		table = append(table, []string{conf.Name, conf.Spec.Package})
	}
	pterm.DefaultTable.WithHasHeader().WithData(table).Render()
	return nil
}
