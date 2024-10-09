package provider

import (
	"context"

	crossv1 "github.com/crossplane/crossplane/apis/pkg/v1"
	"go.uber.org/zap"

	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/pkg/errors"
	"github.com/web-seven/overlock/internal/engine"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (p *Provider) ApplyProvider(ctx context.Context, links []string, config *rest.Config, logger *zap.SugaredLogger) error {
	scheme := runtime.NewScheme()
	crossv1.AddToScheme(scheme)
	if kube, err := client.New(config, client.Options{Scheme: scheme}); err == nil {
		for _, link := range links {
			cfg := &crossv1.Provider{}
			engine.BuildPack(cfg, link, map[string]string{})
			pa := resource.NewAPIPatchingApplicator(kube)

			if err := pa.Apply(ctx, cfg); err != nil {
				return errors.Wrap(err, "Error apply Provider(s).")
			}
		}
	} else {
		return err
	}

	logger.Info("Provider(s) applied successfully.")
	return nil
}
