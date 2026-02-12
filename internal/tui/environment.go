package tui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/web-seven/overlock/internal/engine"
	"github.com/web-seven/overlock/pkg/environment"
	"go.uber.org/zap"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

// EnvironmentView represents the different views in environment management
type EnvironmentView int

const (
	EnvViewList EnvironmentView = iota
	EnvViewCreate
	EnvViewDelete
	EnvViewDetails
)

// EnvironmentModel manages the environment section of the TUI
type EnvironmentModel struct {
	View             EnvironmentView
	table            table.Model
	environments     []EnvironmentInfo
	selectedEnv      int
	createForm       *CreateEnvForm
	deleteConfirm    bool
	width            int
	height           int
	styles           Styles
	logger           *zap.SugaredLogger
	loading          bool
	errorMsg         string
	successMsg       string
	showingConfirm   bool
	confirmSelection int // 0 = No, 1 = Yes
}

// EnvironmentInfo holds information about an environment
type EnvironmentInfo struct {
	Name   string
	Type   string
	Status string
}

// CreateEnvForm represents the environment creation form
type CreateEnvForm struct {
	inputs       []textinput.Model
	focusIndex   int
	engineType   int // 0=kind, 1=k3s, 2=k3d
	httpPort     string
	httpsPort    string
	selectedOpts []bool // Track which options are selected
}

// Messages for async operations
type envListLoadedMsg struct {
	environments []EnvironmentInfo
	err          error
}

type envCreateMsg struct {
	success bool
	err     error
}

type envDeleteMsg struct {
	success bool
	err     error
}

// NewEnvironmentModel creates a new environment management model
func NewEnvironmentModel(width, height int, styles Styles, logger *zap.SugaredLogger) *EnvironmentModel {
	columns := []table.Column{
		{Title: "Name", Width: 30},
		{Title: "Type", Width: 10},
		{Title: "Status", Width: 15},
	}

	t := table.New(
		table.WithColumns(columns),
		table.WithFocused(true),
		table.WithHeight(height-10),
	)

	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(styles.Theme.Border).
		BorderBottom(true).
		Bold(true).
		Foreground(styles.Theme.Primary)
	s.Selected = s.Selected.
		Foreground(styles.Theme.SelectedFg).
		Background(styles.Theme.SelectedBg).
		Bold(true)

	t.SetStyles(s)

	return &EnvironmentModel{
		View:         EnvViewList,
		table:        t,
		environments: []EnvironmentInfo{},
		width:        width,
		height:       height,
		styles:       styles,
		logger:       logger,
		loading:      true,
	}
}

// Init initializes the environment model
func (m *EnvironmentModel) Init() tea.Cmd {
	return m.loadEnvironments()
}

// Update handles messages for the environment model
func (m *EnvironmentModel) Update(msg tea.Msg) (*EnvironmentModel, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.table.SetHeight(m.height - 10)
		return m, nil

	case envListLoadedMsg:
		m.loading = false
		if msg.err != nil {
			m.errorMsg = fmt.Sprintf("Error loading environments: %v", msg.err)
			return m, nil
		}
		m.environments = msg.environments
		m.updateTableRows()
		return m, nil

	case envCreateMsg:
		if msg.success {
			m.successMsg = "Environment created successfully!"
			m.View = EnvViewList
			m.loading = true
			return m, m.loadEnvironments()
		} else {
			m.errorMsg = fmt.Sprintf("Error creating environment: %v", msg.err)
		}
		return m, nil

	case envDeleteMsg:
		m.showingConfirm = false
		if msg.success {
			m.successMsg = "Environment deleted successfully!"
			m.loading = true
			return m, m.loadEnvironments()
		} else {
			m.errorMsg = fmt.Sprintf("Error deleting environment: %v", msg.err)
		}
		return m, nil

	case tea.KeyMsg:
		// Clear messages on any key press
		if m.errorMsg != "" || m.successMsg != "" {
			m.errorMsg = ""
			m.successMsg = ""
		}

		switch m.View {
		case EnvViewList:
			return m.handleListKeys(msg)
		case EnvViewCreate:
			return m.handleCreateKeys(msg)
		case EnvViewDelete:
			return m.handleDeleteKeys(msg)
		case EnvViewDetails:
			return m.handleDetailsKeys(msg)
		}
	}

	// Update table for navigation
	if m.View == EnvViewList {
		m.table, cmd = m.table.Update(msg)
		return m, cmd
	}

	return m, nil
}

