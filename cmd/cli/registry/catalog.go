package registry

import (
	"context"
	"os"

	"go.uber.org/zap"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	overlockerrors "github.com/web-seven/overlock/pkg/errors"
	"github.com/web-seven/overlock/pkg/registry"
)

type catalogCmd struct {
	Registry string `arg:"" help:"Name of the registry to list repositories from."`
	Output   string `short:"o" enum:"text,json" default:"text" help:"Output format: text or json."`
}

func (c *catalogCmd) Run(ctx context.Context, client *kubernetes.Clientset, config *rest.Config, logger *zap.SugaredLogger) error {
	reg, err := registry.GetRegistry(ctx, client, c.Registry)
	if err != nil {
		logger.Error(err)
		os.Exit(exitUnreachable)
	}

	repos, err := reg.Catalog(ctx, config, logger)
	if err != nil {
		if overlockerrors.IsPackageNotFoundError(err) {
			logger.Errorf("Registry '%s' has no repositories.", c.Registry)
			os.Exit(exitNotFound)
		}
		logger.Error(err)
		os.Exit(exitUnreachable)
	}

	if len(repos) == 0 {
		logger.Infof("Registry '%s' has no repositories.", c.Registry)
		os.Exit(exitNotFound)
	}

	return printList(c.Output, "REPOSITORY", repos)
}
