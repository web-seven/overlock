package environment

import (
	"context"
	"errors"
	"os"

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
	MountPath                 string   `optional:"" help:"Path for mount to /storage host directory. By default no mounts."`
	ContainerPath             string   `optional:"" help:"Container mount path for the volume." default:"/storage"`
	Providers                 []string `optional:"" help:"List of providers to apply to the environment."`
	Configurations            []string `optional:"" help:"List of configurations to apply to the environment."`
	Functions                 []string `optional:"" help:"List of functions to apply to the environment."`
	CreateAdminServiceAccount bool     `optional:"" help:"Create admin service account with cluster-admin privileges."`
	AdminServiceAccountName   string   `optional:"" help:"Name for the admin service account. Only relevant when create-admin-service-account is enabled. Defaults to 'overlock-admin' if not specified."`
	Cpu                       string   `optional:"" help:"CPU limit for k3s-docker containers (e.g., 2, 0.5, 50%)." default:""`
	MaxReconcileRate          int      `optional:"" help:"Maximum number of reconciliations per second for Crossplane (e.g., 1). Defaults to Crossplane's built-in default if not set." default:"0"`
}

func (c *createCmd) Run(ctx context.Context, logger *zap.SugaredLogger) error {
	configPath := c.Config
	userProvidedConfig := c.Config != ""

	if !userProvidedConfig {
		configPath = "./overlock.yaml"
	}
	cfg, err := loadConfig(configPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			if userProvidedConfig {
				logger.Errorf("Configuration file not found at specified path: %s", configPath)
				return err
			}
		} else {
			logger.Infof("Failed to parse the configuration file at '%s'.", configPath)
			logger.Info("For guidance on the correct structure, refer to the documentation: https://docs.overlock.network/environment/cfg-file")
			return nil
		}
	}

	if cfg != nil {
		if err := mergo.MergeWithOverwrite(&c.createOptions, cfg, mergo.WithOverride); err != nil {
			logger.Errorf("Failed to merge configuration: %v", err)
			return overlockerrors.NewInvalidConfigErrorWithCause("", "", "failed to merge configuration options", err)
		}
	}

	return environment.
		New(c.Engine, c.Name).
		WithHttpPort(c.HttpPort).
		WithHttpsPort(c.HttpsPort).
		WithContext(c.Context).
		WithMountPath(c.MountPath).
		WithContainerPath(c.ContainerPath).
		WithEngineConfig(c.EngineConfig).
		WithProviders(c.Providers).
		WithConfigurations(c.Configurations).
		WithFunctions(c.Functions).
		WithAdminServiceAccount(c.CreateAdminServiceAccount, c.AdminServiceAccountName).
		WithCpu(c.Cpu).
		WithMaxReconcileRate(c.MaxReconcileRate).
		Create(ctx, logger)
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
