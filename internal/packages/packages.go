package packages

import (
	"context"
	"strings"

	"github.com/google/go-containerregistry/pkg/name"

	"github.com/Masterminds/semver/v3"
	"k8s.io/client-go/dynamic"
)

const (
	tagDelim = ":"
)

type Package struct {
	Name string
	Url  string
}

func (p *Package) UpgradeVersion(ctx context.Context, dc dynamic.Interface, pname string, pkgs []Package) string {
	pRef, _ := name.ParseReference(pname, name.WithDefaultRegistry(""))
	requestedVersion, _ := semver.NewVersion(pRef.Identifier())
	requestedVersion = semver.New(requestedVersion.Major(), requestedVersion.Minor(), 0, "", "")
	for _, pkg := range pkgs {
		epRef, _ := name.ParseReference(pkg.Url, name.WithDefaultRegistry(""))
		deployedVersion, _ := semver.NewVersion(epRef.Identifier())
		deployedVersion = semver.New(deployedVersion.Major(), deployedVersion.Minor(), 0, "", "")
		if epRef.Context().Name() == pRef.Context().Name() && requestedVersion.String() == deployedVersion.String() {
			pRef = epRef
		}
	}
	version, _ := semver.NewVersion(pRef.Identifier())
	return strings.Join([]string{pRef.Context().Name(), version.IncPatch().String()}, tagDelim)
}
