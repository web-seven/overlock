package environment

import (
	"context"
	"time"

	condition "github.com/crossplane/crossplane-runtime/apis/common/v1"

	"github.com/charmbracelet/log"
	v1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	configuration "github.com/crossplane/crossplane/apis/pkg/v1"
	"github.com/kndpio/kndp/internal/kube"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

func CheckHealthStatus(status []condition.Condition) bool {
	healthStatus := false
	for _, condition := range status {
		if condition.Type == "Healthy" && condition.Status == "True" {
			healthStatus = true
		}
	}
	return healthStatus
}

func GetConfigurations(ctx context.Context, logger *log.Logger, sourceDynamicClient dynamic.Interface, paramsConfiguration kube.ResourceParams) ([]unstructured.Unstructured, error) {

	configurations, err := kube.GetKubeResources(paramsConfiguration)
	if err != nil {
		logger.Error(err)
		return nil, err
	}

	return configurations, nil
}

func MoveConfigurations(ctx context.Context, logger *log.Logger, destClientset dynamic.Interface, configurations []unstructured.Unstructured, paramsConfiguration kube.ResourceParams) error {
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
				logger.Infof("Configuration created successfully %s", item.GetName())

			}

		}

		//Check configuration health status
		configurationHealthy := false

		for !configurationHealthy {
			configurationHealthy = true
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
				condition := paramsConf.Status.Conditions
				healthStatus := CheckHealthStatus(condition)
				if !healthStatus {
					configurationHealthy = false
					break
				}
				time.Sleep(5 * time.Second)
			}
			if !configurationHealthy {
				time.Sleep(10 * time.Second)
			}
		}
	} else {
		logger.Fatal("No Kubernetes resources to move !")
	}

	return nil
}

func MoveCompositeResources(ctx context.Context, logger *log.Logger, sourceContext dynamic.Interface, destinationContext dynamic.Interface, XRDs []unstructured.Unstructured) error {
	for _, xrd := range XRDs {
		var paramsXRs v1.CompositeResourceDefinition
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(xrd.UnstructuredContent(), &paramsXRs); err != nil {
			logger.Printf("Failed to convert item %s: %v\n", xrd.GetName(), err)
			return nil
		}
		for _, version := range paramsXRs.Spec.Versions {
			XRs, err := kube.GetKubeResources(kube.ResourceParams{
				Dynamic:   sourceContext,
				Ctx:       ctx,
				Group:     paramsXRs.Spec.Group,
				Version:   version.Name,
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
					Version:  version.Name,
					Resource: paramsXRs.Spec.Names.Plural,
				}
				_, err = destinationContext.Resource(resourceId).Namespace("").Create(ctx, &xr, metav1.CreateOptions{})
				if err != nil {
					logger.Fatal(err)
					return nil
				} else {
					logger.Infof("Resource created successfully %s", xr.GetName())
				}
			}
		}
	}
	return nil
}
