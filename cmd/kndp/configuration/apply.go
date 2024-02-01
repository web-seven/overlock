package configuration

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"os"
	"os/exec"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/charmbracelet/huh"
	"github.com/kndpio/kndp/internal/install/helm"
	"github.com/pterm/pterm"
)

type applyCmd struct {
	Link string `arg:"" required:"" help:"Link URL to Crossplane configuration to be applied to Environment in next format: xpkg.upbound.io/upbound/platform-ref-aws:v0.6.0."`
}

func GetResources(dynamic dynamic.Interface, ctx context.Context, group string, version string, resource string, namespace string) ([]unstructured.Unstructured, error) {
	resourceId := schema.GroupVersionResource{
		Group:    group,
		Version:  version,
		Resource: resource,
	}
	list, err := dynamic.Resource(resourceId).Namespace(namespace).
		List(ctx, metav1.ListOptions{})

	if err != nil {
		return nil, err
	}

	return list.Items, nil
}

func (c *applyCmd) Run(p pterm.TextPrinter) error {

	var xrds []string

	ctx := context.Background()
	config := ctrl.GetConfigOrDie()
	dynamicClient := dynamic.NewForConfigOrDie(config)
	items, _ := GetResources(dynamicClient, ctx, "apiextensions.crossplane.io", "v1", "compositeresourcedefinitions", "")

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
			"packages": []string{c.Link},
		},
	}

	err = installer.Upgrade("", parameters)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	}

	pterm.Success.Println("Crossplane upgrade completed successfully.")

	//Display list of Crossplane configurations
	form := huh.NewForm(
		huh.NewGroup(
			func() *huh.MultiSelect[string] {
				multiSelect := huh.NewMultiSelect[string]().
					Title("Select Crossplane configuration to create resource:")
				var options []huh.Option[string]
				for _, item := range items {
					options = append(options, huh.NewOption(item.GetName(), item.GetName()))
				}
				multiSelect.Options(options...)
				multiSelect.Value(&xrds)
				return multiSelect
			}(),
		),
	)
	err = form.Run()
	if err != nil {
		log.Fatal(err)
	}

	selectedValues := xrds
	if len(selectedValues) > 0 {
		for _, value := range selectedValues {
			//This will execute kndp resource create <value> --interactive for every value selected
			cmd := exec.Command("kndp", "resource", "create", value, "--interactive")
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr

			if err := cmd.Run(); err != nil {
				log.Fatal(err)
			}
		}
	} else {
		fmt.Println("No values selected.")
	}

	return nil
}
