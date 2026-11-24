package environment

import (
	"context"

	"go.uber.org/zap"

	"github.com/web-seven/overlock/pkg/environment"
)

type stopCmd struct {
	Name   string `arg:"" required:"" help:"Name of environment."`
	Engine string `optional:"" help:"Specifies the Kubernetes engine to use for the runtime environment." default:"kind"`
}

func (c *stopCmd) Run(ctx context.Context, logger *zap.SugaredLogger) error {
	return environment.
		New(c.Engine, c.Name).
		Stop(ctx, logger)
}
