package xrd

import (
	"context"

	"github.com/web-seven/overlock/internal/kube"

	"k8s.io/apimachinery/pkg/runtime"

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
)

type ObjectRef struct {
	Name string `json:"name"`
	Kind string `json:"kind"`
}
type ConfigurationRevision struct {
	Spec struct {
		Image string `json:"image"`
	} `json:"spec"`
	Status struct {
		ObjectRefs []ObjectRef `json:"objectRefs"`
	}
}

func GetXRDs(Link string, ctx context.Context, config *rest.Config, dynamicClient *dynamic.DynamicClient) []string {
	var xrds []string
	var params = kube.ResourceParams{
		Dynamic:   dynamicClient,
		Ctx:       ctx,
		Group:     "pkg.crossplane.io",
		Version:   "v1",
		Resource:  "configurationrevisions",
		Namespace: "",
	}

	items, _ := kube.GetKubeResources(params)
	for _, item := range items {
		configRev := &ConfigurationRevision{}
		runtime.DefaultUnstructuredConverter.FromUnstructured(item.Object, configRev)
		if configRev.Spec.Image == Link {
			for _, objectRef := range configRev.Status.ObjectRefs {
				if objectRef.Kind == "CompositeResourceDefinition" {
					xrds = append(xrds, objectRef.Name)
				}
			}
		}
	}
	return xrds
}
