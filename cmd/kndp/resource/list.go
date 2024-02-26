package resource

import (
	"context"
	"strings"

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
		labels := extractLabels(resource.GetLabels())
		tbl.AddRow(resource.GetName(), resource.GetAPIVersion(), resource.GetKind(), labels)

	}

	tbl.Print()

	return nil
}
func extractLabels(labels map[string]string) string {
	var sb strings.Builder
	for k, v := range labels {
		sb.WriteString(k)
		sb.WriteString(": ")
		sb.WriteString(v)
		sb.WriteString(", ")
	}
	return sb.String()
}
