package environment

import (
	"context"
	"os/exec"
	"strings"
	"time"

	"github.com/charmbracelet/log"
	condition "github.com/crossplane/crossplane-runtime/apis/common/v1"
	v1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	configuration "github.com/crossplane/crossplane/apis/pkg/v1"
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

func checkHealthStatus(status []condition.Condition) bool {
	healthStatus := false
	for _, condition := range status {
		if condition.Type == "Healthy" && condition.Status == "True" {
			healthStatus = true
		}
	}
	return healthStatus
}

func (c *moveCmd) Run(ctx context.Context, logger *log.Logger) error {
	// Create a Kubernetes client for the source cluster
	sourceConfig, err := ctrl.GetConfigWithContext(c.Source)
	if err != nil {
		logger.Error(err)
		return nil
	}
	sourceDynamicClient, err := dynamic.NewForConfig(sourceConfig)
	if err != nil {
		logger.Error(err)
		return nil
	}

	paramsConfiguration := kube.ResourceParams{
		Dynamic:    sourceDynamicClient,
		Ctx:        ctx,
		Group:      "pkg.crossplane.io",
		Version:    "v1",
		Resource:   "configurations",
		Namespace:  "",
		ListOption: metav1.ListOptions{},
	}

	configurations, err := kube.GetKubeResources(paramsConfiguration)

	if err != nil {
		logger.Error(err)
		return nil
	}

	// Create a Kubernetes client for the destination cluster

	destConfig, err := ctrl.GetConfigWithContext(c.Destination)
	if err != nil {
		logger.Error(err)
		return nil
	}

	destClientset, err := dynamic.NewForConfig(destConfig)
	if err != nil {
		logger.Error(err)
		return nil
	}

	//Apply configurations

	if len(configurations) > 0 {
		logger.Info("Moving Kubernetes resources to the destination cluster, please wait ...")

		for _, item := range configurations {
			item.SetResourceVersion("")
			resourceId := schema.GroupVersionResource{
				Group:    paramsConfiguration.Group,
				Version:  paramsConfiguration.Version,
				Resource: paramsConfiguration.Resource,
			}
			_, err := destClientset.Resource(resourceId).Namespace(paramsConfiguration.Namespace).Create(ctx, &item, metav1.CreateOptions{})
			if err != nil {
				logger.Fatal(err)
			} else {
				logger.Info("Configuration created successfully ", item.GetName())
			}

		}

		//Check configuration health status

		for {
			outerLoopBreak := false
			destConf, _ := kube.GetKubeResources(kube.ResourceParams{
				Dynamic:    destClientset,
				Ctx:        ctx,
				Group:      "pkg.crossplane.io",
				Version:    "v1",
				Resource:   "configurations",
				Namespace:  "",
				ListOption: metav1.ListOptions{},
			})
			for _, conf := range destConf {
				var paramsConf configuration.Configuration
				if err := runtime.DefaultUnstructuredConverter.FromUnstructured(conf.UnstructuredContent(), &paramsConf); err != nil {
					logger.Printf("Failed to convert item %s: %v\n", conf.GetName(), err)
					continue
				}
				status := paramsConf.Status.Conditions
				healthStatus := checkHealthStatus(status)
				if healthStatus {
					outerLoopBreak = true
					break
				}
				time.Sleep(3 * time.Second)
			}
			if outerLoopBreak {
				break
			}
			time.Sleep(3 * time.Second)
		}

		//Get composite resources from XRDs definition and apply them

		XRDs, err := kube.GetKubeResources(kube.ResourceParams{
			Dynamic:    sourceDynamicClient,
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
		for _, xrd := range XRDs {
			var paramsXRs v1.CompositeResourceDefinition
			if err := runtime.DefaultUnstructuredConverter.FromUnstructured(xrd.UnstructuredContent(), &paramsXRs); err != nil {
				logger.Printf("Failed to convert item %s: %v\n", xrd.GetName(), err)
				return nil
			}

			XRs, err := kube.GetKubeResources(kube.ResourceParams{
				Dynamic:   sourceDynamicClient,
				Ctx:       ctx,
				Group:     paramsXRs.Spec.Group,
				Version:   paramsXRs.Spec.Versions[0].Name,
				Resource:  paramsXRs.Spec.Names.Plural,
				Namespace: "",
				ListOption: metav1.ListOptions{
					LabelSelector: "app.kubernetes.io/managed-by=kndp",
				},
			})
			if err != nil {
				logger.Error(err)
				return nil
			}

			for _, xr := range XRs {
				xr.SetResourceVersion("")
				resourceId := schema.GroupVersionResource{
					Group:    paramsXRs.Spec.Group,
					Version:  paramsXRs.Spec.Versions[0].Name,
					Resource: paramsXRs.Spec.Names.Plural,
				}
				_, err = destClientset.Resource(resourceId).Namespace("").Create(ctx, &xr, metav1.CreateOptions{})
				if err != nil {
					logger.Warn(err)
					return nil
				} else {
					logger.Info("Resource created successfully ", xr.GetName())
				}
			}
		}
	} else {
		logger.Fatal("No Kubernetes resources to move !")
	}

	if err == nil {
		cmd := exec.Command("kind", "delete", "cluster", "--name", strings.TrimPrefix(c.Source, "kind-"))
		cmd.Run()
		logger.Info("Successfully moved Kubernetes resources to the destination cluster.")
	}

	return nil
}
