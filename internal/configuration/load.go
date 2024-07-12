package configuration

import (
	"bufio"
	"io"
	"os"

	"github.com/kndpio/kndp/internal/loader"
)

const (
	tagDelim         = ":"
	regRepoDelimiter = "/"
)

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
	c.Image, err = loader.LoadPathArchive(tmpFile.Name())
	return err
}
