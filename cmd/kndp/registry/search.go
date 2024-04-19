package registry

import (
	"context"

	"net/url"
	"strings"

	"github.com/charmbracelet/log"
	"github.com/kndpio/kndp/internal/registry"
	"github.com/pterm/pterm"
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
	tableRegs := pterm.TableData{
		{"URL", "VERSION"},
	}
	for _, r := range registries {
		registryUrl := r.Annotations["kndp-registry-server-url"]
		u, _ := url.Parse(registryUrl)
		org := strings.TrimPrefix(u.Path, "/")
		// Switch statement to handle different registry types
		switch {
		case strings.Contains(registryUrl, "ghcr.io"):
			pkgs, pkgVersions, err := registry.GetGithubPackages(ctx, c.Query, c.Versions, r, registryUrl, org, logger)
			if err != nil {
				return err
			}
			for _, pkg := range pkgs {
				if !strings.Contains(*pkg.Name, c.Query) {
					continue
				}
				versions := pkgVersions[*pkg.Name]
				for _, v := range versions {
					tags := v.GetMetadata().Container.Tags
					if len(tags) > 0 {
						tableRegs = append(tableRegs, []string{
							"ghcr.io/" + org + "/" + *pkg.Name,
							tags[0],
						})
					}
				}
			}
			if len(tableRegs) <= 1 {
				logger.Info("No packages found")
			} else {
				pterm.DefaultTable.WithHasHeader().WithData(tableRegs).Render()
			}

		default:
			_, _, err := registry.GetGithubPackages(ctx, c.Query, c.Versions, r, registryUrl, org, logger)
			if err != nil {
				return err
			}
		}

	}

	return nil
}
