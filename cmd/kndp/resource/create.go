package resource

import (
	"context"
	"log"

	crossv1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	"github.com/kndpio/kndp/internal/resources"
	"github.com/pterm/pterm"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

type createCmd struct {
	Type string `arg:"" required:"" help:"XRD type name."`
}

func (c *createCmd) Run(ctx context.Context, p pterm.TextPrinter, client *dynamic.DynamicClient) error {

	xrd := crossv1.CompositeResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name: c.Type,
		},
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apiextensions.k8s.io/v1",
			Kind:       "customresourcedefinitions",
		},
	}
	createXResource(ctx, xrd, client)
	return nil
}

func createXResource(ctx context.Context, xrd crossv1.CompositeResourceDefinition, client *dynamic.DynamicClient) bool {
	xResource := resources.XResource{}
	form := xResource.GetSchemaFormFromXRDefinition(
		ctx,
		xrd,
		client,
	)

	err := form.Run()
	if err != nil {
		log.Fatal(err)
	}

	if form.GetBool("confirm") {

		groupVersion := schema.GroupVersionResource{
			Group:    xResource.GroupVersionKind().Group,
			Version:  xResource.GroupVersionKind().Version,
			Resource: xResource.Resource,
		}

		_, err := client.Resource(
			groupVersion,
		).Create(ctx, &xResource.Unstructured, metav1.CreateOptions{})
		if err != nil {
			log.Fatal(err)
		}
		return true
	}
	return false
}
