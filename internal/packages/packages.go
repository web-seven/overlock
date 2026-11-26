package packages

import (
	"context"
	"strings"

	"github.com/web-seven/overlock/internal/engine"

	"github.com/google/go-containerregistry/pkg/name"

	semver "github.com/Masterminds/semver/v3"
	"k8s.io/client-go/dynamic"
)

const (
	tagDelim    = ":"
	PackagePath = "package"
)

type Package struct {
	Name string
	Url  string
}

func (p *Package) UpgradeVersion(ctx context.Context, dc dynamic.Interface, pname string, pkgs []Package) (string, error) {
	pRef, _ := name.ParseReference(pname, name.WithDefaultRegistry(""))
	requestedVersion, err := semver.NewVersion(pRef.Identifier())
	if err != nil {
		return "", err
	}

	// Start from the deployed version of the same package and minor version,
	// if any. Deployed packages are fully qualified, so a reference without a
	// registry matches the same repository in the local registry.
	base := requestedVersion
	requestedMinorVersion := semver.New(requestedVersion.Major(), requestedVersion.Minor(), 0, "", "")
	for _, pkg := range pkgs {
		epRef, _ := name.ParseReference(pkg.Url, name.WithDefaultRegistry(""))
		if !sameRepository(pRef, epRef) {
			continue
		}
		deployedVersion, err := semver.NewVersion(epRef.Identifier())
		if err != nil {
			continue
		}
		deployedMinorVersion := semver.New(deployedVersion.Major(), deployedVersion.Minor(), 0, "", "")
		if requestedMinorVersion.String() == deployedMinorVersion.String() {
			base = deployedVersion
		}
	}

	// Keep the "v" prefix of the requested tag, which semver drops.
	prefix := ""
	if strings.HasPrefix(pRef.Identifier(), "v") {
		prefix = "v"
	}
	return strings.Join([]string{pRef.Context().Name(), prefix + base.IncPatch().String()}, tagDelim), nil
}

// sameRepository reports whether two package references name the same
// repository; a reference without a registry refers to the local registry.
func sameRepository(a, b name.Reference) bool {
	registry := func(r name.Reference) string {
		if r.Context().RegistryStr() == "" {
			return engine.LocalRegistryDomain()
		}
		return r.Context().RegistryStr()
	}
	return registry(a) == registry(b) && a.Context().RepositoryStr() == b.Context().RepositoryStr()
}
