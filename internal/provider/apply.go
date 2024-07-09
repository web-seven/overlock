package provider

import (
	"context"
	"strings"

	crossv1 "github.com/crossplane/crossplane/apis/pkg/v1"

	"github.com/charmbracelet/log"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/kndpio/kndp/internal/engine"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (p *Provider) ApplyProvider(ctx context.Context, links string, config *rest.Config, logger *log.Logger) error {
	scheme := runtime.NewScheme()
	crossv1.AddToScheme(scheme)
	if kube, err := client.New(config, client.Options{Scheme: scheme}); err == nil {
		for _, link := range strings.Split(links, ",") {
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
