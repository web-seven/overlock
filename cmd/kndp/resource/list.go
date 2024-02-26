package resource

import (
	"context"

	"github.com/charmbracelet/log"
	"github.com/kndpio/kndp/internal/resources"
	"github.com/rodaine/table"

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
)

type listCmd struct {
}

func (listCmd) Run(ctx context.Context, config *rest.Config, dynamicClient *dynamic.DynamicClient, logger *log.Logger) error {
	tbl := table.New("NAME", "API-VERSION", "KIND", "LABELS")

	xresources := resources.GetXResources(ctx, dynamicClient, logger)
	for _, resource := range xresources {
		labels := resources.ExtractLabels(resource.GetLabels())
		tbl.AddRow(resource.GetName(), resource.GetAPIVersion(), resource.GetKind(), labels)

	}

	tbl.Print()

	return nil
}
