package environment

import (
	"context"
	"log"
	"net/url"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kndpio/kndp/internal/install/helm"
	"github.com/kndpio/kndp/internal/kube"
	"github.com/pterm/pterm"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime/pkg/client/config"
)

type moveCmd struct {
	Dest string `arg:"" required:"" help:"Name destination context of environment."`
}

func (c *moveCmd) Run(ctx context.Context, p pterm.TextPrinter, config *rest.Config, dynamicClient *dynamic.DynamicClient) error {
	log.Println("Moving Kubernetes resources to the destination cluster...")
	params := kube.ResourceParams{
		Dynamic:   dynamicClient,
		Ctx:       ctx,
		Group:     "pkg.crossplane.io",
		Version:   "v1",
		Resource:  "configurations",
		Namespace: "",
	}

	// Retrieve resources from the source cluster
	items, err := kube.GetKubeResources(params)
	if err != nil {
		log.Println(err)
	}

	// Create a Kubernetes client for the destination cluster
	destConfig, err := ctrl.GetConfigWithContext(c.Dest)
	if err != nil {
		log.Println(err)
	}

	destClientset, err := dynamic.NewForConfig(destConfig)
	if err != nil {
		log.Println(err)
	}

	chartName := "crossplane"
	repoURL, err := url.Parse("https://charts.crossplane.io/stable")
	if err != nil {
		log.Println(err)
	}

	setWait := helm.InstallerModifierFn(helm.Wait())
	installer, err := helm.NewManager(destConfig, chartName, repoURL, setWait)
	if err != nil {
		log.Println(err)
	}
	installer.Install("", nil)

	// Create or update resources in the destination cluster
	for _, item := range items {
		item.SetResourceVersion("")
		resourceId := schema.GroupVersionResource{
			Group:    params.Group,
			Version:  params.Version,
			Resource: params.Resource,
		}
		_, err := destClientset.Resource(resourceId).Namespace(params.Namespace).Create(ctx, &item, metav1.CreateOptions{})
		if err != nil {
			log.Println(err)
		} else {
			log.Println("Successfully moved Kubernetes resources to the destination cluster.")
		}
	}

	return nil
}
