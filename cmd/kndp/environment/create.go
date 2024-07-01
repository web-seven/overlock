package environment

import (
	"context"

	"github.com/charmbracelet/log"
	"github.com/kndpio/kndp/internal/environment"
)

type createCmd struct {
	Name     string `arg:"" requried:"" help:"Name of environment."`
	HostPort int    `optional:"" short:"p" help:"Host port for mapping" default:"80"`
	Context  string `optional:"" short:"c" help:"Kubernetes context where Environment will be created."`
	Engine   string `optional:"" short:"e" help:"Specifies the Kubernetes engine to use for the runtime environment." default:"kind"`
}

func (c *createCmd) Run(ctx context.Context, logger *log.Logger) error {
	return environment.Create(ctx, c.Context, c.Engine, c.Name, c.HostPort, logger)
}
