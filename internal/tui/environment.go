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

// EnvironmentView represents the current view state
type EnvironmentView int

const (
	ViewList EnvironmentView = iota
	ViewCreate
	ViewDelete
	ViewDetails
)

// EnvironmentItem represents an environment in the list
type EnvironmentItem struct {
	name        string
	engineType  string
	status      string
	description string
}

// Implement list.Item interface
func (i EnvironmentItem) Title() string       { return i.name }
func (i EnvironmentItem) Description() string { return fmt.Sprintf("%s • %s", i.engineType, i.status) }
func (i EnvironmentItem) FilterValue() string { return i.name }

// EnvironmentModel manages the environment interface state
type EnvironmentModel struct {
	logger         *zap.SugaredLogger
	styles         Styles
	width          int
	height         int
	currentView    EnvironmentView
	list           list.Model
	table          table.Model
	environments   []EnvironmentItem
	selectedEnv    *EnvironmentItem
	createInputs   []textinput.Model
	focusIndex     int
	confirmDelete  bool
	err            error
	loading        bool
	message        string
}

// NewEnvironmentModel creates a new environment management model
func NewEnvironmentModel(logger *zap.SugaredLogger, styles Styles) *EnvironmentModel {
	// Create list for environment selection
	items := []list.Item{}
	delegate := list.NewDefaultDelegate()
	l := list.New(items, delegate, 0, 0)
	l.Title = "Environments"
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(true)
	l.SetShowHelp(true)

	// Create table for environment list view
	columns := []table.Column{
		{Title: "Name", Width: 30},
		{Title: "Type", Width: 10},
		{Title: "Status", Width: 15},
	}
	t := table.New(
		table.WithColumns(columns),
		table.WithFocused(true),
		table.WithHeight(10),
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

	// Create text inputs for create form
	inputs := make([]textinput.Model, 4)

	inputs[0] = textinput.New()
	inputs[0].Placeholder = "my-environment"
	inputs[0].Focus()
	inputs[0].CharLimit = 50
	inputs[0].Width = 40
	inputs[0].Prompt = "> "

	inputs[1] = textinput.New()
	inputs[1].Placeholder = "kind"
	inputs[1].CharLimit = 10
	inputs[1].Width = 40
	inputs[1].Prompt = "> "

	inputs[2] = textinput.New()
	inputs[2].Placeholder = "80"
	inputs[2].CharLimit = 5
	inputs[2].Width = 40
	inputs[2].Prompt = "> "

	inputs[3] = textinput.New()
	inputs[3].Placeholder = "443"
	inputs[3].CharLimit = 5
	inputs[3].Width = 40
	inputs[3].Prompt = "> "

	return &EnvironmentModel{
		logger:       logger,
		styles:       styles,
		currentView:  ViewList,
		list:         l,
		table:        t,
		environments: []EnvironmentItem{},
		createInputs: inputs,
		focusIndex:   0,
	}
}

// Init initializes the environment model
func (m *EnvironmentModel) Init() tea.Cmd {
	return m.loadEnvironments
}

// Update handles messages
func (m *EnvironmentModel) Update(msg tea.Msg) (*EnvironmentModel, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.table.SetWidth(msg.Width - 4)
		m.table.SetHeight(msg.Height - 10)
		m.list.SetSize(msg.Width-4, msg.Height-10)

	case tea.KeyMsg:
		switch m.currentView {
		case ViewList:
			return m.handleListKeys(msg)
		case ViewCreate:
			return m.handleCreateKeys(msg)
		case ViewDelete:
			return m.handleDeleteKeys(msg)
		case ViewDetails:
			return m.handleDetailsKeys(msg)
		}

	case environmentsLoadedMsg:
		m.environments = msg.environments
		m.loading = false
		m.updateTableRows()
		m.updateListItems()

	case environmentCreatedMsg:
		m.loading = false
		m.message = fmt.Sprintf("Environment '%s' created successfully!", msg.name)
		m.currentView = ViewList
		return m, m.loadEnvironments

	case environmentDeletedMsg:
		m.loading = false
		m.message = fmt.Sprintf("Environment '%s' deleted successfully!", msg.name)
		m.currentView = ViewList
		return m, m.loadEnvironments

	case errorMsg:
		m.loading = false
		m.err = msg.err
		m.message = ""

	case clearMessageMsg:
		m.message = ""
		m.err = nil
	}

	// Update sub-components
	switch m.currentView {
	case ViewList:
		m.table, cmd = m.table.Update(msg)
		cmds = append(cmds, cmd)
	case ViewCreate:
		for i := range m.createInputs {
			m.createInputs[i], cmd = m.createInputs[i].Update(msg)
			cmds = append(cmds, cmd)
		}
	}

	return m, tea.Batch(cmds...)
}

