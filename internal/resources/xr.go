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
	"github.com/kndpio/kndp/internal/kube"
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
	XRD      *crossv1.CompositeResourceDefinition
	state    state
	lg       *lipgloss.Renderer
	styles   *Styles
	groups   map[string]*huh.Group
	view     *huh.Group
	theme    *huh.Theme
	keymap   *huh.KeyMap
	width    int
	logger   *log.Logger
	client   *dynamic.DynamicClient
	ctx      context.Context
	unstructured.Unstructured
}

var apiFields = []string{"apiVersion", "kind"}
var metadataFields = []string{"metadata"}

const maxWidth = 200

var path = ""

var (
	red    = lipgloss.AdaptiveColor{Light: "#FE5F86", Dark: "#FE5F86"}
	indigo = lipgloss.AdaptiveColor{Light: "#5A56E0", Dark: "#7571F9"}
	green  = lipgloss.AdaptiveColor{Light: "#02BA84", Dark: "#02BF87"}
)

type Styles struct {
	Base,
	HeaderText,
	Status,
	StatusHeader,
	Highlight,
	ErrorHeaderText,
	Help lipgloss.Style
}

func NewStyles(lg *lipgloss.Renderer) *Styles {
	s := Styles{}
	s.Base = lg.NewStyle().
		Padding(1, 4, 0, 1)
	s.HeaderText = lg.NewStyle().
		Foreground(indigo).
		Bold(true).
		Padding(0, 1, 0, 2)
	s.Status = lg.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(indigo).
		PaddingLeft(1).
		MarginTop(1)
	s.StatusHeader = lg.NewStyle().
		Foreground(green).
		Bold(true)
	s.Highlight = lg.NewStyle().
		Foreground(lipgloss.Color("212"))
	s.ErrorHeaderText = s.HeaderText.Copy().
		Foreground(red)
	s.Help = lg.NewStyle().
		Foreground(lipgloss.Color("240"))
	return &s
}

type state int

const (
	statusNormal state = iota
	stateDone
)

func CreateResourceModel(ctx context.Context, xrd *crossv1.CompositeResourceDefinition, client *dynamic.DynamicClient) XResource {
	m := XResource{width: maxWidth}
	m.lg = lipgloss.DefaultRenderer()
	m.styles = NewStyles(m.lg)
	if m.logger == nil {
		m.logger = log.Default()
	}
	m.client = client
	m.ctx = ctx
	m.XRD = xrd
	m.theme = huh.ThemeCharm()
	m.keymap = huh.NewDefaultKeyMap()

	m.GetSchemaFormFromXRDefinition()
	return m
}

func (m XResource) Init() tea.Cmd {
	var cmds []tea.Cmd

	for _, group := range m.groups {
		group.WithTheme(m.theme)
		group.WithKeyMap(m.keymap)
		cmd := group.Init()
		cmds = append(cmds, cmd)
	}
	return tea.Batch(cmds...)
}

func min(x, y int) int {
	if x > y {
		return y
	}
	return x
}

func reverse() {
	a := strings.Split(path, ".")
	path = strings.Join(a[:len(a)-1], ".")
}

func (m XResource) Update(msg tea.Msg) (tea.Model, tea.Cmd) {

	// huh.Form
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = min(msg.Width, maxWidth) - m.styles.Base.GetHorizontalFrameSize()
	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "ctrl+c", "q":
			return m, tea.Quit
		case "ctrl+d":
			reverse()
		}
	}
	var cmds []tea.Cmd

	for groupPath, group := range m.groups {
		if groupPath == path {
			model, cmd := group.Update(msg)
			if g, ok := model.(*huh.Group); ok {
				m.view = g
				cmds = append(cmds, cmd)
			}
		}
	}

	return m, tea.Batch(cmds...)
}

