package kube

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"

	"k8s.io/apimachinery/pkg/runtime/schema"
)

type ResourceParams struct {
	Dynamic    dynamic.Interface
	Ctx        context.Context
	Group      string
	Version    string
	Resource   string
	Namespace  string
	ListOption metav1.ListOptions
}

func GetKubeResources(p ResourceParams) ([]unstructured.Unstructured, error) {
	resourceId := schema.GroupVersionResource{
		Group:    p.Group,
		Version:  p.Version,
		Resource: p.Resource,
	}
	list, err := p.Dynamic.Resource(resourceId).Namespace(p.Namespace).List(p.Ctx, p.ListOption)

	if err != nil {
		return nil, err
	}

	return list.Items, err
}