// View renders the environment interface
func (m *EnvironmentModel) View() string {
	if m.loading {
		return m.styles.TextStyle.Render("Loading...")
	}

	var content string
	switch m.currentView {
	case ViewList:
		content = m.viewList()
	case ViewCreate:
		content = m.viewCreate()
	case ViewDelete:
		content = m.viewDelete()
	case ViewDetails:
		content = m.viewDetails()
	}

	return content
}

// FooterShortcuts returns the shortcuts for the footer based on current view
func (m *EnvironmentModel) FooterShortcuts() string {
	switch m.currentView {
	case ViewList:
		return "[c] Create  [d] Delete  [enter] Details  [r] Refresh  [esc] Back  [q] Quit"
	case ViewCreate:
		return "[tab] Navigate  [enter] Submit  [esc] Cancel  [q] Quit"
	case ViewDelete:
		return "[y] Confirm  [n] Cancel  [esc] Back  [q] Quit"
	case ViewDetails:
		return "[esc] Back  [q] Quit"
	default:
		return "[esc] Back  [q] Quit"
	}
}

// viewList renders the environment list view
func (m *EnvironmentModel) viewList() string {
	var b strings.Builder

	// Title
	title := m.styles.TextStyle.Bold(true).Foreground(m.styles.Theme.Primary).Render("Environment Management")
	b.WriteString(title)
	b.WriteString("\n\n")

	// Show message or error
	if m.message != "" {
		msg := m.styles.SuccessStyle.Render("✓ " + m.message)
		b.WriteString(msg)
		b.WriteString("\n\n")
	}
	if m.err != nil {
		errMsg := m.styles.ErrorStyle.Render("✗ Error: " + m.err.Error())
		b.WriteString(errMsg)
		b.WriteString("\n\n")
	}

	// Environment table
	if len(m.environments) == 0 {
		noEnvMsg := m.styles.MutedTextStyle.Render("No environments found.")
		b.WriteString(noEnvMsg)
	} else {
		b.WriteString(m.table.View())
	}

	return b.String()
}

// viewCreate renders the create environment form
func (m *EnvironmentModel) viewCreate() string {
	var b strings.Builder

	// Title
	title := m.styles.TextStyle.Bold(true).Foreground(m.styles.Theme.Primary).Render("Create New Environment")
	b.WriteString(title)
	b.WriteString("\n\n")

	// Form fields
	fields := []string{"Name", "Engine (kind/k3s/k3d)", "HTTP Port", "HTTPS Port"}
	for i, field := range fields {
		b.WriteString(m.styles.TextStyle.Render(field + ":"))
		b.WriteString("\n")

		if i == m.focusIndex {
			b.WriteString(m.createInputs[i].View())
		} else {
			b.WriteString(m.styles.MutedTextStyle.Render(m.createInputs[i].View()))
		}
	}

	// Show error if any
	if m.err != nil {
		errMsg := m.styles.ErrorStyle.Render("✗ Error: " + m.err.Error())
		b.WriteString(errMsg)
	}

	return b.String()
}

// viewDelete renders the delete confirmation dialog
func (m *EnvironmentModel) viewDelete() string {
	var b strings.Builder

	// Title
	title := m.styles.ErrorStyle.Render("⚠ Delete Environment")
	b.WriteString(title)
	b.WriteString("\n\n")

	if m.selectedEnv == nil {
		b.WriteString(m.styles.TextStyle.Render("No environment selected."))
		return b.String()
	}

	// Confirmation message
	msg := fmt.Sprintf("Are you sure you want to delete environment '%s'?", m.selectedEnv.name)
	b.WriteString(m.styles.TextStyle.Render(msg))
	b.WriteString("\n\n")

	// Environment details
	details := fmt.Sprintf("Type: %s\nStatus: %s", m.selectedEnv.engineType, m.selectedEnv.status)
	b.WriteString(m.styles.MutedTextStyle.Render(details))
	b.WriteString("\n\n")

	// Warning
	warning := m.styles.WarningStyle.Render("⚠ This action cannot be undone!")
	b.WriteString(warning)
	b.WriteString("\n\n")

	// Show error if any
	if m.err != nil {
		errMsg := m.styles.ErrorStyle.Render("✗ Error: " + m.err.Error())
		b.WriteString(errMsg)
	}

	return b.String()
}

