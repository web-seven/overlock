package environment

import (
	"context"
	"os/exec"
	"strings"
	"time"

	"github.com/charmbracelet/log"
	v1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	"github.com/kndpio/kndp/internal/kube"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	ctrl "sigs.k8s.io/controller-runtime/pkg/client/config"
)

type moveCmd struct {
	Source      string `arg:"" required:"" help:"Name source of environment."`
	Destination string `arg:"" required:"" help:"Name destination of environment."`
}

func (c *moveCmd) Run(ctx context.Context, logger *log.Logger) error {
	logger.Info("Moving Kubernetes resources to the destination cluster ...")
	// Create a Kubernetes client for the source cluster
	sourceConfig, err := ctrl.GetConfigWithContext(c.Source)
	if err != nil {
		logger.Error(err)
		return err
	}
	sourceDynamicClient, err := dynamic.NewForConfig(sourceConfig)
	if err != nil {
		logger.Error(err)
		return err
	}

	paramsConfiguration := kube.ResourceParams{
		Dynamic:   sourceDynamicClient,
		Ctx:       ctx,
		Group:     "pkg.crossplane.io",
		Version:   "v1",
		Resource:  "configurations",
		Namespace: "",
	}

	configurations, err := kube.GetKubeResources(paramsConfiguration)

	if err != nil {
		return err
	}

	// Create a Kubernetes client for the destination cluster

	destConfig, err := ctrl.GetConfigWithContext(c.Destination)
	if err != nil {
		logger.Error(err)
		return err
	}

	destClientset, err := dynamic.NewForConfig(destConfig)
	if err != nil {
		logger.Error(err)
		return err
	}
	logger.Info("Creating resources in destination cluster, please wait ...")

	//Apply configurations

	for _, item := range configurations {
		item.SetResourceVersion("")
		resourceId := schema.GroupVersionResource{
			Group:    paramsConfiguration.Group,
			Version:  paramsConfiguration.Version,
			Resource: paramsConfiguration.Resource,
		}
		_, err := destClientset.Resource(resourceId).Namespace(paramsConfiguration.Namespace).Create(ctx, &item, metav1.CreateOptions{})
		if err != nil {
			logger.Error(err)
		} else {
			logger.Info("Resource", item.GetName(), "created successfully")
		}

	}

	//Get composite resources xrds definition and apply them

	paramsXRDs := kube.ResourceParams{
		Dynamic:   sourceDynamicClient,
		Ctx:       ctx,
		Group:     "apiextensions.crossplane.io",
		Version:   "v1",
		Resource:  "compositeresourcedefinitions",
		Namespace: "",
	}
	XRDs, err := kube.GetKubeResources(paramsXRDs)
	if err != nil {
		logger.Error(err)
	}
	for _, xrd := range XRDs {
		var paramsXRs v1.CompositeResourceDefinition
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(xrd.UnstructuredContent(), &paramsXRs); err != nil {
			logger.Printf("Failed to convert item %s: %v\n", xrd.GetName(), err)
			continue
		}

		XRs, err := kube.GetKubeResources(kube.ResourceParams{
			Dynamic:   sourceDynamicClient,
			Ctx:       ctx,
			Group:     paramsXRs.Spec.Group,
			Version:   paramsXRs.Spec.Versions[0].Name,
			Resource:  paramsXRs.Spec.Names.Plural,
			Namespace: "",
		})
		if err != nil {
			logger.Error(err)
		}

		for _, xr := range XRs {
			xr.SetResourceVersion("")
			labels := xr.GetLabels()
			if labels != nil && labels["app.kubernetes.io/managed-by"] == "kndp" {
				resourceId := schema.GroupVersionResource{
					Group:    paramsXRs.Spec.Group,
					Version:  paramsXRs.Spec.Versions[0].Name,
					Resource: paramsXRs.Spec.Names.Plural,
				}
				for {
					_, err := destClientset.Resource(resourceId).Namespace("").Create(ctx, &xr, metav1.CreateOptions{})
					if err == nil {
						logger.Info("Resource", xr.GetName(), "created successfully")
						break
					}
					time.Sleep(5 * time.Second)
				}
			} else {
				logger.Printf("No resource to create from: %s\n", xrd.GetName())
			}
		}
	}

	cmd := exec.Command("kind", "delete", "cluster", "--name", strings.TrimPrefix(c.Source, "kind-"))
	cmd.Run()

	logger.Info("Successfully moved Kubernetes resources to the destination cluster.")

	return nil
}
