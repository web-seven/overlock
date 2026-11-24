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

type lsCmd struct {
	Registry   string `arg:"" help:"Name of the registry to list tags from."`
	Repository string `arg:"" help:"Repository to list tags of."`
	Output     string `short:"o" enum:"text,json" default:"text" help:"Output format: text or json."`
	Sort       string `enum:"asc,desc" default:"asc" help:"Sort tags by semver precedence."`
	Limit      int    `help:"Limit the number of tags returned. 0 means no limit."`
	Latest     bool   `help:"Return only the most recent tag (shorthand for --sort=desc --limit=1)."`
}

func (c *lsCmd) Run(ctx context.Context, client *kubernetes.Clientset, config *rest.Config, logger *zap.SugaredLogger) error {
	reg, err := registry.GetRegistry(ctx, client, c.Registry)
	if err != nil {
		logger.Error(err)
		os.Exit(exitUnreachable)
	}

	tags, err := reg.ListTags(ctx, c.Repository, config, logger)
	if err != nil {
		if overlockerrors.IsPackageNotFoundError(err) {
			logger.Errorf("Repository '%s' not found in registry '%s'.", c.Repository, c.Registry)
			os.Exit(exitNotFound)
		}
		logger.Error(err)
		os.Exit(exitUnreachable)
	}

	if len(tags) == 0 {
		logger.Infof("Repository '%s' has no tags in registry '%s'.", c.Repository, c.Registry)
		os.Exit(exitNotFound)
	}

	sortTags(tags, c.Sort == "desc" || c.Latest)

	limit := c.Limit
	if c.Latest {
		limit = 1
	}
	if limit > 0 && limit < len(tags) {
		tags = tags[:limit]
	}

	return printList(c.Output, "TAG", tags)
}
