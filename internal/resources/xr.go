package resources

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/charmbracelet/huh"
	crossv1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	v1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

type XResource struct {
	Resource string
	unstructured.Unstructured
}

var apiFields = []string{"apiVersion", "kind"}
var metadataFields = []string{"metadata"}

func (xr *XResource) GetSchemaFormFromXRDefinition(ctx context.Context, xrd crossv1.CompositeResourceDefinition, client *dynamic.DynamicClient) *huh.Form {

	xrdInstance, err := client.Resource(schema.GroupVersionResource{
		Group:    xrd.GroupVersionKind().Group,
		Version:  xrd.GroupVersionKind().Version,
		Resource: xrd.GroupVersionKind().Kind,
	}).Get(ctx, xrd.Name, metav1.GetOptions{})

	if err != nil {
		fmt.Println(err)
	}

	runtime.DefaultUnstructuredConverter.FromUnstructured(xrdInstance.UnstructuredContent(), &xrd)

	formGroups := []*huh.Group{}

	selectedVersion := v1.CompositeResourceDefinitionVersion{}
	if len(xrd.Spec.Versions) == 1 {
		selectedVersion = xrd.Spec.Versions[0]
	} else {
		selectedVersionIndex := 0
		versionOptions := []huh.Option[int]{}
		for index, version := range xrd.Spec.Versions {
			versionOptions = append(versionOptions, huh.NewOption(version.Name, index))
		}
		vesionSelectForm := huh.NewForm(
			huh.NewGroup(
				huh.NewSelect[int]().
					Title("Version").
					Options(versionOptions...).
					Value(&selectedVersionIndex),
			),
		)
		vesionSelectForm.Run()
		selectedVersion = xrd.Spec.Versions[selectedVersionIndex]
	}

	versionSchema, _ := parseSchema(selectedVersion.Schema)

	fmt.Println("Type: \t\t" + xrd.Name)
	fmt.Println("Description: \t" + versionSchema.Description)

	versionGroups := xr.getFormGroupsByProps(versionSchema, "")
	formGroups = append(formGroups, versionGroups...)

	xr.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   xrd.Spec.Group,
		Version: selectedVersion.Name,
	})

	xr.SetKind(xrd.Spec.Names.Kind)
	xr.Resource = xrd.Spec.Names.Plural

	formGroups = append(formGroups,
		huh.NewGroup(
			huh.NewConfirm().
				Key("confirm").
				Title("Would you like to create resource?"),
		),
	)

	schemaForm := huh.NewForm(formGroups...)

	return schemaForm
}

