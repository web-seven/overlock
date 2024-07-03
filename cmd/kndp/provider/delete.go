package provider

import (
	"context"

	"github.com/charmbracelet/log"
	"github.com/kndpio/kndp/internal/provider"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
)

type deleteCmd struct {
	ProviderUrl string `arg:"" required:"" help:"Crossplane provider package URL to be removed from the environment."`
}

func (c *deleteCmd) Run(ctx context.Context, config *rest.Config, dynamicClient *dynamic.DynamicClient, logger *log.Logger) error {
	return provider.DeleteProvider(ctx, dynamicClient, c.ProviderUrl, logger)
}
