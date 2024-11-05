package function

import (
	"bufio"
	"io"
	"os"

	"github.com/web-seven/overlock/internal/loader"
)

const (
	tagDelim         = ":"
	regRepoDelimiter = "/"
)

// Load function package from STDIN
func (c *Function) LoadStdinArchive(stream *bufio.Reader) error {
	stdin, err := io.ReadAll(stream)
	if err != nil {
		return err
	}
	tmpFile, err := os.CreateTemp("", "overlock-function-*")
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