// handleListKeys handles keyboard input in list view
func (m *EnvironmentModel) handleListKeys(msg tea.KeyMsg) (*EnvironmentModel, tea.Cmd) {
	switch msg.String() {
	case "c":
		// Create new environment
		m.View = EnvViewCreate
		m.createForm = newCreateEnvForm()
		return m, textinput.Blink
	case "d", "delete":
		// Delete selected environment
		if len(m.environments) > 0 {
			m.View = EnvViewDelete
			m.showingConfirm = true
			m.confirmSelection = 0 // Default to "No"
		}
		return m, nil
	case "r", "f5":
		// Refresh environment list
		m.loading = true
		return m, m.loadEnvironments()
	case "enter":
		// View environment details
		if len(m.environments) > 0 {
			m.View = EnvViewDetails
		}
		return m, nil
	}

	var cmd tea.Cmd
	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

// handleCreateKeys handles keyboard input in create view
func (m *EnvironmentModel) handleCreateKeys(msg tea.KeyMsg) (*EnvironmentModel, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.View = EnvViewList
		m.createForm = nil
		return m, nil
	case "tab", "shift+tab", "up", "down":
		return m.handleCreateFormNavigation(msg)
	case "enter":
		return m.handleCreateFormSubmit()
	}

	// Update focused input
	if m.createForm != nil && m.createForm.focusIndex < len(m.createForm.inputs) {
		var cmd tea.Cmd
		m.createForm.inputs[m.createForm.focusIndex], cmd = m.createForm.inputs[m.createForm.focusIndex].Update(msg)
		return m, cmd
	}

	return m, nil
}

// handleDeleteKeys handles keyboard input in delete confirmation view
func (m *EnvironmentModel) handleDeleteKeys(msg tea.KeyMsg) (*EnvironmentModel, tea.Cmd) {
	switch msg.String() {
	case "esc", "n":
		m.View = EnvViewList
		m.showingConfirm = false
		return m, nil
	case "left", "h":
		m.confirmSelection = 0 // No
		return m, nil
	case "right", "l":
		m.confirmSelection = 1 // Yes
		return m, nil
	case "enter":
		if m.confirmSelection == 1 {
			// Delete confirmed
			return m, m.deleteEnvironment()
		}
		m.View = EnvViewList
		m.showingConfirm = false
		return m, nil
	case "y":
		// Quick yes
		return m, m.deleteEnvironment()
	}
	return m, nil
}

// handleDetailsKeys handles keyboard input in details view
func (m *EnvironmentModel) handleDetailsKeys(msg tea.KeyMsg) (*EnvironmentModel, tea.Cmd) {
	switch msg.String() {
	case "esc", "backspace":
		m.View = EnvViewList
		return m, nil
	}
	return m, nil
}

// handleCreateFormNavigation handles navigation in create form
func (m *EnvironmentModel) handleCreateFormNavigation(msg tea.KeyMsg) (*EnvironmentModel, tea.Cmd) {
	if m.createForm == nil {
		return m, nil
	}

	s := msg.String()

	// Navigate between inputs
	if s == "tab" || s == "down" {
		m.createForm.focusIndex++
		if m.createForm.focusIndex > len(m.createForm.inputs) {
			m.createForm.focusIndex = 0
		}
	} else if s == "shift+tab" || s == "up" {
		m.createForm.focusIndex--
		if m.createForm.focusIndex < 0 {
			m.createForm.focusIndex = len(m.createForm.inputs)
		}
	}

	// Update focus
	cmds := make([]tea.Cmd, len(m.createForm.inputs))
	for i := 0; i < len(m.createForm.inputs); i++ {
		if i == m.createForm.focusIndex {
			cmds[i] = m.createForm.inputs[i].Focus()
		} else {
			m.createForm.inputs[i].Blur()
		}
	}

	return m, tea.Batch(cmds...)
}

// handleCreateFormSubmit handles form submission
func (m *EnvironmentModel) handleCreateFormSubmit() (*EnvironmentModel, tea.Cmd) {
	if m.createForm == nil || m.createForm.focusIndex != len(m.createForm.inputs) {
		// Not on submit button yet
		return m.handleCreateFormNavigation(tea.KeyMsg{Type: tea.KeyTab})
	}

	// Validate and create environment
	name := m.createForm.inputs[0].Value()
	if name == "" {
		m.errorMsg = "Environment name is required"
		return m, nil
	}

	return m, m.createEnvironment(name)
}

