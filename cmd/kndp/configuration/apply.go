package configuration

import (
	"context"
	"fmt"
	"net/url"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"

	"github.com/charmbracelet/huh"
	"github.com/kndpio/kndp/internal/install/helm"
	"github.com/pterm/pterm"
)

type applyCmd struct {
	Link string `arg:"" required:"" help:"Link URL to Crossplane configuration to be applied to Environment."`
}
type ResourceParams struct {
	dynamic   dynamic.Interface
	ctx       context.Context
	group     string
	version   string
	resource  string
	namespace string
}

type ObjectRef struct {
	Name string `json:"name"`
	Kind string `json:"kind"`
}
type ConfigurationRevisions struct {
	Spec struct {
		Image string `json:"image"`
	} `json:"spec"`
	Status struct {
		ObjectRefs []ObjectRef `json:"objectRefs"`
	}
}

func GetResources(p ResourceParams) ([]unstructured.Unstructured, error) {
	resourceId := schema.GroupVersionResource{
		Group:    p.group,
		Version:  p.version,
		Resource: p.resource,
	}
	list, err := p.dynamic.Resource(resourceId).Namespace(p.namespace).
		List(p.ctx, metav1.ListOptions{})

	if err != nil {
		return nil, err
	}

	return list.Items, nil
}

func fetchXRDs(Link string, ctx context.Context, config *rest.Config, dynamicClient *dynamic.DynamicClient) []string {
	var xrds []string
	var ResourceParams = ResourceParams{
		dynamic:   dynamicClient,
		ctx:       ctx,
		group:     "pkg.crossplane.io",
		version:   "v1",
		resource:  "configurationrevisions",
		namespace: "",
	}

	items, _ := GetResources(ResourceParams)
	for _, item := range items {
		image := &ConfigurationRevisions{}
		runtime.DefaultUnstructuredConverter.FromUnstructured(item.Object, image)
		if image.Spec.Image == Link {
			for _, objectRef := range image.Status.ObjectRefs {
				if objectRef.Kind == "CompositeResourceDefinition" {
					xrds = append(xrds, objectRef.Name)
				}
			}
		}
	}
	return xrds
}

func applyConfiguration(Link string, config *rest.Config) {
	chartName := "crossplane"

	repoURL, err := url.Parse("https://charts.crossplane.io/stable")
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	}

	installer, err := helm.NewManager(config, chartName, repoURL)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	}

	parameters := map[string]interface{}{
		"configuration": map[string]interface{}{
			"packages": []string{Link},
		},
	}

	err = installer.Upgrade("", parameters)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	}

	pterm.Success.Println("KNDP configuration applied successfully.")

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
	formConfirm.Run()
	if createResource {
		if len(xrds) > 0 {
			form.Run()
		} else {
			fmt.Println("No resource to create from: ", Link)
		}
		selectedValues := xrds
		for _, value := range selectedValues {
			fmt.Println("Creating KNDP resource from: ", value, "\n")
		}
	}
}

func (c *applyCmd) Run(p pterm.TextPrinter, ctx context.Context, config *rest.Config, dynamicClient *dynamic.DynamicClient) error {
	applyConfiguration(c.Link, config)
	xrds := fetchXRDs(c.Link, ctx, config, dynamicClient)
	handleForm(xrds, c.Link)

	return nil
}
