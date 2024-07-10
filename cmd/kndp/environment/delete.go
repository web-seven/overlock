package environment

import (
	"context"

	"github.com/charmbracelet/log"
	"github.com/kndpio/kndp/internal/environment"
)

type deleteCmd struct {
	Name   string `arg:"" required:"" help:"Name of environment."`
	Engine string `optional:"" help:"Specifies the Kubernetes engine to use for the runtime environment." default:"kind"`
	Force  bool   `optional:"" short:"f" help:"Force deletion of kndp environment." default:"false"`
}

func (c *deleteCmd) Run(ctx context.Context, logger *log.Logger) error {
	return environment.
		New(c.Engine, c.Name).
		Delete(c.Force, logger)
}
