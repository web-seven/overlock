package resources

import (
	"context"
	"strings"

	"github.com/charmbracelet/log"
	v1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	"github.com/kndpio/kndp/internal/kube"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

func ApplyResources(ctx context.Context, client *dynamic.DynamicClient, logger *log.Logger, file string) error {
	resources, err := transformToUnstructured(file, logger)

	if err != nil {
		return err
	}
	for _, resource := range resources {
		apiAndVersion := strings.Split(resource.GetAPIVersion(), "/")

		resourceId := schema.GroupVersionResource{
			Group:    apiAndVersion[0],
			Version:  apiAndVersion[1],
			Resource: strings.ToLower(resource.GetKind()) + "s",
		}
		res, err := client.Resource(resourceId).Create(ctx, &resource, metav1.CreateOptions{})

		if err != nil {
			return err
		} else {
			logger.Infof("Resource %s from %s successfully applied", res.GetName(), res.GetAPIVersion())
		}
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
