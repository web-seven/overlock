package configuration

import (
	"context"
	"strings"

	crossv1 "github.com/crossplane/crossplane/apis/pkg/v1"
	"github.com/kndpio/kndp/internal/engine"
	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"
)

func DeleteConfiguration(ctx context.Context, urls string, dynamicClient *dynamic.DynamicClient, logger *zap.SugaredLogger) error {

	for _, url := range strings.Split(urls, ",") {
		cfg := crossv1.Configuration{}
		engine.BuildPack(&cfg, url, map[string]string{})

		err := dynamicClient.Resource(ResourceId()).Namespace("").Delete(ctx, cfg.GetName(), metav1.DeleteOptions{})
		if err != nil {
			return err
		}
	}

	logger.Info("Configuration(s) removed successfully.")
	return nil
}
