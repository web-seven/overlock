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

func (p *Package) UpgradeVersion(ctx context.Context, dc dynamic.Interface, pkgs []Package) error {
	pRef, _ := name.ParseReference(p.Name, name.WithDefaultRegistry(""))
	requestedVersion, err := semver.NewVersion(pRef.Identifier())
	if err != nil {
		return err
	}
	requestedVersion = semver.New(requestedVersion.Major(), requestedVersion.Minor(), 0, "", "")
	for _, pkg := range pkgs {
		epRef, _ := name.ParseReference(pkg.Url, name.WithDefaultRegistry(""))
		deployedVersion, err := semver.NewVersion(epRef.Identifier())
		if err != nil {
			return err
		}
		deployedVersion = semver.New(deployedVersion.Major(), deployedVersion.Minor(), 0, "", "")
		if epRef.Context().Name() == pRef.Context().Name() && requestedVersion.String() == deployedVersion.String() {
			pRef = epRef
		}
	}
	version, err := semver.NewVersion(pRef.Identifier())
	if err != nil {
		return err
	}
	p.Name = strings.Join([]string{pRef.Context().Name(), version.IncPatch().String()}, tagDelim)
	return nil
}
