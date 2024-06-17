package configuration

import (
	"context"

	"github.com/kndpio/kndp/internal/configuration"

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"

	"github.com/charmbracelet/log"
)

type applyCmd struct {
	Link    string `arg:"" required:"" help:"Link URL (or multiple comma separated) to Crossplane configuration to be applied to Environment."`
	Wait    bool   `optional:"" short:"w" help:"Wait until configuration is installed."`
	Timeout string `optional:"" short:"t" help:"Timeout is used to set how much to wait until configuration is installed (valid time units are ns, us, ms, s, m, h)"`
}

func (c *applyCmd) Run(ctx context.Context, dc *dynamic.DynamicClient, config *rest.Config, logger *log.Logger) error {
	return configuration.ApplyConfiguration(ctx, c.Link, config, dc, c.Wait, c.Timeout, logger)
}
