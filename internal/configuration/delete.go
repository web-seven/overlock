package configuration

import (
	"context"

	"github.com/charmbracelet/log"
	crossv1 "github.com/crossplane/crossplane/apis/pkg/v1"
	"github.com/kndpio/kndp/internal/engine"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"
)

func DeleteConfiguration(ctx context.Context, url string, dynamicClient *dynamic.DynamicClient, logger *log.Logger) error {

	cfg := crossv1.Configuration{}
	engine.BuildPack(&cfg, url, map[string]string{})

	err := dynamicClient.Resource(ResourceId()).Namespace("").Delete(ctx, cfg.GetName(), metav1.DeleteOptions{})
	if err != nil {
		return err
	}

	logger.Info("Configuration removed successfully.")
	return nil
}
