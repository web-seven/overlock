package search

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/charmbracelet/log"
	"github.com/kndpio/kndp/internal/registry"

	"github.com/google/go-github/v61/github"
	"github.com/pterm/pterm"
)

// GitHubreg search for packages in GitHub Container Registry
func GitHubreg(ctx context.Context, query string, versions bool, r *registry.Registry, registryUrl string, org string, logger *log.Logger) error {
	auth := registry.RegistryConfig{}
	json.Unmarshal([]byte(r.Data[".dockerconfigjson"]), &auth)
	clientgh := github.NewClient(nil).WithAuthToken(auth.Auths[registryUrl].Password)

	pkgType := "container"

	tableRegs := pterm.TableData{
		{"URL", "VERSION"},
	}

	pkgs, _, err := clientgh.Organizations.ListPackages(ctx, org, &github.PackageListOptions{PackageType: &pkgType})
	if err != nil {
		logger.Errorf("Cannot get packages from %s", registryUrl)
		return err
	}

	for _, pkg := range pkgs {
		// Filter packages based on the query
		if !strings.Contains(*pkg.Name, query) {
			continue
		}

		pkgVersions, _, err := clientgh.Organizations.PackageGetAllVersions(ctx, org, pkgType, pkg.GetName(), nil)
		if err != nil {
			logger.Errorf("Cannot get package versions %s", registryUrl)
			return err
		}

		var latestVersion *github.PackageVersion
		for _, v := range pkgVersions {
			tags := v.GetMetadata().Container.Tags
			if len(tags) > 0 {
				if !versions {
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

	if len(tableRegs) <= 1 {
		logger.Info("No packages found")
		return nil
	} else {
		pterm.DefaultTable.WithHasHeader().WithData(tableRegs).Render()
	}
	return nil

}
