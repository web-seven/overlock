package configuration

import (
	"context"
	"time"

	condition "github.com/crossplane/crossplane-runtime/apis/common/v1"

	configuration "github.com/crossplane/crossplane/apis/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/web-seven/overlock/internal/image"
	"github.com/web-seven/overlock/internal/kube"
	"github.com/web-seven/overlock/internal/packages"
	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

const (
	apiGroup   = "pkg.crossplane.io"
	apiVersion = "v1"
	apiPlural  = "configurations"
)

type Configuration struct {
	Name  string
	Image image.Image
	packages.Package
}

func New(name string) *Configuration {
	return &Configuration{
		Name:  name,
		Image: image.Image{Image: empty.Image},
	}
}

func CheckHealthStatus(status []condition.Condition) bool {
	healthStatus := false
	for _, condition := range status {
		if condition.Type == "Healthy" && condition.Status == "True" {
			healthStatus = true
		}
	}
	return healthStatus
}

func GetConfiguration(ctx context.Context, logger *zap.SugaredLogger, sourceDynamicClient dynamic.Interface, paramsConfiguration kube.ResourceParams) ([]unstructured.Unstructured, error) {

	configurations, err := kube.GetKubeResources(paramsConfiguration)
	if err != nil {
		return nil, err
	}

	return configurations, nil
}

func MoveConfigurations(ctx context.Context, logger *zap.SugaredLogger, destClientset dynamic.Interface, configurations []unstructured.Unstructured, paramsConfiguration kube.ResourceParams) error {
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
				return err
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
					logger.Infof("Failed to convert item %s: %v\n", conf.GetName(), err)
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
		logger.Warn("Configuration resources not found")
	}

	return nil
}

func ResourceId() schema.GroupVersionResource {
	return schema.GroupVersionResource{
		Group:    apiGroup,
		Version:  apiVersion,
		Resource: apiPlural,
	}
}
