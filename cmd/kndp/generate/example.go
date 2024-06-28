package generate

import (
	"context"

	"github.com/charmbracelet/log"
	"github.com/kndpio/kndp/internal/generate"
)

type exampleCmd struct {
	Path string `help:"Path to CRD YAML file archive."`
}

func (c *exampleCmd) Run(ctx context.Context, logger *log.Logger) error {
	return generate.GenerateCompositeResource(ctx, c.Path, logger)
}
