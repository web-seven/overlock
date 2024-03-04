package resource

import (
	"context"

	"github.com/charmbracelet/log"

	crossv1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	"github.com/kndpio/kndp/internal/resources"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

type createCmd struct {
	Type string `arg:"" required:"" help:"XRD type name."`
}

func (c *createCmd) Run(ctx context.Context, client *dynamic.DynamicClient, logger *log.Logger) error {

	xrd := crossv1.CompositeResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name: c.Type,
		},
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apiextensions.k8s.io/v1",
			Kind:       "customresourcedefinitions",
		},
	}
	CreateXResource(ctx, xrd, client, logger)
	return nil
}

func CreateXResource(ctx context.Context, xrd crossv1.CompositeResourceDefinition, client *dynamic.DynamicClient, logger *log.Logger) bool {
	xResource := resources.XResource{}
	form := xResource.GetSchemaFormFromXRDefinition(
		ctx,
		xrd,
		client,
		logger,
	)

	if form != nil {
		form.Run()
	} else {
		return false
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
			logger.Error(err)
		}
		return true
	}
	return false
}
