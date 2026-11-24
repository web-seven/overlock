package registry

import (
	"context"

	"github.com/pterm/pterm"
	"go.uber.org/zap"
	"k8s.io/client-go/kubernetes"

	"github.com/web-seven/overlock/pkg/registry"
)

type listCmd struct {
}

func (c listCmd) Run(ctx context.Context, client *kubernetes.Clientset, logger *zap.SugaredLogger) error {
	registries, err := registry.Registries(ctx, client)
	if err != nil {
		logger.Error(err)
	}

	tableRegs := pterm.TableData{
		[]string{"NAME", "SERVER", "DATE"},
	}

	for _, reg := range registries {
		tableRegs = append(tableRegs, []string{
			reg.GetName(),
			reg.Annotations["overlock-registry-server-url"],
			reg.CreationTimestamp.String(),
		})
	}
	pterm.DefaultTable.WithHasHeader().WithData(tableRegs).Render()

	return nil
}
