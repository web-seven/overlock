package configuration

import (
	"context"

	"github.com/kndpio/kndp/internal/kube"

	crossv1 "github.com/crossplane/crossplane/apis/pkg/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
)

type Configuration struct {
	Name    string
	Package string
}

func ListConfigurations(ctx context.Context, config *rest.Config, dynamicClient *dynamic.DynamicClient) []Configuration {
	var configurationList []Configuration
	var params = kube.ResourceParams{
		Dynamic:   dynamicClient,
		Ctx:       ctx,
		Group:     "pkg.crossplane.io",
		Version:   "v1",
		Resource:  "configurations",
		Namespace: "",
	}
	var paramsXRs crossv1.Configuration
	items, _ := kube.GetKubeResources(params)
	for _, item := range items {
		runtime.DefaultUnstructuredConverter.FromUnstructured(item.UnstructuredContent(), &paramsXRs)
		configurationList = append(configurationList, Configuration{
			Name:    paramsXRs.Name,
			Package: paramsXRs.Spec.Package,
		})
	}

	return configurationList
}
