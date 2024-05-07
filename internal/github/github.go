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

// GetPackages list packages and their versions from Container Registry
func GetPackages(ctx context.Context, query string, version bool, r *registry.Registry, registryUrl string, org string, logger *log.Logger) (pterm.TableData, error) {
	auth := registry.RegistryConfig{}
	json.Unmarshal([]byte(r.Data[".dockerconfigjson"]), &auth)
	clientgh := github.NewClient(nil).WithAuthToken(auth.Auths[registryUrl].Password)
	tableRegs := pterm.TableData{
		{"URL", "VERSION"},
	}
	pkgType := "container"
	var allPkgs []*github.Package
	opts := &github.PackageListOptions{
		PackageType: &pkgType,
	}

	for {
		pkgs, resp, err := clientgh.Organizations.ListPackages(ctx, org, opts)
		if err != nil {
			logger.Errorf("Cannot get packages from %s", registryUrl)
			return nil, err
		}
		allPkgs = append(allPkgs, pkgs...)

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	pkgVersions := make(map[string][]*github.PackageVersion)
	for _, pkg := range allPkgs {
		versions, _, err := clientgh.Organizations.PackageGetAllVersions(ctx, org, pkgType, pkg.GetName(), nil)
		if err != nil {
			logger.Errorf("Cannot get package versions for %s/%s", org, *pkg.Name)
			return nil, err
		}
		if !version {
			if len(versions) > 0 {
				pkgVersions[*pkg.Name] = []*github.PackageVersion{versions[0]}
			}
		} else {
			pkgVersions[*pkg.Name] = versions
		}
	}

	for _, pkg := range allPkgs {
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

	return tableRegs, nil
}
