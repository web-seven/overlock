package environment

import (
	"context"

	"github.com/web-seven/overlock/internal/environment"
	"go.uber.org/zap"
)

type upgradeCmd struct {
	Name    string `arg:"" required:"" help:"Environment name where engine will be upgraded."`
	Engine  string `optional:"" help:"Specifies the Kubernetes engine to use for the runtime environment." default:"kind"`
	Context string `optional:"" short:"c" help:"Kubernetes context where Environment will be upgraded."`
}

func (c *upgradeCmd) Run(ctx context.Context, logger *zap.SugaredLogger) error {
	return environment.
		New(c.Engine, c.Name).
		WithContext(c.Context).
		Upgrade(ctx, logger)
}
