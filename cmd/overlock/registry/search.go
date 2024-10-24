package registry

import (
	"context"

	"github.com/pterm/pterm"
	"github.com/web-seven/overlock/internal/search"
	"go.uber.org/zap"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// SearchCmd is the struct representing the search command
type SearchCmd struct {
	// Query is the search query
	Query    string `arg:"" help:"search query"`
	Versions bool   `optional:""  short:"v" help:"display all versions"`
}

func (c *SearchCmd) Run(ctx context.Context, client *kubernetes.Clientset, config *rest.Config, logger *zap.SugaredLogger) error {
	tableRegs, err := search.SearchPackages(ctx, client, config, c.Query, c.Versions, logger)
	if err != nil {
		return err
	}
	if len(tableRegs) <= 1 {
		logger.Info("No packages found")
	} else {
		pterm.DefaultTable.WithHasHeader().WithData(tableRegs).Render()
	}
	return nil
}