// GetView renders the environment management view
func (m *EnvironmentModel) GetView() string {
	switch m.View {
	case EnvViewList:
		return m.renderListView()
	case EnvViewCreate:
		return m.renderCreateView()
	case EnvViewDelete:
		return m.renderDeleteView()
	case EnvViewDetails:
		return m.renderDetailsView()
	default:
		return m.renderListView()
	}
}

// renderListView renders the environment list
func (m *EnvironmentModel) renderListView() string {
	var b strings.Builder

	// Title
	title := m.styles.TitleStyle.Bold(true).Foreground(m.styles.Theme.Primary).Render("Environment Management")
	b.WriteString(title)
	b.WriteString("\n\n")

	// Show error or success message
	if m.errorMsg != "" {
		b.WriteString(m.styles.ErrorStyle.Render(m.errorMsg))
		b.WriteString("\n\n")
	} else if m.successMsg != "" {
		b.WriteString(m.styles.SuccessStyle.Render(m.successMsg))
		b.WriteString("\n\n")
	}

	// Show loading or table
	if m.loading {
		b.WriteString(m.styles.MutedTextStyle.Render("Loading environments..."))
	} else if len(m.environments) == 0 {
		b.WriteString(m.styles.MutedTextStyle.Render("No environments found"))
		b.WriteString("\n\n")
		b.WriteString(m.styles.TextStyle.Render("Press "))
		b.WriteString(m.styles.KeyStyle.Render("c"))
		b.WriteString(m.styles.TextStyle.Render(" to create your first environment"))
	} else {
		b.WriteString(m.table.View())
	}

	b.WriteString("\n\n")

	// Help text
	helpStyle := m.styles.MutedTextStyle
	b.WriteString(helpStyle.Render("Controls: "))
	b.WriteString(m.styles.KeyStyle.Render("↑/↓"))
	b.WriteString(helpStyle.Render(" navigate  "))
	b.WriteString(m.styles.KeyStyle.Render("c"))
	b.WriteString(helpStyle.Render(" create  "))
	b.WriteString(m.styles.KeyStyle.Render("d"))
	b.WriteString(helpStyle.Render(" delete  "))
	b.WriteString(m.styles.KeyStyle.Render("r"))
	b.WriteString(helpStyle.Render(" refresh  "))
	b.WriteString(m.styles.KeyStyle.Render("enter"))
	b.WriteString(helpStyle.Render(" details"))

	return b.String()
}

// renderCreateView renders the environment creation form
func (m *EnvironmentModel) renderCreateView() string {
	var b strings.Builder

	title := m.styles.TitleStyle.Bold(true).Foreground(m.styles.Theme.Primary).Render("Create New Environment")
	b.WriteString(title)
	b.WriteString("\n\n")

	if m.errorMsg != "" {
		b.WriteString(m.styles.ErrorStyle.Render(m.errorMsg))
		b.WriteString("\n\n")
	}

	if m.createForm == nil {
		return b.String()
	}

	// Render form fields
	for i, input := range m.createForm.inputs {
		label := ""
		switch i {
		case 0:
			label = "Environment Name:"
		case 1:
			label = "HTTP Port (default: 80):"
		case 2:
			label = "HTTPS Port (default: 443):"
		}

		if i == m.createForm.focusIndex {
			b.WriteString(m.styles.KeyStyle.Render("> " + label))
		} else {
			b.WriteString(m.styles.TextStyle.Render("  " + label))
		}
		b.WriteString("\n")
		b.WriteString("  " + input.View())
		b.WriteString("\n\n")
	}

	// Engine type selector
	label := "Engine Type:"
	if m.createForm.focusIndex == len(m.createForm.inputs) {
		b.WriteString(m.styles.KeyStyle.Render("> " + label))
	} else {
		b.WriteString(m.styles.TextStyle.Render("  " + label))
	}
	b.WriteString("\n  ")

	engines := []string{"kind", "k3s", "k3d"}
	for i, eng := range engines {
		if i == m.createForm.engineType {
			b.WriteString(m.styles.BadgePrimaryStyle.Render(" " + eng + " "))
		} else {
			b.WriteString(m.styles.BadgeStyle.Render(" " + eng + " "))
		}
		b.WriteString(" ")
	}

	b.WriteString("\n\n")

	// Submit button
	submitLabel := "[ Create Environment ]"
	if m.createForm.focusIndex > len(m.createForm.inputs) {
		b.WriteString(m.styles.KeyStyle.Render("> " + submitLabel))
	} else {
		b.WriteString(m.styles.TextStyle.Render("  " + submitLabel))
	}

	b.WriteString("\n\n")

	// Help text
	helpStyle := m.styles.MutedTextStyle
	b.WriteString(helpStyle.Render("Controls: "))
	b.WriteString(m.styles.KeyStyle.Render("tab"))
	b.WriteString(helpStyle.Render(" next field  "))
	b.WriteString(m.styles.KeyStyle.Render("enter"))
	b.WriteString(helpStyle.Render(" create  "))
	b.WriteString(m.styles.KeyStyle.Render("esc"))
	b.WriteString(helpStyle.Render(" cancel"))

	return b.String()
}

