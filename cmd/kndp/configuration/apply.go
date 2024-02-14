package configuration

import (
	"context"

	crossv1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	"github.com/kndpio/kndp/cmd/kndp/resource"
	"github.com/kndpio/kndp/internal/configuration"
	"github.com/kndpio/kndp/internal/xrd"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"

	"github.com/charmbracelet/huh"
	"github.com/pterm/pterm"
)

type applyCmd struct {
	Link string `arg:"" required:"" help:"Link URL to Crossplane configuration to be applied to Environment."`
}

func handleForm(ctx context.Context, client *dynamic.DynamicClient, xrds []string, Link string) {
	var createResource bool
	formConfirm := huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title("Would you like to create a resource from kndp configuration?").
				Affirmative("Yes!").
				Negative("No.").
				Value(&createResource),
		),
	)
	// Display a list of Crossplane configurations
	form := huh.NewForm(
		huh.NewGroup(
			func() *huh.MultiSelect[string] {
				multiSelect := huh.NewMultiSelect[string]().
					Title("Select KNDP configuration to create a resource:")
				var options []huh.Option[string]
				for _, xrd := range xrds {
					options = append(options, huh.NewOption(xrd, xrd).Selected(false))
				}
				multiSelect.Options(options...)
				multiSelect.Value(&xrds)

				return multiSelect
			}(),
		),
	)
	if len(xrds) > 0 {
		formConfirm.Run()
		if createResource {
			form.Run()
			selectedValues := xrds
			for _, value := range selectedValues {
				xrd := crossv1.CompositeResourceDefinition{
					ObjectMeta: metav1.ObjectMeta{
						Name: value,
					},
					TypeMeta: metav1.TypeMeta{
						APIVersion: "apiextensions.k8s.io/v1",
						Kind:       "customresourcedefinitions",
					},
				}
				resource.CreateXResource(ctx, xrd, client)
			}
		}
	}
}

func (c *applyCmd) Run(p pterm.TextPrinter, ctx context.Context, config *rest.Config, dynamicClient *dynamic.DynamicClient) error {
	configuration.ApplyConfiguration(c.Link, config)
	xrds := xrd.GetXRDs(c.Link, ctx, config, dynamicClient)
	handleForm(ctx, dynamicClient, xrds, c.Link)

	return nil
}
