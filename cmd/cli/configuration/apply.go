package configuration

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"

	"github.com/web-seven/overlock/pkg/configuration"
)

type applyCmd struct {
	Link    string `arg:"" required:"" help:"Link URL (or multiple comma separated) to Crossplane configuration to be applied to Environment."`
	Wait    bool   `optional:"" short:"w" help:"Wait until configuration is installed."`
	Timeout string `optional:"" short:"t" help:"Timeout is used to set how much to wait until configuration is installed (valid time units are ns, us, ms, s, m, h)"`
}

func (c *applyCmd) Run(ctx context.Context, dc *dynamic.DynamicClient, config *rest.Config, logger *zap.SugaredLogger) error {
	cfg := configuration.New(c.Link)
	if err := cfg.Apply(ctx, config, logger); err != nil {
		return fmt.Errorf("failed to apply configuration: %w", err)
	}
	if !c.Wait {
		return nil
	}

	var timeoutChan <-chan time.Time
	if c.Timeout != "" {
		timeout, err := time.ParseDuration(c.Timeout)
		if err != nil {
			return fmt.Errorf("failed to parse timeout duration: %w", err)
		}
		timeoutChan = time.After(timeout)
	}
	if err := configuration.HealthCheck(ctx, dc, c.Link, c.Wait, timeoutChan, logger); err != nil {
		return fmt.Errorf("configuration health check failed: %w", err)
	}
	return nil
}
