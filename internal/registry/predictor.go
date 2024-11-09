package registry

import (
	"context"

	"github.com/posener/complete"
	"k8s.io/client-go/kubernetes"
)

func PredictRegistries(ctx context.Context, client *kubernetes.Clientset) complete.Predictor {
	return registries(ctx, client)
}

func registries(ctx context.Context, client *kubernetes.Clientset) complete.PredictFunc {
	return func(a complete.Args) (prediction []string) {
		regs := []string{}
		registries, err := Registries(ctx, client)
		if err != nil {
			return regs
		}
		for _, reg := range registries {
			regs = append(regs, reg.GetName())
		}
		return regs
	}
}
