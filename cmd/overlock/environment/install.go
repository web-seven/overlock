package environment

import (
	"context"
	"fmt"
	"strings"

	"go.uber.org/zap"

	"github.com/web-seven/overlock/pkg/environment"
)

type installCmd struct {
	Name           string   `arg:"" required:"" help:"Name of environment."`
	Context        string   `required:"" short:"c" help:"Kubernetes context to install the engine into."`
	NodeLabel      []string `optional:"" help:"Node label in key=value format used as a nodeSelector to schedule the engine onto selected node(s). Can be specified multiple times."`
	NodeTaint      []string `optional:"" help:"Node taint in key=value:Effect format; a matching toleration is added so the engine can be placed on tainted node(s). Can be specified multiple times."`
	Providers      []string `optional:"" help:"List of providers to apply to the environment."`
	Configurations []string `optional:"" help:"List of configurations to apply to the environment."`
	Functions      []string `optional:"" help:"List of functions to apply to the environment."`
}

func (c *installCmd) Run(ctx context.Context, logger *zap.SugaredLogger) error {
	nodeSelector, err := parseNodeLabels(c.NodeLabel)
	if err != nil {
		return err
	}

	tolerations, err := parseNodeTaints(c.NodeTaint)
	if err != nil {
		return err
	}

	return environment.
		New("", c.Name).
		WithContext(c.Context).
		WithProviders(c.Providers).
		WithConfigurations(c.Configurations).
		WithFunctions(c.Functions).
		WithNodeSelector(nodeSelector).
		WithTolerations(tolerations).
		Install(ctx, logger)
}

// parseNodeLabels parses "key=value" strings into a nodeSelector map.
func parseNodeLabels(labels []string) (map[string]interface{}, error) {
	if len(labels) == 0 {
		return nil, nil
	}
	nodeSelector := make(map[string]interface{}, len(labels))
	for _, label := range labels {
		key, value, ok := strings.Cut(label, "=")
		if !ok || key == "" || value == "" {
			return nil, fmt.Errorf("invalid --node-label %q, expected key=value", label)
		}
		nodeSelector[key] = value
	}
	return nodeSelector, nil
}

// parseNodeTaints parses "key=value:Effect" strings into tolerations matching those taints.
func parseNodeTaints(taints []string) ([]interface{}, error) {
	if len(taints) == 0 {
		return nil, nil
	}
	tolerations := make([]interface{}, 0, len(taints))
	for _, taint := range taints {
		keyValue, effect, ok := strings.Cut(taint, ":")
		if !ok || effect == "" {
			return nil, fmt.Errorf("invalid --node-taint %q, expected key=value:Effect", taint)
		}
		key, value, ok := strings.Cut(keyValue, "=")
		if !ok || key == "" || value == "" {
			return nil, fmt.Errorf("invalid --node-taint %q, expected key=value:Effect", taint)
		}
		tolerations = append(tolerations, map[string]interface{}{
			"key":      key,
			"operator": "Equal",
			"value":    value,
			"effect":   effect,
		})
	}
	return tolerations, nil
}
