package configuration

import (
	"context"
	"fmt"

	"github.com/kndpio/kndp/internal/configuration"
	"github.com/kndpio/kndp/internal/xrd"

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"

	"github.com/charmbracelet/huh"
	"github.com/pterm/pterm"
)

type applyCmd struct {
	Link string `arg:"" required:"" help:"Link URL to Crossplane configuration to be applied to Environment."`
}

func handleForm(xrds []string, Link string) {
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
				fmt.Println("Creating KNDP resource from: ", value, "\n")
			}
		}

	}
}

func (c *applyCmd) Run(p pterm.TextPrinter, ctx context.Context, config *rest.Config, dynamicClient *dynamic.DynamicClient) error {
	configuration.ApplyConfiguration(c.Link, config)
	xrds := xrd.GetXRDs(c.Link, ctx, config, dynamicClient)
	handleForm(xrds, c.Link)

	return nil
}
