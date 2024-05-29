package configuration

import (
	"context"
	"strings"

	"github.com/charmbracelet/log"
	"github.com/pkg/errors"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"

	"github.com/kndpio/kndp/internal/engine"

	"github.com/crossplane/crossplane-runtime/pkg/resource"
	crossv1 "github.com/crossplane/crossplane/apis/pkg/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func ApplyConfiguration(ctx context.Context, links string, config *rest.Config, logger *log.Logger) error {
	scheme := runtime.NewScheme()
	crossv1.AddToScheme(scheme)
	if kube, err := client.New(config, client.Options{Scheme: scheme}); err == nil {
		for _, link := range strings.Split(links, ",") {
			cfg := &crossv1.Configuration{}
			engine.BuildPack(cfg, link, map[string]string{})
			pa := resource.NewAPIPatchingApplicator(kube)

			if err := pa.Apply(ctx, cfg); err != nil {
				return errors.Wrap(err, "Error apply configuration(s).")
			}
		}
	} else {
		return err
	}
	logger.Info("Configuration(s) applied successfully.")
	return nil
}