// viewDetails renders the environment details view
func (m *EnvironmentModel) viewDetails() string {
	var b strings.Builder

	// Title
	title := m.styles.TextStyle.Bold(true).Foreground(m.styles.Theme.Primary).Render("Environment Details")
	b.WriteString(title)
	b.WriteString("\n\n")

	if m.selectedEnv == nil {
		b.WriteString(m.styles.TextStyle.Render("No environment selected."))
		return b.String()
	}

	// Environment details
	nameLabel := m.styles.KeyStyle.Render("Name:")
	nameValue := m.styles.ValueStyle.Render(m.selectedEnv.name)
	b.WriteString(fmt.Sprintf("%s %s\n", nameLabel, nameValue))

	typeLabel := m.styles.KeyStyle.Render("Type:")
	typeValue := m.styles.ValueStyle.Render(m.selectedEnv.engineType)
	b.WriteString(fmt.Sprintf("%s %s\n", typeLabel, typeValue))

	statusLabel := m.styles.KeyStyle.Render("Status:")
	statusStyle := m.styles.SuccessStyle
	if m.selectedEnv.status == "inactive" {
		statusStyle = m.styles.MutedTextStyle
	}
	statusValue := statusStyle.Render(m.selectedEnv.status)
	b.WriteString(fmt.Sprintf("%s %s\n", statusLabel, statusValue))

	return b.String()
}

// Key handlers

func (m *EnvironmentModel) handleListKeys(msg tea.KeyMsg) (*EnvironmentModel, tea.Cmd) {
	var cmd tea.Cmd

	switch msg.String() {
	case "c":
		// Create new environment
		m.currentView = ViewCreate
		m.err = nil
		m.message = ""
		m.focusIndex = 0
		m.createInputs[0].Focus()
		for i := 1; i < len(m.createInputs); i++ {
			m.createInputs[i].Blur()
		}
		return m, nil

	case "d":
		// Delete environment
		if len(m.environments) > 0 {
			selectedIdx := m.table.Cursor()
			if selectedIdx < len(m.environments) {
				m.selectedEnv = &m.environments[selectedIdx]
				m.currentView = ViewDelete
				m.err = nil
				m.message = ""
			}
		}
		return m, nil

	case "enter":
		// View details
		if len(m.environments) > 0 {
			selectedIdx := m.table.Cursor()
			if selectedIdx < len(m.environments) {
				m.selectedEnv = &m.environments[selectedIdx]
				m.currentView = ViewDetails
				m.err = nil
				m.message = ""
			}
		}
		return m, nil

	case "r":
		// Refresh list
		m.loading = true
		m.err = nil
		m.message = ""
		return m, m.loadEnvironments

	default:
		// Clear messages on any other key
		if m.message != "" || m.err != nil {
			m.message = ""
			m.err = nil
		}
		m.table, cmd = m.table.Update(msg)
		return m, cmd
	}
}