func (m XResource) View() string {
	s := m.styles
	v := ""
	if m.view != nil {
		v += strings.TrimSuffix(m.view.View(), "\n\n")
	}

	form := m.lg.NewStyle().Margin(1, 0).Render(v)

	// Status (right side)
	var status string
	{
		var (
			buildInfo      = "(None)"
			role           string
			jobDescription string
		)

		const statusWidth = 28
		statusMarginLeft := m.width - statusWidth - lipgloss.Width(form) - s.Status.GetMarginRight()
		status = s.Status.Copy().
			Height(lipgloss.Height(form)).
			Width(statusWidth).
			MarginLeft(statusMarginLeft).
			Render(s.StatusHeader.Render("Current Build") + "\n" +
				buildInfo +
				role +
				"Path:" + path +
				jobDescription)
	}

	// errors := m.form.Errors()
	header := m.appBoundaryView("Charm Employment Application")
	// if len(errors) > 0 {
	// 	header = m.appErrorBoundaryView(m.errorView())
	// }
	body := lipgloss.JoinHorizontal(lipgloss.Top, form, status)

	// footer := m.appBoundaryView(m.form.Help().ShortHelpView(m.form.KeyBinds()))
	// if len(errors) > 0 {
	// 	footer = m.appErrorBoundaryView("")
	// }

	return s.Base.Render(header + "\n" + body)
}

// func (m XResource) errorView() string {
// 	var s string
// 	for _, err := range m.form.Errors() {
// 		s += err.Error()
// 	}
// 	return s
// }

func (m XResource) appBoundaryView(text string) string {
	return lipgloss.PlaceHorizontal(
		m.width,
		lipgloss.Left,
		m.styles.HeaderText.Render(text),
		lipgloss.WithWhitespaceChars("/"),
		lipgloss.WithWhitespaceForeground(indigo),
	)
}

func (m XResource) appErrorBoundaryView(text string) string {
	return lipgloss.PlaceHorizontal(
		m.width,
		lipgloss.Left,
		m.styles.ErrorHeaderText.Render(text),
		lipgloss.WithWhitespaceChars("/"),
		lipgloss.WithWhitespaceForeground(red),
	)
}

func (xr *XResource) WithLogger(logger *log.Logger) {
	xr.logger = logger
}

func (xr *XResource) GetSchemaFormFromXRDefinition() *XResource {
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

func (xr *XResource) getFormGroupsByProps(schema *extv1.JSONSchemaProps, parent string) map[string]*huh.Group {
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
		group := huh.NewGroup(formFields...).Description(schema.Description).
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

func ApplyResources(ctx context.Context, client *dynamic.DynamicClient, logger *log.Logger, file string) error {
	resources, err := transformToUnstructured(file, logger)

	if err != nil {
		return err
	}
	for _, resource := range resources {
		apiAndVersion := strings.Split(resource.GetAPIVersion(), "/")

		resourceId := schema.GroupVersionResource{
			Group:    apiAndVersion[0],
			Version:  apiAndVersion[1],
			Resource: strings.ToLower(resource.GetKind()) + "s",
		}
		res, err := client.Resource(resourceId).Create(ctx, &resource, metav1.CreateOptions{})

		if err != nil {
			return err
		} else {
			logger.Infof("Resource %s from %s successfully applied", res.GetName(), res.GetAPIVersion())
		}
	}
	return nil
}

func MoveCompositeResources(ctx context.Context, logger *log.Logger, sourceContext dynamic.Interface, destinationContext dynamic.Interface, XRDs []unstructured.Unstructured) error {
	for _, xrd := range XRDs {
		var paramsXRs v1.CompositeResourceDefinition
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(xrd.UnstructuredContent(), &paramsXRs); err != nil {
			logger.Printf("Failed to convert item %s: %v\n", xrd.GetName(), err)
			return nil
		}
		for _, version := range paramsXRs.Spec.Versions {
			XRs, err := kube.GetKubeResources(kube.ResourceParams{
				Dynamic:   sourceContext,
				Ctx:       ctx,
				Group:     paramsXRs.Spec.Group,
				Version:   version.Name,
				Resource:  paramsXRs.Spec.Names.Plural,
				Namespace: "",
				ListOption: metav1.ListOptions{
					LabelSelector: "app.kubernetes.io/managed-by=kndp",
				},
			})
			if err != nil {
				logger.Error(err)
				return nil
			}

			for _, xr := range XRs {
				xr.SetResourceVersion("")
				resourceId := schema.GroupVersionResource{
					Group:    paramsXRs.Spec.Group,
					Version:  version.Name,
					Resource: paramsXRs.Spec.Names.Plural,
				}
				_, err = destinationContext.Resource(resourceId).Namespace("").Create(ctx, &xr, metav1.CreateOptions{})
				if err != nil {
					logger.Fatal(err)
					return nil
				} else {
					logger.Infof("Resource created successfully %s", xr.GetName())
				}
			}
		}
	}
	return nil
}
