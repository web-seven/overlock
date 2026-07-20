package environment

import (
	"context"

	"go.uber.org/zap"

	"github.com/web-seven/overlock/pkg/environment"
	overlockerrors "github.com/web-seven/overlock/pkg/errors"
)

type createCmd struct {
	Name   string `arg:"" optional:"" help:"Name of environment. If omitted, falls back to 'name' in the Overlock configuration file."`
	Config string `optional:"" help:"Path to the Overlock configuration file. Defaults to ./overlock.yaml if present."`
	createOptions
}

type createOptions struct {
	// ConfigName is the fallback environment name loaded from the configuration
	// file (see loadConfig). The CLI's positional Name argument on createCmd is
	// still the primary way to set the name and always takes precedence when set.
	ConfigName                string   `kong:"-" yaml:"name,omitempty"`
	HttpPort                  int      `optional:"" short:"p" help:"Http host port for mapping" default:"80"`
	HttpsPort                 int      `optional:"" short:"s" help:"Https host port for mapping" default:"443"`
	Context                   string   `optional:"" short:"c" help:"Kubernetes context where Environment will be created."`
	Engine                    string   `optional:"" short:"e" help:"Specifies the Kubernetes engine to use for the runtime environment (kind, k3s, k3d, k3s-docker)." default:"kind"`
	EngineConfig              string   `optional:"" help:"Path to the configuration file for the engine. Currently supported for kind clusters."`
	EngineK3sVersion          string   `optional:"" name:"engine-k3s-version" help:"k3s version for the k3s-docker engine. Defaults to v1.36.2+k3s1."`
	Mount                     []string `optional:"" help:"Bind mount in host:container format (e.g., /data:/storage). Can be specified multiple times."`
	Providers                 []string `optional:"" help:"List of providers to apply to the environment."`
	Configurations            []string `optional:"" help:"List of configurations to apply to the environment."`
	Functions                 []string `optional:"" help:"List of functions to apply to the environment."`
	CreateAdminServiceAccount bool     `optional:"" help:"Create admin service account with cluster-admin privileges."`
	AdminServiceAccountName   string   `optional:"" help:"Name for the admin service account. Only relevant when create-admin-service-account is enabled. Defaults to 'overlock-admin' if not specified."`
	Cpu                       string   `optional:"" help:"CPU limit for k3s-docker containers (e.g., 2, 0.5, 50%)." default:""`
	MaxReconcileRate          int      `optional:"" help:"Maximum number of reconciliations per second for Crossplane (e.g., 1)." default:"1"`
	// Nodes is only settable via a configuration file (see loadConfig), not as a CLI flag.
	Nodes []environment.NodeSpec `kong:"-" yaml:"nodes,omitempty"`
}

func (c *createCmd) Run(ctx context.Context, logger *zap.SugaredLogger) error {
	stop, err := c.loadAndMergeConfig(logger)
	if err != nil {
		return err
	}
	if stop {
		return nil
	}

	if c.Name == "" {
		c.Name = c.createOptions.ConfigName
	}
	if c.Name == "" {
		return overlockerrors.NewInvalidConfigError("name", "", "environment name must be provided either as a positional argument or via 'name' in the configuration file")
	}

	if len(c.Nodes) > 0 && c.Engine != "k3s-docker" {
		logger.Warnf("nodes declared in configuration are only supported for the k3s-docker engine; skipping %d node(s)", len(c.Nodes))
	}

	env := environment.
		New(c.Engine, c.Name).
		WithHttpPort(c.HttpPort).
		WithHttpsPort(c.HttpsPort).
		WithContext(c.Context).
		WithMounts(c.Mount).
		WithEngineConfig(c.EngineConfig).
		WithEngineK3sVersion(c.EngineK3sVersion).
		WithProviders(c.Providers).
		WithConfigurations(c.Configurations).
		WithFunctions(c.Functions).
		WithAdminServiceAccount(c.CreateAdminServiceAccount, c.AdminServiceAccountName).
		WithCpu(c.Cpu).
		WithMaxReconcileRate(c.MaxReconcileRate).
		WithNodes(c.Nodes)

	if err := env.Create(ctx, logger); err != nil {
		return err
	}

	return nil
}
