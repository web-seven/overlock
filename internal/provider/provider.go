package provider

import (
	"context"

	regv1 "github.com/google/go-containerregistry/pkg/v1"

	"github.com/charmbracelet/log"
	provider "github.com/crossplane/crossplane/apis/pkg/v1"
	"github.com/kndpio/kndp/internal/kube"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
)

type Provider struct {
	Name  string
	Image regv1.Image
}

// New Provider entity
func New(name string) *Provider {
	return &Provider{
		Name: name,
	}
}

func ListProviders(ctx context.Context, dynamicClient dynamic.Interface, logger *log.Logger) []provider.Provider {
	destConf, _ := kube.GetKubeResources(kube.ResourceParams{
		Dynamic:    dynamicClient,
		Ctx:        ctx,
		Group:      "pkg.crossplane.io",
		Version:    "v1",
		Resource:   "providers",
		Namespace:  "",
		ListOption: metav1.ListOptions{},
	})
	var providers []provider.Provider
	for _, conf := range destConf {
		var paramsProvider provider.Provider
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(conf.UnstructuredContent(), &paramsProvider); err != nil {
			logger.Printf("Failed to convert item %s: %v\n", conf.GetName(), err)
			continue
		}
		providers = append(providers, paramsProvider)
	}
	return providers
}
