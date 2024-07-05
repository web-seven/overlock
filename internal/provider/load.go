package provider

import "github.com/google/go-containerregistry/pkg/v1/tarball"

// Load Provider package from TAR archive path
func (p *Provider) LoadPathArchive(path string) error {
	image, err := tarball.ImageFromPath(path, nil)
	if err != nil {
		return err
	}
	p.Image = image
	return nil
}