// renderDeleteView renders the delete confirmation dialog
func (m *EnvironmentModel) renderDeleteView() string {
	var b strings.Builder

	if len(m.environments) == 0 {
		return "No environment selected"
	}

	selectedIdx := m.table.Cursor()
	if selectedIdx >= len(m.environments) {
		selectedIdx = 0
	}
	env := m.environments[selectedIdx]

	title := m.styles.TitleStyle.Bold(true).Foreground(m.styles.Theme.Error).Render("Delete Environment")
	b.WriteString(title)
	b.WriteString("\n\n")

	if m.errorMsg != "" {
		b.WriteString(m.styles.ErrorStyle.Render(m.errorMsg))
		b.WriteString("\n\n")
	}

	warning := m.styles.WarningStyle.Render("⚠ Warning:")
	b.WriteString(warning)
	b.WriteString("\n")
	b.WriteString(m.styles.TextStyle.Render(fmt.Sprintf("You are about to delete environment '%s'", env.Name)))
	b.WriteString("\n")
	b.WriteString(m.styles.MutedTextStyle.Render("This action cannot be undone."))
	b.WriteString("\n\n")

	// Confirmation buttons
	b.WriteString(m.styles.TextStyle.Render("Are you sure? "))

	if m.confirmSelection == 0 {
		b.WriteString(m.styles.BadgePrimaryStyle.Render(" No "))
	} else {
		b.WriteString(m.styles.BadgeStyle.Render(" No "))
	}
	b.WriteString("  ")

	if m.confirmSelection == 1 {
		b.WriteString(m.styles.BadgeStyle.Background(m.styles.Theme.Error).Render(" Yes "))
	} else {
		b.WriteString(m.styles.BadgeStyle.Render(" Yes "))
	}

	b.WriteString("\n\n")

	// Help text
	helpStyle := m.styles.MutedTextStyle
	b.WriteString(helpStyle.Render("Controls: "))
	b.WriteString(m.styles.KeyStyle.Render("←/→"))
	b.WriteString(helpStyle.Render(" select  "))
	b.WriteString(m.styles.KeyStyle.Render("enter"))
	b.WriteString(helpStyle.Render(" confirm  "))
	b.WriteString(m.styles.KeyStyle.Render("y"))
	b.WriteString(helpStyle.Render(" yes  "))
	b.WriteString(m.styles.KeyStyle.Render("n/esc"))
	b.WriteString(helpStyle.Render(" cancel"))

	return b.String()
}

// renderDetailsView renders the environment details view
func (m *EnvironmentModel) renderDetailsView() string {
	var b strings.Builder

	if len(m.environments) == 0 {
		return "No environment selected"
	}

	selectedIdx := m.table.Cursor()
	if selectedIdx >= len(m.environments) {
		selectedIdx = 0
	}
	env := m.environments[selectedIdx]

	title := m.styles.TitleStyle.Bold(true).Foreground(m.styles.Theme.Primary).Render("Environment Details")
	b.WriteString(title)
	b.WriteString("\n\n")

	// Details
	b.WriteString(m.styles.KeyStyle.Render("Name: "))
	b.WriteString(m.styles.TextStyle.Render(env.Name))
	b.WriteString("\n")

	b.WriteString(m.styles.KeyStyle.Render("Type: "))
	b.WriteString(m.styles.TextStyle.Render(env.Type))
	b.WriteString("\n")

	b.WriteString(m.styles.KeyStyle.Render("Status: "))
	statusStyle := m.styles.SuccessStyle
	if env.Status != "Running" {
		statusStyle = m.styles.MutedTextStyle
	}
	b.WriteString(statusStyle.Render(env.Status))
	b.WriteString("\n\n")

	// Help text
	helpStyle := m.styles.MutedTextStyle
	b.WriteString(helpStyle.Render("Press "))
	b.WriteString(m.styles.KeyStyle.Render("esc"))
	b.WriteString(helpStyle.Render(" to go back"))

	return b.String()
}

