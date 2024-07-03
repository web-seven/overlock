package provider

import (
	"context"

	"github.com/charmbracelet/log"
	provider "github.com/crossplane/crossplane/apis/pkg/v1"
	"github.com/kndpio/kndp/internal/kube"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
)

// DeleteProvider deletes a crossplane provider from current environment
func DeleteProvider(ctx context.Context, dynamicClient dynamic.Interface, url string, logger *log.Logger) error {
	var providerParams = kube.ResourceParams{
		Dynamic:    dynamicClient,
		Ctx:        ctx,
		Group:      "pkg.crossplane.io",
		Version:    "v1",
		Resource:   "providers",
		Namespace:  "",
		ListOption: metav1.ListOptions{},
	}
	destConf, _ := kube.GetKubeResources(providerParams)
	logger.Debug("Getting providers from environment")
	for _, conf := range destConf {
		var provider provider.Provider
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(conf.UnstructuredContent(), &provider); err != nil {
			logger.Printf("Failed to convert item %s: %v\n", conf.GetName(), err)
			continue
		}
		if provider.Spec.Package == url {
			err := kube.DeleteKubeResources(ctx, providerParams, provider.Name)
			if err != nil {
				return err
			}
			logger.Infof("Provider %s deleted succesffuly!", url)
			return nil
		}
	}
	logger.Errorf("Provider %s does not exist in environment", url)
	return nil
}
