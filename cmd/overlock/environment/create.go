package environment

import (
	"context"

	"github.com/web-seven/overlock/pkg/environment"
	"go.uber.org/zap"
)

type createCmd struct {
	Name         string `arg:"" requried:"" help:"Name of environment."`
	HttpPort     int    `optional:"" short:"p" help:"Http host port for mapping" default:"80"`
	HttpsPort    int    `optional:"" short:"s" help:"Https host port for mapping" default:"443"`
	Context      string `optional:"" short:"c" help:"Kubernetes context where Environment will be created."`
	Engine       string `optional:"" short:"e" help:"Specifies the Kubernetes engine to use for the runtime environment." default:"kind"`
	EngineConfig string `optional:"" help:"Path to the configuration file for the engine. Currently supported for kind clusters."`
	MountPath    string `optional:"" help:"Path for mount to /storage host directory. By default no mounts."`
}

func (c *createCmd) Run(ctx context.Context, logger *zap.SugaredLogger) error {
	return environment.
		New(c.Engine, c.Name).
		WithHttpPort(c.HttpPort).
		WithHttpsPort(c.HttpsPort).
		WithContext(c.Context).
		WithMountPath(c.MountPath).
		WithEngineConfig(c.EngineConfig).
		Create(ctx, logger)
}