// updateTableRows updates the table with current environment data
func (m *EnvironmentModel) updateTableRows() {
	rows := make([]table.Row, len(m.environments))
	for i, env := range m.environments {
		rows[i] = table.Row{env.Name, env.Type, env.Status}
	}
	m.table.SetRows(rows)
}

// loadEnvironments loads the list of environments
func (m *EnvironmentModel) loadEnvironments() tea.Cmd {
	return func() tea.Msg {
		envs, err := fetchEnvironments()
		return envListLoadedMsg{
			environments: envs,
			err:          err,
		}
	}
}

// createEnvironment creates a new environment
func (m *EnvironmentModel) createEnvironment(name string) tea.Cmd {
	return func() tea.Msg {
		engineTypes := []string{"kind", "k3s", "k3d"}
		engineType := engineTypes[m.createForm.engineType]

		ctx := context.Background()
		logger, _ := zap.NewProduction()
		defer logger.Sync()
		sugar := logger.Sugar()

		err := environment.
			New(engineType, name).
			WithHttpPort(80).
			WithHttpsPort(443).
			Create(ctx, sugar)

		return envCreateMsg{
			success: err == nil,
			err:     err,
		}
	}
}

// deleteEnvironment deletes the selected environment
func (m *EnvironmentModel) deleteEnvironment() tea.Cmd {
	return func() tea.Msg {
		selectedIdx := m.table.Cursor()
		if selectedIdx >= len(m.environments) {
			return envDeleteMsg{success: false, err: fmt.Errorf("invalid selection")}
		}

		env := m.environments[selectedIdx]

		logger, _ := zap.NewProduction()
		defer logger.Sync()
		sugar := logger.Sugar()

		// Extract engine type from environment name
		engineType := "kind"
		if strings.Contains(strings.ToLower(env.Type), "k3s") {
			engineType = "k3s"
		} else if strings.Contains(strings.ToLower(env.Type), "k3d") {
			engineType = "k3d"
		}

		err := environment.
			New(engineType, env.Name).
			Delete(true, sugar)

		return envDeleteMsg{
			success: err == nil,
			err:     err,
		}
	}
}

// newCreateEnvForm creates a new environment creation form
func newCreateEnvForm() *CreateEnvForm {
	inputs := make([]textinput.Model, 3)

	// Name input
	inputs[0] = textinput.New()
	inputs[0].Placeholder = "my-environment"
	inputs[0].Focus()
	inputs[0].CharLimit = 50
	inputs[0].Width = 40

	// HTTP Port
	inputs[1] = textinput.New()
	inputs[1].Placeholder = "80"
	inputs[1].CharLimit = 5
	inputs[1].Width = 40

	// HTTPS Port
	inputs[2] = textinput.New()
	inputs[2].Placeholder = "443"
	inputs[2].CharLimit = 5
	inputs[2].Width = 40

	return &CreateEnvForm{
		inputs:     inputs,
		focusIndex: 0,
		engineType: 0,
	}
}

// fetchEnvironments retrieves the list of environments from Kubernetes contexts
func fetchEnvironments() ([]EnvironmentInfo, error) {
	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig == "" {
		kubeconfig = filepath.Join(os.Getenv("HOME"), ".kube", "config")
	}

	configFile, err := clientcmd.LoadFromFile(kubeconfig)
	if err != nil {
		return nil, err
	}

	var environments []EnvironmentInfo

	for name := range configFile.Contexts {
		configClient, err := config.GetConfigWithContext(name)
		if err != nil {
			continue
		}

		if engine.IsHelmReleaseFound(configClient) {
			types := regexp.MustCompile(`(\w+)`).FindStringSubmatch(name)
			envType := "UNKNOWN"
			if len(types) > 0 {
				envType = strings.ToUpper(types[0])
			}

			environments = append(environments, EnvironmentInfo{
				Name:   name,
				Type:   envType,
				Status: "Running",
			})
		}
	}

	return environments, nil
}

// RenderEnvironmentView renders the environment management view for the main app
func RenderEnvironmentView(width, height int, styles Styles) string {
	logger, _ := zap.NewProduction()
	defer logger.Sync()
	sugar := logger.Sugar()

	model := NewEnvironmentModel(width, height, styles, sugar)
	model.Init()

	// Synchronously load environments for initial render
	envs, err := fetchEnvironments()
	if err == nil {
		model.environments = envs
		model.updateTableRows()
		model.loading = false
	} else {
		model.errorMsg = fmt.Sprintf("Error loading environments: %v", err)
		model.loading = false
	}

	return model.GetView()
}
