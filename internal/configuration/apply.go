package configuration

import (
	"context"
	"strings"
	"time"

	"github.com/charmbracelet/log"
	"github.com/pkg/errors"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"

	"github.com/kndpio/kndp/internal/engine"

	"github.com/crossplane/crossplane-runtime/pkg/resource"
	crossv1 "github.com/crossplane/crossplane/apis/pkg/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// RunConfigurationHealthCheck performs a health check on configurations defined by the links string.
func RunConfigurationHealthCheck(ctx context.Context, dc dynamic.Interface, links string, wait bool, timer string, logger *log.Logger) error {
	var timeoutChan <-chan time.Time
	if !wait {
		return nil
	}

	if timer != "" {
		timeout, err := time.ParseDuration(timer)
		if err != nil {
			return err
		}
		timeoutChan = time.After(timeout)
	}

	linkSet := make(map[string]struct{})
	for _, link := range strings.Split(links, ",") {
		linkSet[link] = struct{}{}
	}
	cfgs := GetConfigurations(ctx, dc)

	for {
		select {
		case <-timeoutChan:
			logger.Error("Timeout reached.")
			return nil
		default:
			allHealthy := true
			for _, cfg := range cfgs {
				if _, linkMatched := linkSet[cfg.Spec.Package]; linkMatched {
					condition := cfg.Status.Conditions
					isHealthy := CheckHealthStatus(condition)
					if !isHealthy {
						allHealthy = false
						break
					}
				}
			}
			if allHealthy {
				logger.Info("Configuration(s) are healthy.")
				return nil
			}
			time.Sleep(5 * time.Second)
		}
	}
}

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
