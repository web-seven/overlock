package provider

import (
	"context"

	"github.com/charmbracelet/log"
	"github.com/pterm/pterm"

	"github.com/kndpio/kndp/internal/provider"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
)

type listCmd struct {
}

func (c *listCmd) Run(ctx context.Context, config *rest.Config, dynamicClient *dynamic.DynamicClient, logger *log.Logger) error {
	providers := provider.ListProviders(ctx, dynamicClient, logger)
	table := pterm.TableData{{"NAME", "PACKAGE"}}
	for _, provider := range providers {
		table = append(table, []string{provider.Name, provider.Spec.Package})
	}
	pterm.DefaultTable.WithHasHeader().WithData(table).Render()
	return nil
}
