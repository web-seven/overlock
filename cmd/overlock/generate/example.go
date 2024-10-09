package generate

import (
	"context"

	"github.com/web-seven/overlock/internal/generate"
	"go.uber.org/zap"
)

type exampleCmd struct {
	Path string `help:"Path to CRD YAML file archive."`
}

func (c *exampleCmd) Run(ctx context.Context, logger *zap.SugaredLogger) error {
	return generate.GenerateCompositeResource(ctx, c.Path, logger)
}
