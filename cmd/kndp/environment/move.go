package environment

import (
	"context"
	"os/exec"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kndpio/kndp/internal/environment"
	"github.com/kndpio/kndp/internal/kube"

	"github.com/charmbracelet/log"
)

type moveCmd struct {
	Source      string `arg:"" required:"" help:"Name source of environment."`
	Destination string `arg:"" required:"" help:"Name destination of environment."`
}

func (c *moveCmd) Run(ctx context.Context, logger *log.Logger) error {

	// Create a Kubernetes client
	sourceContext, err := kube.CreateKubernetesClients(ctx, logger, c.Source)
	if err != nil {
		logger.Error(err)
		return nil
	}
	destinationContext, err := kube.CreateKubernetesClients(ctx, logger, c.Destination)
	if err != nil {
		logger.Error(err)
		return nil
	}

	//Apply configurations
	paramsConfiguration := kube.ResourceParams{
		Dynamic:    sourceContext,
		Ctx:        ctx,
		Group:      "pkg.crossplane.io",
		Version:    "v1",
		Resource:   "configurations",
		Namespace:  "",
		ListOption: metav1.ListOptions{},
	}
	configurations, err := environment.GetConfigurations(ctx, logger, sourceContext, paramsConfiguration)
	if err != nil {
		logger.Error(err)
		return nil
	}

	//Check configuration health status and move configurations to destination cluster
	err = environment.MoveConfigurations(ctx, logger, destinationContext, configurations, paramsConfiguration)
	if err != nil {
		logger.Error(err)
		return nil
	}

	//Get composite resources from XRDs definition and apply them
	XRDs, err := kube.GetKubeResources(kube.ResourceParams{
		Dynamic:    sourceContext,
		Ctx:        ctx,
		Group:      "apiextensions.crossplane.io",
		Version:    "v1",
		Resource:   "compositeresourcedefinitions",
		Namespace:  "",
		ListOption: metav1.ListOptions{},
	})
	if err != nil {
		logger.Fatal(err)
		return nil
	}
	err = environment.MoveCompositeResources(ctx, logger, sourceContext, destinationContext, XRDs)

	//Delete source cluster after all resources are successfully created in destination cluster
	if err == nil {
		cmd := exec.Command("kind", "delete", "cluster", "--name", strings.TrimPrefix(c.Source, "kind-"))
		cmd.Run()
		logger.Info("Successfully moved Kubernetes resources to the destination cluster.")
	}

	return nil
}
