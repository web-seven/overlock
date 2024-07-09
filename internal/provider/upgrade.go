package provider

import (
	"context"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/charmbracelet/log"
	"github.com/google/go-containerregistry/pkg/name"
	"k8s.io/client-go/dynamic"
)

func (p *Provider) UpgradeVersion(ctx context.Context, dc dynamic.Interface, logger *log.Logger) error {
	pRef, _ := name.ParseReference(p.Name, name.WithDefaultRegistry(""))
	requestedVersion, err := semver.NewVersion(pRef.Identifier())
	if err != nil {
		return err
	}
	requestedVersion = semver.New(requestedVersion.Major(), requestedVersion.Minor(), 0, "", "")
	ePrvds := ListProviders(ctx, dc, logger)
	for _, ePrvd := range ePrvds {
		epRef, _ := name.ParseReference(ePrvd.Spec.Package, name.WithDefaultRegistry(""))
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
