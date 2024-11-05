package function

import (
	"context"

	crossv1 "github.com/crossplane/crossplane/apis/pkg/v1beta1"
	"github.com/web-seven/overlock/internal/kube"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
)

func GetFunctions(ctx context.Context, dynamicClient dynamic.Interface) []crossv1.Function {
	var params = kube.ResourceParams{
		Dynamic:   dynamicClient,
		Ctx:       ctx,
		Group:     "pkg.crossplane.io",
		Version:   "v1beta1",
		Resource:  "functions",
		Namespace: "",
	}
	var functions []crossv1.Function
	items, _ := kube.GetKubeResources(params)
	for _, item := range items {
		var function crossv1.Function
		runtime.DefaultUnstructuredConverter.FromUnstructured(item.UnstructuredContent(), &function)
		functions = append(functions, function)
	}

	return functions
}
