package github

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/kndpio/kndp/internal/registry"
	"github.com/pterm/pterm"
	"go.uber.org/zap"

	"github.com/google/go-github/v61/github"
)

func getAllPackages(ctx context.Context, client *github.Client, org string, opts *github.PackageListOptions, allPkgs []*github.Package) ([]*github.Package, error) {
	pkgs, resp, err := client.Organizations.ListPackages(ctx, org, opts)
	if err != nil {
		return nil, err
	}
	allPkgs = append(allPkgs, pkgs...)

	if resp.NextPage == 0 {
		return allPkgs, nil
	}

	opts.Page = resp.NextPage
	return getAllPackages(ctx, client, org, opts, allPkgs)
}

// GetPackages list packages and their versions from Container Registry
func GetPackages(ctx context.Context, query string, version bool, r *registry.Registry, registryUrl string, org string, logger *zap.SugaredLogger) (pterm.TableData, error) {
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

	allPkgs, err := getAllPackages(ctx, clientgh, org, opts, allPkgs)
	if err != nil {
		logger.Errorf("Cannot get packages from %s", registryUrl)
		return nil, err
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
