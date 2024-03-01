package registry

import (
	"context"

	"github.com/charmbracelet/log"
	"github.com/kndpio/kndp/internal/registry"
	"k8s.io/client-go/kubernetes"
)

type deleteCmd struct {
	Name string `required:"" help:"Registry name."`
}

func (c deleteCmd) Run(ctx context.Context, client *kubernetes.Clientset, logger *log.Logger) error {
	reg := registry.Registry{Name: c.Name}
	err := reg.Delete(ctx, client)
	if err != nil {
		logger.Error(err)
	} else {
		logger.Info("Registry was removed successfully.")
	}
	return nil
}
