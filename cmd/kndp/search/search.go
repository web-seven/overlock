package search

import (
	"context"
	"encoding/json"

	"net/url"
	"strings"

	"github.com/charmbracelet/log"
	"github.com/google/go-github/v61/github"
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

func getTableRegs(ctx context.Context, clientgh *github.Client, org string, query string, pkgType string, allVersions bool, logger *log.Logger) (pterm.TableData, error) {
	tableRegs := pterm.TableData{
		{"URL", "VERSION"},
	}

	pkgs, _, err := clientgh.Organizations.ListPackages(ctx, org, &github.PackageListOptions{PackageType: &pkgType})
	if err != nil {
		logger.Error("Cannot get packages")
		return nil, err
	}

	for _, pkg := range pkgs {
		// Filter packages based on the query
		if !strings.Contains(*pkg.Name, query) {
			continue
		}

		pkgVersions, _, err := clientgh.Organizations.PackageGetAllVersions(ctx, org, pkgType, pkg.GetName(), nil)
		if err != nil {
			logger.Error("Cannot get package versions")
			return nil, err
		}

		var latestVersion *github.PackageVersion
		for _, v := range pkgVersions {
			tags := v.GetMetadata().Container.Tags
			if len(tags) > 0 {
				if !allVersions {
					latestVersion = v
					break
				}
				tableRegs = append(tableRegs, []string{
					"ghcr.io/" + org + "/" + *pkg.Name,
					tags[0],
				})
			}
		}

		if latestVersion != nil {
			tags := latestVersion.GetMetadata().Container.Tags
			if len(tags) > 0 {
				tableRegs = append(tableRegs, []string{
					"ghcr.io/" + org + "/" + *pkg.Name,
					tags[0],
				})
			}
		}
	}

	return tableRegs, nil
}

func (c *SearchCmd) Run(ctx context.Context, client *kubernetes.Clientset, config *rest.Config, logger *log.Logger) error {
	var pkgType = "container"
	registries, err := registry.Registries(ctx, client)
	if err != nil {
		logger.Error("Cannot get registries")
		return err
	}

	for _, r := range registries {
		registryUrl := r.Annotations["kndp-registry-server-url"]
		u, _ := url.Parse(registryUrl)
		org := strings.TrimPrefix(u.Path, "/")

		if strings.Contains(registryUrl, "ghcr.io") {
			auth := registry.RegistryConfig{}
			json.Unmarshal([]byte(r.Data[".dockerconfigjson"]), &auth)
			clientgh := github.NewClient(nil).WithAuthToken(auth.Auths[registryUrl].Password)

			tableRegs, err := getTableRegs(ctx, clientgh, org, c.Query, pkgType, c.Versions, logger)
			if err != nil {
				logger.Error("Cannot get table registries")
			}
			if len(tableRegs) <= 1 {
				logger.Info("No packages found")
				return nil
			} else {
				pterm.DefaultTable.WithHasHeader().WithData(tableRegs).Render()
			}

		} else {
			logger.Errorf("Registry not supported %s", registryUrl)
		}
	}

	return nil
}
