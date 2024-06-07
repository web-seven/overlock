package configuration

import (
	"bufio"
	"io"
	"os"

	"github.com/google/go-containerregistry/pkg/v1/tarball"
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
