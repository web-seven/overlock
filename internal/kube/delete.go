package kube

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func DeleteKubeResources(ctx context.Context, p ResourceParams, resourceName string) error {
	resourceID := schema.GroupVersionResource{
		Group:    p.Group,
		Version:  p.Version,
		Resource: p.Resource,
	}

	err := p.Dynamic.Resource(resourceID).Namespace(p.Namespace).Delete(ctx, resourceName, metav1.DeleteOptions{})
	if err != nil {
		return err
	}

	return err
}
