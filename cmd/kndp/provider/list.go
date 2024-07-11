package provider

import (
	"context"

	"github.com/pterm/pterm"
	"go.uber.org/zap"

	"github.com/kndpio/kndp/internal/provider"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
)

type listCmd struct {
}

func (c *listCmd) Run(ctx context.Context, config *rest.Config, dynamicClient *dynamic.DynamicClient, logger *zap.SugaredLogger) error {
	providers := provider.ListProviders(ctx, dynamicClient, logger)
	table := pterm.TableData{{"NAME", "PACKAGE"}}
	for _, provider := range providers {
		table = append(table, []string{provider.Name, provider.Spec.Package})
	}
	pterm.DefaultTable.WithHasHeader().WithData(table).Render()
	return nil
}
