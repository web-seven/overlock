package loader

import (
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
)

// Load LoadPathArchive from TAR archive path
func LoadPathArchive(path string) (v1.Image, error) {
	image, err := tarball.ImageFromPath(path, nil)
	if err != nil {
		return nil, err
	}
	return image, nil
}
