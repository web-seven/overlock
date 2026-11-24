package environment

import (
	"context"

	"go.uber.org/zap"

	"github.com/web-seven/overlock/pkg/environment"
)

type deleteCmd struct {
	Name    string `arg:"" required:"" help:"Name of environment."`
	Engine  string `optional:"" help:"Specifies the Kubernetes engine to use for the runtime environment." default:"kind"`
	Confirm bool   `optional:"" short:"c" help:"Confirm deletion of overlock environment." default:"false"`
}

func (c *deleteCmd) Run(ctx context.Context, logger *zap.SugaredLogger) error {
	return environment.
		New(c.Engine, c.Name).
		Delete(c.Confirm, logger)
}
