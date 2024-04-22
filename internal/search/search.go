package search

import (
	"context"
	"net/url"
	"strings"

	"github.com/charmbracelet/log"

	"github.com/kndpio/kndp/internal/github"
	"github.com/kndpio/kndp/internal/registry"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func SearchCmd(ctx context.Context, client *kubernetes.Clientset, config *rest.Config, query string, versions bool, logger *log.Logger) error {
	registries, err := registry.Registries(ctx, client)
	if err != nil {
		logger.Error("Cannot get registries")
		return err
	}

	for _, r := range registries {
		registryUrl := r.Annotations["kndp-registry-server-url"]
		u, _ := url.Parse(registryUrl)
		org := strings.TrimPrefix(u.Path, "/")
		// Switch statement to handle different registry types
		switch {
		case strings.Contains(registryUrl, "ghcr.io"):
			_, _, err := github.GetGithubPackages(ctx, query, versions, r, registryUrl, org, logger)
			if err != nil {
				return err
			}

		default:
			logger.Errorf("Registry %s is not supported", registryUrl)
		}

	}
	return nil
}
