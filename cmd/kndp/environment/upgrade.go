package environment

import (
	"context"

	"github.com/charmbracelet/log"
	"github.com/kndpio/kndp/internal/environment"
)

type upgradeCmd struct {
	Name              string `arg:"" required:"" help:"Environment name where engine will be upgraded."`
	Engine            string `arg:"" required:"" help:"Specifies the Kubernetes engine to use for the runtime environment." default:"kind"`
	Context           string `optional:"" short:"c" help:"Kubernetes context where Environment will be upgraded."`
	IngressController string `optional:"" help:"Specifies the Ingress Controller type. (Default: nginx)" default:"nginx"`
}

func (c *upgradeCmd) Run(ctx context.Context, logger *log.Logger) error {
	return environment.
		New(c.Engine, c.Name).
		WithContext(c.Context).
		WithIngressController(c.IngressController).
		Upgrade(ctx, logger)
}
