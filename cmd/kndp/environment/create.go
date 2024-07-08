package environment

import (
	"context"

	"github.com/charmbracelet/log"
	"github.com/kndpio/kndp/internal/environment"
)

type createCmd struct {
	Name              string `arg:"" requried:"" help:"Name of environment."`
	HttpPort          int    `optional:"" short:"p" help:"Http host port for mapping" default:"80"`
	HttpsPort         int    `optional:"" short:"s" help:"Https host port for mapping" default:"443"`
	Context           string `optional:"" short:"c" help:"Kubernetes context where Environment will be created."`
	Engine            string `optional:"" short:"e" help:"Specifies the Kubernetes engine to use for the runtime environment." default:"kind"`
	IngressController string `optional:"" help:"Specifies the Ingress Controller type. (Default: nginx)" default:"nginx"`
	PolicyController  string `optional:"" help:"Specifies the Policy Controller type. (Default: kyverno)" default:"kyverno"`
}

func (c *createCmd) Run(ctx context.Context, logger *log.Logger) error {
	return environment.
		New(c.Engine, c.Name).
		WithHttpPort(c.HttpPort).
		WithHttpsPort(c.HttpsPort).
		WithContext(c.Context).
		WithIngressController(c.IngressController).
		WithPolicyController(c.PolicyController).
		Create(ctx, logger)
}
