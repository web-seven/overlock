package environment

import (
	"context"

	"go.uber.org/zap"

	"github.com/web-seven/overlock/pkg/environment"
)

type nodeCmd struct {
	Create nodeCreateCmd `cmd:"" help:"Create a new node in an Environment"`
	Delete nodeDeleteCmd `cmd:"" help:"Delete a node from an Environment"`
}

type nodeCreateCmd struct {
	Name        string   `arg:"" required:"" help:"Name of the node."`
	Environment string   `required:"" help:"Name of the target environment (k3s cluster)."`
	Engine      string   `optional:"" help:"Specifies the Kubernetes engine to use for the runtime environment." default:"k3s-docker"`
	Scopes      []string `optional:"" help:"Comma-separated list of node scopes (engine, workloads)."`
}

func (c *nodeCreateCmd) Run(ctx context.Context, logger *zap.SugaredLogger) error {
	return environment.
		New(c.Engine, c.Environment).
		CreateNode(ctx, c.Name, c.Scopes, logger)
}

type nodeDeleteCmd struct {
	Name        string   `arg:"" required:"" help:"Name of the node to delete."`
	Environment string   `required:"" help:"Name of the target environment."`
	Engine      string   `optional:"" help:"Specifies the Kubernetes engine to use for the runtime environment." default:"k3s-docker"`
	Scopes      []string `optional:"" help:"Comma-separated list of node scopes (engine, workloads)."`
}

func (c *nodeDeleteCmd) Run(ctx context.Context, logger *zap.SugaredLogger) error {
	return environment.
		New(c.Engine, c.Environment).
		DeleteNode(ctx, c.Name, c.Scopes, logger)
}
