package function

import (
	"context"
	"strings"
	"time"

	"github.com/pkg/errors"
	"go.uber.org/zap"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"

	"github.com/web-seven/overlock/internal/engine"

	"github.com/crossplane/crossplane-runtime/pkg/resource"
	crossv1 "github.com/crossplane/crossplane/apis/pkg/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const apiName = "functions.pkg.crossplane.io"

// RunFunctionHealthCheck performs a health check on functions defined by the links string.
func HealthCheck(ctx context.Context, dc dynamic.Interface, links string, wait bool, timeoutChan <-chan time.Time, logger *zap.SugaredLogger) error {

	linkSet := make(map[string]struct{})
	for _, link := range strings.Split(links, ",") {
		linkSet[link] = struct{}{}
	}
	cfgs := GetFunctions(ctx, dc)

	for {
		select {
		case <-timeoutChan:
			logger.Error("Timeout reached.")
			return nil
		default:
			allHealthy := true
			for _, cfg := range cfgs {
				if _, linkMatched := linkSet[cfg.Spec.Package]; linkMatched {
					if !CheckHealthStatus(cfg.Status.Conditions) {
						allHealthy = false
						break
					}
				}
			}
			if allHealthy {
				logger.Info("Function(s) are healthy.")
				return nil
			}
			time.Sleep(5 * time.Second)
		}
	}
}

func ApplyFunction(ctx context.Context, links string, config *rest.Config, logger *zap.SugaredLogger) error {

	_, err := engine.VerifyApi(ctx, config, apiName)
	if err != nil {
		logger.Debug(err)
		logger.Infoln("Crossplane not installed in current context.")
		logger.Infoln("Function not applied.")
		return nil
	}

	scheme := runtime.NewScheme()
	crossv1.AddToScheme(scheme)
	if kube, err := client.New(config, client.Options{Scheme: scheme}); err == nil {
		for _, link := range strings.Split(links, ",") {
			cfg := &crossv1.Function{}
			engine.BuildPack(cfg, link, map[string]string{})
			pa := resource.NewAPIPatchingApplicator(kube)

			if err := pa.Apply(ctx, cfg); err != nil {
				return errors.Wrap(err, "Error apply function(s).")
			}
		}
	} else {
		return err
	}

	logger.Info("Function(s) applied successfully.")
	return nil
}

func (c *Function) Apply(ctx context.Context, config *rest.Config, logger *zap.SugaredLogger) error {

	_, err := engine.VerifyApi(ctx, config, apiName)
	if err != nil {
		logger.Debug(err)
		logger.Infoln("Crossplane not installed in current context.")
		logger.Infoln("Function not applied.")
		return nil
	}

	scheme := runtime.NewScheme()
	crossv1.AddToScheme(scheme)
	if kube, err := client.New(config, client.Options{Scheme: scheme}); err == nil {
		for _, link := range strings.Split(c.Name, ",") {
			cfg := &crossv1.Function{}
			logger.Debugf("Building package %s", link)
			engine.BuildPack(cfg, link, map[string]string{})
			pa := resource.NewAPIPatchingApplicator(kube)

			if err := pa.Apply(ctx, cfg); err != nil {
				return errors.Wrap(err, "Error apply Function(s).")
			}
		}
	} else {
		return err
	}

	logger.Info("Function(s) applied successfully.")
	return nil
}
