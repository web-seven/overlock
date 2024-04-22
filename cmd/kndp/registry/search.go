package registry

import (
	"context"

	"github.com/charmbracelet/log"
	"github.com/kndpio/kndp/internal/search"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// SearchCmd is the struct representing the search command
type SearchCmd struct {
	// Query is the search query
	Query    string `arg:"" help:"search query"`
	Versions bool   `optional:""  short:"v" help:"display all versions"`
}

func (c *SearchCmd) Run(ctx context.Context, client *kubernetes.Clientset, config *rest.Config, logger *log.Logger) error {
	err := search.SearchCmd(ctx, client, config, c.Query, c.Versions, logger)
	if err != nil {
		return err
	}
	return nil
}
