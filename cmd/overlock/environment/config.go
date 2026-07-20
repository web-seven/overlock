package environment

import (
	"errors"
	"os"
	"path/filepath"

	"dario.cat/mergo"
	"go.uber.org/zap"
	yaml "gopkg.in/yaml.v2"

	overlockerrors "github.com/web-seven/overlock/pkg/errors"
)

const cfgFileDocsHint = "For guidance on the correct structure, refer to the documentation: https://docs.overlock.network/environment/cfg-file"

// loadAndMergeConfig loads the Overlock configuration file(s) and merges them
// into the command options. When --config is set, only that file is used;
// otherwise the layered defaults (overlock.yaml, .overlock.yaml,
// .overlock.*.yaml) are merged in order.
//
// The returned stop flag is true when a malformed configuration file was found
// and the command should abort gracefully without an error.
func (c *createCmd) loadAndMergeConfig(logger *zap.SugaredLogger) (stop bool, err error) {
	if c.Config != "" {
		cfg, err := loadConfig(c.Config)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				logger.Errorf("Configuration file not found at specified path: %s", c.Config)
				return false, err
			}
			logger.Infof("Failed to parse the configuration file at '%s'.", c.Config)
			logger.Info(cfgFileDocsHint)
			return true, nil
		}
		return false, c.mergeConfig(cfg, logger)
	}

	paths, err := layeredConfigPaths()
	if err != nil {
		return false, err
	}
	for _, path := range paths {
		cfg, err := loadConfig(path)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			logger.Infof("Failed to parse the configuration file at '%s'.", path)
			logger.Info(cfgFileDocsHint)
			return true, nil
		}
		if err := c.mergeConfig(cfg, logger); err != nil {
			return false, err
		}
	}
	return false, nil
}

// mergeConfig merges cfg into the command options, overwriting existing values.
func (c *createCmd) mergeConfig(cfg *createOptions, logger *zap.SugaredLogger) error {
	if err := mergo.MergeWithOverwrite(&c.createOptions, cfg, mergo.WithOverride); err != nil {
		logger.Errorf("Failed to merge configuration: %v", err)
		return overlockerrors.NewInvalidConfigErrorWithCause("", "", "failed to merge configuration options", err)
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
