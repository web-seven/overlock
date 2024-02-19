package environment

import (
	"context"
	"fmt"
	"log"
	"os/exec"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	ctrl "sigs.k8s.io/controller-runtime/pkg/client/config"

	v1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	"github.com/kndpio/kndp/internal/kube"
	"github.com/pterm/pterm"
)

type moveCmd struct {
	Source      string `arg:"" required:"" help:"Name source of environment."`
	Destination string `arg:"" required:"" help:"Name destination of environment."`
}

func getAppliedConfigurations(paramsList []kube.ResourceParams) []unstructured.Unstructured {
	var items []unstructured.Unstructured
	for _, params := range paramsList {
		items, err := kube.GetKubeResources(params)
		if err != nil {
			log.Println(err)
			continue
		}
		return items
	}
	return items
}

func (c *moveCmd) Run(ctx context.Context, p pterm.TextPrinter) error {
	log.Println("Moving Kubernetes resources to the destination cluster...")
	// Create a Kubernetes client for the source cluster
	sourceConfig, err := ctrl.GetConfigWithContext(c.Source)
	if err != nil {
		log.Println(err)
		return err
	}
	sourceDynamicClient, err := dynamic.NewForConfig(sourceConfig)
	if err != nil {
		log.Println(err)
		return err
	}

	paramsConfiguration := []kube.ResourceParams{
		{
			Dynamic:   sourceDynamicClient,
			Ctx:       ctx,
			Group:     "pkg.crossplane.io",
			Version:   "v1",
			Resource:  "configurations",
			Namespace: "",
		},
	}

	configurations := getAppliedConfigurations(paramsConfiguration)

	if err != nil {
		return err
	}
	// Create a Kubernetes client for the destination cluster
	destConfig, err := ctrl.GetConfigWithContext(c.Destination)
	if err != nil {
		log.Println(err)
		return err
	}

	destClientset, err := dynamic.NewForConfig(destConfig)
	if err != nil {
		log.Println(err)
		return err
	}

	fmt.Println("Creating resources in destination cluster, please wait ...")
	//Apply configurations
	for _, item := range configurations {
		item.SetResourceVersion("")
		var resourceId schema.GroupVersionResource
		for _, params := range paramsConfiguration {
			resourceId = schema.GroupVersionResource{
				Group:    params.Group,
				Version:  params.Version,
				Resource: params.Resource,
			}
			_, err := destClientset.Resource(resourceId).Namespace(params.Namespace).Create(ctx, &item, metav1.CreateOptions{})
			if err != nil {
				fmt.Println(err)
			} else {
				fmt.Println("Resource", item.GetName(), "created successfully")
			}
		}
	}
	//Get composite resources xrds definition and apply them

	paramsXRDs := []kube.ResourceParams{
		{
			Dynamic:   sourceDynamicClient,
			Ctx:       ctx,
			Group:     "apiextensions.crossplane.io",
			Version:   "v1",
			Resource:  "compositeresourcedefinitions",
			Namespace: "",
		},
	}
	XRDs := getAppliedConfigurations(paramsXRDs)

	var paramsXR []kube.ResourceParams
	for _, item := range XRDs {
		var paramsXRs v1.CompositeResourceDefinition
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(item.UnstructuredContent(), &paramsXRs); err != nil {
			fmt.Printf("Failed to convert item %s: %v\n", item.GetName(), err)
			continue
		}

		paramsXR = append(paramsXR, kube.ResourceParams{
			Dynamic:   sourceDynamicClient,
			Ctx:       ctx,
			Group:     paramsXRs.Spec.Group,
			Version:   paramsXRs.Spec.Versions[0].Name,
			Resource:  paramsXRs.Spec.Names.Plural,
			Namespace: "",
		})
	}

	XRs := getAppliedConfigurations(paramsXR)
	cmd := exec.Command("kind", "delete", "cluster", "--name", strings.TrimPrefix(c.Source, "kind-"))
	cmd.Run()

	for _, item := range XRs {
		item.SetResourceVersion("")
		labels := item.GetLabels()
		var resourceId schema.GroupVersionResource
		if labels != nil && labels["kndp"] == "resources" {
			for _, params := range paramsXR {
				resourceId = schema.GroupVersionResource{
					Group:    params.Group,
					Version:  params.Version,
					Resource: params.Resource,
				}
				for {
					_, err := destClientset.Resource(resourceId).Namespace(params.Namespace).Create(ctx, &item, metav1.CreateOptions{})
					fmt.Println(err)
					if err == nil {
						fmt.Println("Resource", item.GetName(), "created successfully")
						break
					}
					time.Sleep(5 * time.Second)
				}
			}
		} else {
			fmt.Println("No resource to create...")
		}
	}

	log.Println("Successfully moved Kubernetes resources to the destination cluster.")

	return nil
}
