package configuration

import (
	"bufio"
	"context"
	"io"
	"os"
	"strings"

	semver "github.com/Masterminds/semver/v3"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"k8s.io/client-go/dynamic"
)

const (
	tagDelim         = ":"
	regRepoDelimiter = "/"
)

// Load configuration package from TAR archive path
func (c *Configuration) LoadPathArchive(path string) error {
	image, err := tarball.ImageFromPath(path, nil)
	if err != nil {
		return err
	}
	c.Image = image
	return nil
}

// Load configuration package from STDIN
func (c *Configuration) LoadStdinArchive(stream *bufio.Reader) error {
	stdin, err := io.ReadAll(stream)
	if err != nil {
		return err
	}
	tmpFile, err := os.CreateTemp("", "kndp-configuration-*")
	if err != nil {
		return err
	}
	tmpFile.Write(stdin)
	if err != nil {
		return err
	}

	return c.LoadPathArchive(tmpFile.Name())
}

// Upgrade patch part of configuration version based on deployd configuration
// Details: https://github.com/kndpio/cli/issues/131
func (c *Configuration) UpgradeVersion(ctx context.Context, dc dynamic.Interface) error {

	cRef, _ := name.ParseReference(c.Name, name.WithDefaultRegistry(""))
	requestedVersion, err := semver.NewVersion(cRef.Identifier())
	if err != nil {
		return err
	}
	requestedVersion = semver.New(requestedVersion.Major(), requestedVersion.Minor(), 0, "", "")
	eCfgs := GetConfigurations(ctx, dc)
	for _, eCfg := range eCfgs {
		ecRef, _ := name.ParseReference(eCfg.Spec.Package, name.WithDefaultRegistry(""))
		deployedVersion, err := semver.NewVersion(ecRef.Identifier())
		if err != nil {
			return err
		}
		deployedVersion = semver.New(deployedVersion.Major(), deployedVersion.Minor(), 0, "", "")
		if ecRef.Context().Name() == cRef.Context().Name() && requestedVersion == deployedVersion {
			cRef = ecRef
		}
	}
	version, err := semver.NewVersion(cRef.Identifier())
	if err != nil {
		return err
	}

	c.Name = strings.Join([]string{cRef.Context().Name(), version.IncPatch().String()}, tagDelim)
	return nil
}
