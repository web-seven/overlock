package registry

import (
	"context"

	"github.com/posener/complete"
	"k8s.io/client-go/kubernetes"

	"github.com/web-seven/overlock/pkg/registry"
)

type Cmd struct {
	Create    createCmd    `cmd:"" help:"Create registry"`
	List      listCmd      `cmd:"" help:"List registries"`
	Delete    deleteCmd    `cmd:"" help:"Delete registry"`
	LoadImage loadImageCmd `cmd:"" name:"load-image" help:"Load OCI image to registry"`
}

func Predictors(ctx context.Context, client *kubernetes.Clientset) map[string]complete.Predictor {
	return map[string]complete.Predictor{
		"registry": registry.PredictRegistries(ctx, client),
	}
}
