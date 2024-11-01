package registry

import (
	"github.com/posener/complete"
)

func PredictRegistries() complete.Predictor {
	return registries()
}

func registries() complete.PredictFunc {
	return func(a complete.Args) (prediction []string) {
		return []string{}
	}
}
