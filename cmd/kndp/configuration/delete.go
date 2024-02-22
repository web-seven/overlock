package configuration

import (
	"context"

	"github.com/kndpio/kndp/internal/kube"

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"

	"github.com/charmbracelet/log"
)

type deleteCmd struct {
	ConfigurationName string `arg:"" required:"" help:"Specifies the name of configuration to be deleted from Environment."`
}

func (c *deleteCmd) Run(ctx context.Context, config *rest.Config, dynamicClient *dynamic.DynamicClient, logger *log.Logger) error {
	var params = kube.ResourceParams{
		Dynamic:   dynamicClient,
		Ctx:       ctx,
		Group:     "pkg.crossplane.io",
		Version:   "v1",
		Resource:  "configurations",
		Namespace: "",
	}
	err := kube.DeleteKubeResources(ctx, params, c.ConfigurationName)
	if err != nil {
		logger.Error(err)
	} else {
		logger.Info("Configuration deleted succesfully.")
	}
	return nil
}
