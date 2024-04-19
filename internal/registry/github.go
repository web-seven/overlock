package registry

import (
	"context"
	"encoding/json"

	"github.com/charmbracelet/log"

	"github.com/google/go-github/v61/github"
)

// GetGithubPackages list packages and their versions from GitHub Container Registry
func GetGithubPackages(ctx context.Context, query string, version bool, r *Registry, registryUrl string, org string, logger *log.Logger) ([]*github.Package, map[string][]*github.PackageVersion, error) {
	auth := RegistryConfig{}
	json.Unmarshal([]byte(r.Data[".dockerconfigjson"]), &auth)
	clientgh := github.NewClient(nil).WithAuthToken(auth.Auths[registryUrl].Password)

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
	return pkgs, pkgVersions, nil
}
