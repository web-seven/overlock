package configuration

import (
	"context"
	"fmt"

	"github.com/pterm/pterm"
	"go.uber.org/zap"
	"k8s.io/client-go/dynamic"

	"github.com/web-seven/overlock/pkg/configuration"
)

type listCmd struct {
}

func (listCmd) Run(ctx context.Context, dynamicClient *dynamic.DynamicClient, logger *zap.SugaredLogger) error {
	configurations := configuration.GetConfigurations(ctx, dynamicClient)
	table := pterm.TableData{[]string{"NAME", "PACKAGE"}}
	for _, conf := range configurations {
		table = append(table, []string{conf.Name, conf.Spec.Package})
	}
	if err := pterm.DefaultTable.WithHasHeader().WithData(table).Render(); err != nil {
		return fmt.Errorf("failed to render table: %w", err)
	}
	return nil
}
