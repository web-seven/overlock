package environment

import (
	"context"
	"os/exec"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kndpio/kndp/internal/configuration"
	"github.com/kndpio/kndp/internal/kube"
	"github.com/kndpio/kndp/internal/resources"

	"github.com/charmbracelet/log"
)

func MoveKndpResources(ctx context.Context, logger *log.Logger, source string, destination string) error {

	// Create a Kubernetes client
	sourceContext := kube.Context(ctx, logger, source)
	destinationContext := kube.Context(ctx, logger, destination)

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
	configurations, err := configuration.GetConfiguration(ctx, logger, sourceContext, paramsConfiguration)
	if err != nil {
		logger.Error(err)
		return nil
	}

	//Check configuration health status and move configurations to destination cluster
	err = configuration.MoveConfigurations(ctx, logger, destinationContext, configurations, paramsConfiguration)
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
	err = resources.MoveCompositeResources(ctx, logger, sourceContext, destinationContext, XRDs)

	//Delete source cluster after all resources are successfully created in destination cluster
	if err == nil {
		cmd := exec.Command("kind", "delete", "cluster", "--name", strings.TrimPrefix(source, "kind-"))
		cmd.Run()
		logger.Info("Successfully moved Kubernetes resources to the destination cluster.")
	}

	return nil
}