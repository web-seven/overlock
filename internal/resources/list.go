package resources

import (
	"context"

	v1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	"github.com/kndpio/kndp/internal/kube"
	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
)

func GetXResources(ctx context.Context, dynamicClient *dynamic.DynamicClient, logger *zap.SugaredLogger) []unstructured.Unstructured {

	paramsXRDs := kube.ResourceParams{
		Dynamic:   dynamicClient,
		Ctx:       ctx,
		Group:     "apiextensions.crossplane.io",
		Version:   "v1",
		Resource:  "compositeresourcedefinitions",
		Namespace: "",
	}
	XRDs, err := kube.GetKubeResources(paramsXRDs)
	if err != nil {
		logger.Error(err)
	}
	var XRs []unstructured.Unstructured

	for _, xrd := range XRDs {
		var paramsXRs v1.CompositeResourceDefinition
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(xrd.UnstructuredContent(), &paramsXRs)
		if err != nil {
			logger.Error(err)
		}
		for _, version := range paramsXRs.Spec.Versions {
			xrList, err := kube.GetKubeResources(kube.ResourceParams{
				Dynamic:   dynamicClient,
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
			}

			XRs = append(XRs, xrList...)
		}
	}

	return XRs
}
