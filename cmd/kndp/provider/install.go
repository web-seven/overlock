package provider

import (
	"context"

	"github.com/charmbracelet/log"

	"github.com/kndpio/kndp/internal/provider"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
)

type installCmd struct {
	ProviderUrl string `arg:"" required:"" help:"Provider URL to Crossplane provider to be installed to Environment."`
}

func (c *installCmd) Run(ctx context.Context, config *rest.Config, dynamicClient *dynamic.DynamicClient, logger *log.Logger) error {
	provider.InstallProvider(c.ProviderUrl, config, logger)

	return nil
}
