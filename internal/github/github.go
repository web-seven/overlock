package github

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/charmbracelet/log"
	"github.com/kndpio/kndp/internal/registry"
	"github.com/pterm/pterm"

	"github.com/google/go-github/v61/github"
)

// GetGithubPackages list packages and their versions from GitHub Container Registry
func GetGithubPackages(ctx context.Context, query string, version bool, r *registry.Registry, registryUrl string, org string, logger *log.Logger) ([]*github.Package, map[string][]*github.PackageVersion, error) {
	auth := registry.RegistryConfig{}
	json.Unmarshal([]byte(r.Data[".dockerconfigjson"]), &auth)
	clientgh := github.NewClient(nil).WithAuthToken(auth.Auths[registryUrl].Password)
	tableRegs := pterm.TableData{
		{"URL", "VERSION"},
	}
	pkgType := "container"

	pkgs, _, err := clientgh.Organizations.ListPackages(ctx, org, &github.PackageListOptions{PackageType: &pkgType})
	if err != nil {
		logger.Errorf("Cannot get packages from %s", registryUrl)
		return nil, nil, err
	}

	pkgVersions := make(map[string][]*github.PackageVersion)
	for _, pkg := range pkgs {
		versions, _, err := clientgh.Organizations.PackageGetAllVersions(ctx, org, pkgType, pkg.GetName(), nil)
		if err != nil {
			logger.Errorf("Cannot get package versions for %s/%s", org, *pkg.Name)
			return nil, nil, err
		}
		if !version {
			if len(versions) > 0 {
				pkgVersions[*pkg.Name] = []*github.PackageVersion{versions[0]}
			}
		} else {
			pkgVersions[*pkg.Name] = versions
		}
	}

	for _, pkg := range pkgs {
		if !strings.Contains(*pkg.Name, query) {
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

	return pkgs, pkgVersions, nil
}
