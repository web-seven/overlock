package registry

import (
	"context"

	"github.com/posener/complete"
	"github.com/web-seven/overlock/pkg/registry"
	"k8s.io/client-go/kubernetes"
)

type Cmd struct {
	Create createCmd `cmd:"" help:"Create registry"`
	List   listCmd   `cmd:"" help:"List registries"`
	Delete deleteCmd `cmd:"" help:"Delete registry"`
}

func Predictors(ctx context.Context, client *kubernetes.Clientset) map[string]complete.Predictor {
	return map[string]complete.Predictor{
		"registry": registry.PredictRegistries(ctx, client),
	}
}
