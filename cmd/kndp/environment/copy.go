package environment

import (
	"context"

	"github.com/kndpio/kndp/internal/environment"

	"go.uber.org/zap"
)

type copyCmd struct {
	Source       string `arg:"" required:"" help:"Name source of environment."`
	Destination  string `arg:"" required:"" help:"Name destination of environment."`
	SourceEngine string `arg:"" required:"" help:"Specifies the Kubernetes engine to use for the runtime environment." default:"kind"`
}

func (c *copyCmd) Run(ctx context.Context, logger *zap.SugaredLogger) error {
	return environment.
		New(c.Source, c.Source).
		CopyEnvironment(ctx, logger, c.Source, c.Destination)
}
