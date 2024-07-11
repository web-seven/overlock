package resource

import (
	"context"

	"github.com/ghodss/yaml"
	"github.com/kndpio/kndp/internal/resources"
	"github.com/rodaine/table"
	"go.uber.org/zap"

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
)

type listCmd struct {
}

func (listCmd) Run(ctx context.Context, config *rest.Config, dynamicClient *dynamic.DynamicClient, logger *zap.SugaredLogger) error {
	tbl := table.New("NAME", "API-VERSION", "KIND", "CREATION-DATE", "UPDATE-DATE")

	xresources := resources.GetXResources(ctx, dynamicClient, logger)
	for _, resource := range xresources {
		labels := resource.GetLabels()
		tbl.AddRow(resource.GetName(), resource.GetAPIVersion(), resource.GetKind(), labels["creation-date"], labels["update-date"])

		jsonFormat, _ := resource.MarshalJSON()
		yamlFormat, _ := yaml.JSONToYAML(jsonFormat)
		logger.Infof("\n%s JSON: \n%s\n", resource.GetName(), string(jsonFormat))
		logger.Infof("%s YAML: \n%s\n", resource.GetName(), string(yamlFormat))
	}

	if len(xresources) > 0 {
		tbl.Print()
	} else {
		logger.Info("No resources found managed by kndp.")
	}

	return nil
}
