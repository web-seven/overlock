package search

import (
	"context"

	"net/url"
	"strings"

	"github.com/charmbracelet/log"
	"github.com/kndpio/kndp/internal/registry"
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
	registries, err := registry.Registries(ctx, client)
	if err != nil {
		logger.Error("Cannot get registries")
		return err
	}
	for _, r := range registries {
		registryUrl := r.Annotations["kndp-registry-server-url"]
		u, _ := url.Parse(registryUrl)
		org := strings.TrimPrefix(u.Path, "/")
		// Switch statement to handle different registry types
		switch {
		case strings.Contains(registryUrl, "ghcr.io"):
			err := search.GitHubreg(ctx, c.Query, c.Versions, r, registryUrl, org, logger)
			if err != nil {
				return err
			}
		default:
			search.GitHubreg(ctx, c.Query, c.Versions, r, registryUrl, org, logger)
		}

	}

	return nil
}