func (xr *XResource) getFormGroupsByProps(schema *extv1.JSONSchemaProps, parent string) []*huh.Group {
	formGroups := []*huh.Group{}
	formFields := []huh.Field{}

	if xr.Object == nil {
		xr.Object = make(map[string]interface{})
	}
	for propertyName, property := range schema.Properties {

		shortDescription := strings.SplitN(property.Description, ".", 2)[0]
		description := shortDescription + "[" + parent + "] [" + propertyName + "]"
		isRequired := isStringInArray(schema.Required, propertyName)

		if (property.Type == "object" || property.Type == "array") && (!isStringInArray(metadataFields, propertyName) || len(property.Properties) > 0) {
			schemaXr := XResource{}
			(xr.Object)[propertyName] = &schemaXr.Object

			if property.Type == "array" {
				if property.Items.Schema.Type == "string" {

					propertyValue := []string{}

					if property.Items.Schema.Description != "" {
						description = strings.SplitN(property.Items.Schema.Description, ".", 2)[0]
					}
					enums := []string{}
					for _, optionValue := range property.Items.Schema.Enum {
						timmedValue := strings.Trim(string(optionValue.Raw), "\"")
						enums = append(enums, timmedValue)
					}

					formFields = append(formFields,
						huh.NewText().
							Title(description).
							Lines(3).
							Validate(func(s string) error {

								if s != "" {
									if len(enums) > 0 {
										propertyValues := strings.Split(s, "\n")
										for _, optionValue := range propertyValues {
											if !isStringInArray(enums, optionValue) {
												return errors.New("supported values: " + strings.Join(enums, ", "))
											}
										}
									}
									propertyValue = strings.Split(s, "\n")

								}
								return nil
							}),
					)
					(xr.Object)[propertyName] = &propertyValue
				} else if property.Items.Schema.Type == "object" {

					propertyGroups := schemaXr.getFormGroupsByProps(property.Items.Schema, propertyName)
					formGroups = append(formGroups, propertyGroups...)
					(xr.Object)[propertyName] = &[]map[string]interface{}{schemaXr.Object}

				}

			} else {
				propertyGroups := schemaXr.getFormGroupsByProps(&property, propertyName)
				formGroups = append(formGroups, propertyGroups...)
				(xr.Object)[propertyName] = &schemaXr.Object
			}

		} else if property.Type == "string" && !isStringInArray(apiFields, propertyName) {
			propertyValue := ""
			(xr.Object)[propertyName] = &propertyValue

			if len(property.Enum) > 0 {
				if property.Default != nil {
					propertyValue = strings.Trim(string(property.Default.Raw), "\"")
				}
				options := []huh.Option[string]{}
				for _, optionValue := range property.Enum {
					timmedValue := strings.Trim(string(optionValue.Raw), "\"")
					options = append(options, huh.NewOption(timmedValue, timmedValue))
				}
				formFields = append(formFields, huh.NewSelect[string]().
					Options(options...).
					Title(description).
					Value(&propertyValue),
				)
			} else {
				formFields = append(formFields, huh.NewInput().
					Validate(func(s string) error {
						if isRequired && s == "" {
							return errors.New(propertyName + " is required")
						} else {
							return nil
						}
					}).
					Title(description).
					Value(&propertyValue),
				)
			}

		} else if property.Type == "number" && !isStringInArray(apiFields, propertyName) {
			propertyValue := json.Number("")
			(xr.Object)[propertyName] = &propertyValue
			formFields = append(formFields, huh.NewInput().
				Validate(func(s string) error {

					if s != "" && !regexp.MustCompile(`\d`).MatchString(s) {
						return errors.New(propertyName + " shall be numeric")
					}

					if isRequired && s == "" {
						return errors.New(propertyName + " is required")
					}

					propertyValue = json.Number(s)
					return nil

				}).
				Title(description),
			)
		} else if property.Type == "boolean" && !isStringInArray(apiFields, propertyName) {
			propertyValue := false
			(xr.Object)[propertyName] = &propertyValue
			formFields = append(formFields, huh.NewConfirm().
				Title(description).
				Value(&propertyValue),
			)
		} else if property.Type == "object" && isStringInArray(metadataFields, propertyName) {
			propertyValue := metav1.ObjectMeta{
				Name: "",
				Labels: map[string]string{
					"app.kubernetes.io/managed-by": "kndp",
				},
			}
			(xr.Object)[propertyName] = &propertyValue

			formFields = append(formFields, huh.NewInput().
				Validate(func(s string) error {
					if s == "" {
						return errors.New("name is required")
					} else {
						return nil
					}
				}).
				Title("Name of resource").
				Value(&propertyValue.Name),
			)
		}

	}
	if len(formFields) > 0 {
		group := huh.NewGroup(formFields...).Description(schema.Description)
		currentGroups := []*huh.Group{}
		currentGroups = append(currentGroups, group)
		formGroups = append(currentGroups, formGroups...)
	}

	return formGroups
}

func parseSchema(v *v1.CompositeResourceValidation) (*extv1.JSONSchemaProps, error) {
	if v == nil {
		return nil, nil
	}

	s := &extv1.JSONSchemaProps{}
	if err := json.Unmarshal(v.OpenAPIV3Schema.Raw, s); err != nil {
		fmt.Println(err)
	}
	return s, nil
}

func isStringInArray(a []string, s string) bool {
	for _, e := range a {
		if s == e {
			return true
		}
	}
	return false
}
