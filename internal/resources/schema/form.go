package resources

import (
	"context"
	"encoding/json"
	"errors"
	"regexp"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/log"
	crossv1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	v1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

type SchemaFormModel struct {
	Resource string
	XRD      *crossv1.CompositeResourceDefinition
	groups   map[string]*huh.Group
	logger   *log.Logger
	client   *dynamic.DynamicClient
	ctx      context.Context
	unstructured.Unstructured
	styles SchemaFormStyles
}

type SchemaFormStyles struct {
	SchemaForm lipgloss.Style
}

var apiFields = []string{"apiVersion", "kind"}
var metadataFields = []string{"metadata"}

var path = ""

func CreateSchemaForm() SchemaFormModel {
	m := SchemaFormModel{}
	return m
}

// func (m SchemaFormModel) errorView() string {
// 	var s string
// 	for _, err := range m.form.Errors() {
// 		s += err.Error()
// 	}
// 	return s
// }

// Styles
func (m SchemaFormModel) initStyles(lg *lipgloss.Renderer) *SchemaFormStyles {
	s := SchemaFormStyles{}

	return &s
}

// Init
func (m SchemaFormModel) Init() tea.Cmd {
	var cmds []tea.Cmd

	return tea.Batch(cmds...)
}

// Update
func (m SchemaFormModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
	case tea.KeyMsg:
		switch keypress := msg.String(); keypress {

		case "enter":
		}
	}

	return m, tea.Batch(cmds...)
}

// View
func (m SchemaFormModel) View() string {
	return m.styles.SchemaForm.Render()
}

func (xr *SchemaFormModel) WithLogger(logger *log.Logger) {
	xr.logger = logger
}

func (xr *SchemaFormModel) GetSchemaFormFromXRDefinition() *SchemaFormModel {
	xrd := xr.XRD
	xrdInstance, err := xr.client.Resource(schema.GroupVersionResource{
		Group:    xrd.GroupVersionKind().Group,
		Version:  xrd.GroupVersionKind().Version,
		Resource: xrd.GroupVersionKind().Kind,
	}).Get(xr.ctx, xrd.Name, metav1.GetOptions{})

	if err != nil {
		xr.logger.Error(err)
		return nil
	}

	runtime.DefaultUnstructuredConverter.FromUnstructured(xrdInstance.UnstructuredContent(), xrd)

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

	versionSchema, _ := parseSchema(selectedVersion.Schema, xr.logger)

	xr.logger.Info("Type: \t\t" + xrd.Name)
	xr.logger.Info("Description: \t" + versionSchema.Description)

	xr.groups = xr.getFormGroupsByProps(versionSchema, "")

	xr.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   xrd.Spec.Group,
		Version: selectedVersion.Name,
	})

	xr.SetKind(xrd.Spec.Names.Kind)
	xr.Resource = xrd.Spec.Names.Plural

	return xr
}

func (xr *SchemaFormModel) getFormGroupsByProps(schema *extv1.JSONSchemaProps, parent string) map[string]*huh.Group {
	formGroups := map[string]*huh.Group{}
	formFields := []huh.Field{}
	groupsOptions := []huh.Option[string]{}

	if xr.Object == nil {
		xr.Object = make(map[string]interface{})
	}
	for propertyName, property := range schema.Properties {

		breadCrumbs := parent + "." + propertyName

		shortDescription := strings.SplitN(property.Description, ".", 2)[0]
		description := shortDescription
		isRequired := isStringInArray(schema.Required, propertyName)

		if (property.Type == "object" || property.Type == "array") && (!isStringInArray(metadataFields, propertyName) || len(property.Properties) > 0) {
			schemaXr := SchemaFormModel{}
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
							Title(breadCrumbs).
							Description(description).
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
					propertyGroups := schemaXr.getFormGroupsByProps(property.Items.Schema, breadCrumbs)
					mergeGroups(formGroups, propertyGroups)
					(xr.Object)[propertyName] = &[]map[string]interface{}{schemaXr.Object}

				}

			} else {
				propertyGroups := schemaXr.getFormGroupsByProps(&property, breadCrumbs)
				mergeGroups(formGroups, propertyGroups)
				groupsOptions = append(groupsOptions, huh.NewOption[string](propertyName, breadCrumbs))
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
					Title(breadCrumbs).
					Description(description).
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
					Title(breadCrumbs).
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
				Title(breadCrumbs),
			)
		} else if property.Type == "boolean" && !isStringInArray(apiFields, propertyName) {
			propertyValue := false
			(xr.Object)[propertyName] = &propertyValue
			formFields = append(formFields, huh.NewConfirm().
				Title(breadCrumbs).
				Value(&propertyValue),
			)
		} else if property.Type == "object" && isStringInArray(metadataFields, propertyName) {
			propertyValue := metav1.ObjectMeta{
				Name: "",
				Labels: map[string]string{
					"app.kubernetes.io/managed-by": "kndp",
					"creation-date":                time.Now().String(),
					"update-date":                  time.Now().String(),
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

	if len(groupsOptions) > 0 {
		formFields = append(formFields, huh.NewSelect[string]().Options(groupsOptions...).Value(&path))
	}

	if len(formFields) > 0 {
		group := huh.NewGroup(formFields...).Description(schema.Description)
		formGroups[parent] = group
	}

	return formGroups
}

func parseSchema(v *v1.CompositeResourceValidation, logger *log.Logger) (*extv1.JSONSchemaProps, error) {
	if v == nil {
		return nil, nil
	}

	s := &extv1.JSONSchemaProps{}
	if err := json.Unmarshal(v.OpenAPIV3Schema.Raw, s); err != nil {
		logger.Error(err)
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
func mergeGroups(a map[string]*huh.Group, b map[string]*huh.Group) {
	for k, v := range b {
		a[k] = v
	}
}