func (m *EnvironmentModel) handleCreateKeys(msg tea.KeyMsg) (*EnvironmentModel, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg.String() {
	case "esc":
		// Cancel creation
		m.currentView = ViewList
		m.err = nil
		// Reset form
		for i := range m.createInputs {
			m.createInputs[i].SetValue("")
		}
		return m, nil

	case "tab", "shift+tab", "up", "down":
		// Navigate between inputs
		if msg.String() == "up" || msg.String() == "shift+tab" {
			m.focusIndex--
		} else {
			m.focusIndex++
		}

		if m.focusIndex < 0 {
			m.focusIndex = len(m.createInputs) - 1
		} else if m.focusIndex >= len(m.createInputs) {
			m.focusIndex = 0
		}

		for i := range m.createInputs {
			if i == m.focusIndex {
				m.createInputs[i].Focus()
			} else {
				m.createInputs[i].Blur()
			}
		}
		return m, nil

	case "enter":
		// Submit form
		name := m.createInputs[0].Value()
		engine := m.createInputs[1].Value()
		httpPort := m.createInputs[2].Value()
		httpsPort := m.createInputs[3].Value()

		// Validate
		if name == "" {
			m.err = fmt.Errorf("environment name is required")
			return m, nil
		}
		if engine == "" {
			engine = "kind"
		}
		if httpPort == "" {
			httpPort = "80"
		}
		if httpsPort == "" {
			httpsPort = "443"
		}

		m.loading = true
		m.err = nil
		return m, m.createEnvironment(name, engine, httpPort, httpsPort)
	}

	// For all other keys (regular text input), update the focused input
	for i := range m.createInputs {
		m.createInputs[i], cmd = m.createInputs[i].Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m *EnvironmentModel) handleDeleteKeys(msg tea.KeyMsg) (*EnvironmentModel, tea.Cmd) {
	switch msg.String() {
	case "y":
		// Confirm delete
		if m.selectedEnv != nil {
			m.loading = true
			m.err = nil
			return m, m.deleteEnvironment(m.selectedEnv.name, m.selectedEnv.engineType)
		}
		return m, nil

	case "n", "esc":
		// Cancel delete
		m.currentView = ViewList
		m.selectedEnv = nil
		m.err = nil
		return m, nil
	}

	return m, nil
}

func (m *EnvironmentModel) handleDetailsKeys(msg tea.KeyMsg) (*EnvironmentModel, tea.Cmd) {
	switch msg.String() {
	case "esc":
		// Go back to list
		m.currentView = ViewList
		m.selectedEnv = nil
		m.err = nil
		return m, nil
	}

	return m, nil
}

// Data loading and operations

func (m *EnvironmentModel) loadEnvironments() tea.Msg {
	environments := []EnvironmentItem{}

	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig == "" {
		kubeconfig = filepath.Join(os.Getenv("HOME"), ".kube", "config")
	}

	configFile, err := clientcmd.LoadFromFile(kubeconfig)
	if err != nil {
		return errorMsg{err: err}
	}

	for name := range configFile.Contexts {
		configClient, err := config.GetConfigWithContext(name)
		if err != nil {
			continue
		}

		if engine.IsHelmReleaseFound(configClient) {
			types := regexp.MustCompile(`(\w+)`).FindStringSubmatch(name)
			engineType := "unknown"
			if len(types) > 0 {
				engineType = strings.ToLower(types[0])
			}

			status := "active"
			// TODO: Implement actual status checking
			// For now, we'll assume all are active

			environments = append(environments, EnvironmentItem{
				name:       name,
				engineType: engineType,
				status:     status,
			})
		}
	}

	return environmentsLoadedMsg{environments: environments}
}

func (m *EnvironmentModel) createEnvironment(name, engine, httpPort, httpsPort string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()

		// Parse ports
		var http, https int
		fmt.Sscanf(httpPort, "%d", &http)
		fmt.Sscanf(httpsPort, "%d", &https)

		if http == 0 {
			http = 80
		}
		if https == 0 {
			https = 443
		}

		// Create environment using the existing package
		err := environment.
			New(engine, name).
			WithHttpPort(http).
			WithHttpsPort(https).
			Create(ctx, m.logger)

		if err != nil {
			return errorMsg{err: err}
		}

		return environmentCreatedMsg{name: name}
	}
}

func (m *EnvironmentModel) deleteEnvironment(name, engine string) tea.Cmd {
	return func() tea.Msg {
		// Delete environment using the existing package
		err := environment.
			New(engine, name).
			Delete(true, m.logger) // true = skip confirmation (we already confirmed in TUI)

		if err != nil {
			return errorMsg{err: err}
		}

		return environmentDeletedMsg{name: name}
	}
}

func (m *EnvironmentModel) updateTableRows() {
	rows := []table.Row{}
	for _, env := range m.environments {
		rows = append(rows, table.Row{
			env.name,
			strings.ToUpper(env.engineType),
			env.status,
		})
	}
	m.table.SetRows(rows)
}

func (m *EnvironmentModel) updateListItems() {
	items := []list.Item{}
	for _, env := range m.environments {
		items = append(items, env)
	}
	m.list.SetItems(items)
}

// Messages

type environmentsLoadedMsg struct {
	environments []EnvironmentItem
}

type environmentCreatedMsg struct {
	name string
}

type environmentDeletedMsg struct {
	name string
}

type errorMsg struct {
	err error
}

type clearMessageMsg struct{}
