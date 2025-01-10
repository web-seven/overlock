package function

import (
	"context"

	condition "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"

	"github.com/web-seven/overlock/internal/image"
	"github.com/web-seven/overlock/internal/kube"
	"github.com/web-seven/overlock/internal/packages"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

const (
	apiGroup   = "pkg.crossplane.io"
	apiVersion = "v1"
	apiPlural  = "functions"
)

type Function struct {
	Name  string
	Image image.Image
	packages.Package
}

func New(name string) *Function {
	return &Function{
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

func GetFunction(ctx context.Context, logger *zap.SugaredLogger, sourceDynamicClient dynamic.Interface, paramsFunction kube.ResourceParams) ([]unstructured.Unstructured, error) {

	functions, err := kube.GetKubeResources(paramsFunction)
	if err != nil {
		return nil, err
	}

	return functions, nil
}

func ResourceId() schema.GroupVersionResource {
	return schema.GroupVersionResource{
		Group:    apiGroup,
		Version:  apiVersion,
		Resource: apiPlural,
	}
}
