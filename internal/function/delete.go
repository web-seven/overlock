package function

import (
	"context"
	"strings"

	crossv1 "github.com/crossplane/crossplane/apis/pkg/v1beta1"
	"github.com/web-seven/overlock/internal/engine"
	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"
)

func DeleteFunction(ctx context.Context, urls string, dynamicClient *dynamic.DynamicClient, logger *zap.SugaredLogger) error {

	for _, url := range strings.Split(urls, ",") {
		cfg := crossv1.Function{}
		engine.BuildPack(&cfg, url, map[string]string{})

		err := dynamicClient.Resource(ResourceId()).Namespace("").Delete(ctx, cfg.GetName(), metav1.DeleteOptions{})
		if err != nil {
			return err
		}
	}

	logger.Info("Function(s) removed successfully.")
	return nil
}
