package registry

import (
	"context"

	"github.com/web-seven/overlock/internal/registry"
	"go.uber.org/zap"
	"k8s.io/client-go/rest"
)

type deleteCmd struct {
	Name    string `required:"" help:"Registry name."`
	Default bool   `help:"Remove from default."`
}

func (c deleteCmd) Run(ctx context.Context, config *rest.Config, logger *zap.SugaredLogger) error {
	reg := registry.Registry{}
	reg.Name = c.Name
	reg.SetDefault(c.Default)
	reg.SetLocal(c.Name == registry.LocalRegistryName)
	err := reg.Delete(ctx, config, logger)
	if err != nil {
		logger.Info(err)
	} else {
		logger.Info("Registry was removed successfully.")
	}
	return nil
}
