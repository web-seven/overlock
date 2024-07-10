package environment

import (
	"context"

	"github.com/kndpio/kndp/internal/environment"
	"go.uber.org/zap"
)

type deleteCmd struct {
	Name   string `arg:"" required:"" help:"Name of environment."`
	Engine string `optional:"" help:"Specifies the Kubernetes engine to use for the runtime environment." default:"kind"`
}

func (c *deleteCmd) Run(ctx context.Context, logger *zap.Logger) error {
	return environment.
		New(c.Engine, c.Name).
		Delete(logger)
}
