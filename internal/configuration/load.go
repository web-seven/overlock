package configuration

import (
	"bufio"

	"github.com/charmbracelet/log"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
)

func (c *Configuration) LoadPathArchive(path string, logger *log.Logger) error {
	image, err := tarball.ImageFromPath(path, nil)
	if err != nil {
		return err
	}
	c.Image = image
	return nil
}

func (c *Configuration) LoadStdinArchive(stream *bufio.Reader) error {
	return nil
}
