package configuration

import (
	"context"

	"github.com/kndpio/kndp/internal/configuration"

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"

	"github.com/charmbracelet/log"
)

type listCmd struct {
}

func (listCmd) Run(ctx context.Context, config *rest.Config, dynamicClient *dynamic.DynamicClient, logger *log.Logger) error {
	configurations := configuration.GetConfigurations(ctx, config, dynamicClient)
	logger.SetReportTimestamp(false)
	logger.Printf("%-30s %-30s", "NAME", "PACKAGE")
	for _, conf := range configurations {
		logger.Printf("%-30s %-30s", conf.Name, conf.Spec.Package)
	}

	return nil
}
