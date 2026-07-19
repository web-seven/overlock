package environment

import (
	"context"
	"errors"
	"os"
	"path/filepath"

	"dario.cat/mergo"
	"go.uber.org/zap"
	yaml "gopkg.in/yaml.v2"

	"github.com/web-seven/overlock/pkg/environment"
	overlockerrors "github.com/web-seven/overlock/pkg/errors"
)

type createCmd struct {
	Name   string `arg:"" required:"" help:"Name of environment."`
	Config string `optional:"" help:"Path to the Overlock configuration file. Defaults to ./overlock.yaml if present."`
	createOptions
}

type createOptions struct {
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
	Nodes []NodeConfig `kong:"-" yaml:"nodes,omitempty"`
}

// NodeConfig declares a node to create after the environment is up, equivalent
// to an "overlock env node create" invocation. Only meaningful for the
// k3s-docker engine.
type NodeConfig struct {
	Name   string   `yaml:"name"`
	Host   string   `yaml:"host,omitempty"`
	User   string   `yaml:"user,omitempty"`
	Port   int      `yaml:"port,omitempty"`
	Key    string   `yaml:"key,omitempty"`
	Scopes []string `yaml:"scopes,omitempty"`
	Taints []string `yaml:"taints,omitempty"`
	Cpu    string   `yaml:"cpu,omitempty"`
	Mount  []string `yaml:"mount,omitempty"`
}

func (c *createCmd) Run(ctx context.Context, logger *zap.SugaredLogger) error {
	if c.Config != "" {
		cfg, err := loadConfig(c.Config)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				logger.Errorf("Configuration file not found at specified path: %s", c.Config)
				return err
			}
			logger.Infof("Failed to parse the configuration file at '%s'.", c.Config)
			logger.Info("For guidance on the correct structure, refer to the documentation: https://docs.overlock.network/environment/cfg-file")
			return nil
		}
		if err := mergo.MergeWithOverwrite(&c.createOptions, cfg, mergo.WithOverride); err != nil {
			logger.Errorf("Failed to merge configuration: %v", err)
			return overlockerrors.NewInvalidConfigErrorWithCause("", "", "failed to merge configuration options", err)
		}
	} else {
		paths, err := layeredConfigPaths()
		if err != nil {
			return err
		}
		for _, path := range paths {
			cfg, err := loadConfig(path)
			if err != nil {
				if errors.Is(err, os.ErrNotExist) {
					continue
				}
				logger.Infof("Failed to parse the configuration file at '%s'.", path)
				logger.Info("For guidance on the correct structure, refer to the documentation: https://docs.overlock.network/environment/cfg-file")
				return nil
			}
			if err := mergo.MergeWithOverwrite(&c.createOptions, cfg, mergo.WithOverride); err != nil {
				logger.Errorf("Failed to merge configuration: %v", err)
				return overlockerrors.NewInvalidConfigErrorWithCause("", "", "failed to merge configuration options", err)
			}
		}
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
		WithMaxReconcileRate(c.MaxReconcileRate)

	if err := env.Create(ctx, logger); err != nil {
		return err
	}

	for _, node := range c.Nodes {
		if err := createNode(ctx, env.WithCpu(node.Cpu), node.Name, node.Scopes, node.Taints, node.Host, node.User, node.Port, node.Key, node.Mount, logger); err != nil {
			return err
		}
	}
	return nil
}

// layeredConfigPaths returns the ordered list of config file paths to load and merge.
// Load order: overlock.yaml, .overlock.yaml, .overlock.*.yaml (alphabetically sorted).
func layeredConfigPaths() ([]string, error) {
	paths := []string{"overlock.yaml", ".overlock.yaml"}
	matches, err := filepath.Glob(".overlock.*.yaml")
	if err != nil {
		return nil, err
	}
	paths = append(paths, matches...)
	return paths, nil
}

func loadConfig(path string) (*createOptions, error) {
	var cfg createOptions

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, overlockerrors.NewInvalidConfigErrorWithCause("", "", "failed to parse configuration file", err)
	}

	return &cfg, nil
}
