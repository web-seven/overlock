package resources

import (
	"context"
	"strings"

	"github.com/charmbracelet/log"
	v1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	"github.com/kndpio/kndp/internal/kube"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
)

func GetXResources(ctx context.Context, dynamicClient *dynamic.DynamicClient, logger *log.Logger) []unstructured.Unstructured {

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

		xrList, err := kube.GetKubeResources(kube.ResourceParams{
			Dynamic:   dynamicClient,
			Ctx:       ctx,
			Group:     paramsXRs.Spec.Group,
			Version:   paramsXRs.Spec.Versions[0].Name,
			Resource:  paramsXRs.Spec.Names.Plural,
			Namespace: "",
		})

		if err != nil {
			logger.Error(err)
		}

		XRs = append(XRs, xrList...)
	}

	return XRs
}

func ExtractLabels(labels map[string]string) string {
	var sb strings.Builder
	for k, v := range labels {
		sb.WriteString(k)
		sb.WriteString(": ")
		sb.WriteString(v)
		sb.WriteString(", ")
	}
	return sb.String()
}
