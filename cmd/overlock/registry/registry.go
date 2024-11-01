package registry

import (
	"github.com/posener/complete"
	"github.com/web-seven/overlock/internal/registry"
)

type Cmd struct {
	Create createCmd `cmd:"" help:"Create registry"`
	List   listCmd   `cmd:"" help:"List registries"`
	Delete deleteCmd `cmd:"" help:"Delete registry"`
}

func Predictors() map[string]complete.Predictor {
	return map[string]complete.Predictor{
		"registry": registry.PredictRegistries(),
	}
}
