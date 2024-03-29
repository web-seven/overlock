package registry

import (
	"context"

	"github.com/charmbracelet/log"
	"github.com/kndpio/kndp/internal/registry"
	"k8s.io/client-go/rest"
)

type deleteCmd struct {
	Name string `required:"" help:"Registry name."`
}

func (c deleteCmd) Run(ctx context.Context, config *rest.Config, logger *log.Logger) error {
	reg := registry.Registry{}
	reg.Name = c.Name
	err := reg.Delete(ctx, config, logger)
	if err != nil {
		logger.Error(err)
	} else {
		logger.Info("Registry was removed successfully.")
	}
	return nil
}
