package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/web-seven/overlock/internal/engine"
	"go.uber.org/zap"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

// EnvironmentView modes
type envViewMode int

const (
	envViewList envViewMode = iota
	envViewCreate
	envViewDelete
	envViewDetails
)

// Environment represents an environment entry
type Environment struct {
	Name   string
	Type   string
	Status string
}

// EnvironmentViewModel handles the environment management interface
type EnvironmentViewModel struct {
	mode          envViewMode
	environments  []Environment
	table         table.Model
	list          list.Model
	createForm    *CreateEnvironmentForm
	deleteConfirm *DeleteConfirmDialog
	detailsView   *EnvironmentDetails
	selectedEnv   *Environment
	width         int
	height        int
	styles        Styles
	logger        *zap.SugaredLogger
	err           error
	message       string
}

// CreateEnvironmentForm handles the create environment form
type CreateEnvironmentForm struct {
	inputs       []textinput.Model
	focusIndex   int
	engineType   int
	engineTypes  []string
	submitAction bool
	cancelAction bool
}

// DeleteConfirmDialog handles delete confirmation
type DeleteConfirmDialog struct {
	env          Environment
	confirmed    bool
	cancelled    bool
	focusOnYes   bool
}

// EnvironmentDetails shows details for an environment
type EnvironmentDetails struct {
	env Environment
}

// Messages for async operations
type environmentsLoadedMsg struct {
	environments []Environment
	err          error
}

type environmentCreatedMsg struct {
	err error
}

type environmentDeletedMsg struct {
	err error
}

// NewEnvironmentViewModel creates a new environment view model
func NewEnvironmentViewModel(logger *zap.SugaredLogger, styles Styles) *EnvironmentViewModel {
	vm := &EnvironmentViewModel{
		mode:         envViewList,
		environments: []Environment{},
		styles:       styles,
		logger:       logger,
	}
	vm.initTable()
	return vm
}

// Init initializes the environment view
func (vm *EnvironmentViewModel) Init() tea.Cmd {
	return vm.loadEnvironments()
}

// Update handles messages for the environment view
func (vm *EnvironmentViewModel) Update(msg tea.Msg) tea.Cmd {
	switch vm.mode {
	case envViewList:
		return vm.updateList(msg)
	case envViewCreate:
		return vm.updateCreate(msg)
	case envViewDelete:
		return vm.updateDelete(msg)
	case envViewDetails:
		return vm.updateDetails(msg)
	}
	return nil
}

// View renders the environment view
func (vm *EnvironmentViewModel) View(width, height int) string {
	vm.width = width
	vm.height = height

	switch vm.mode {
	case envViewList:
		return vm.viewList()
	case envViewCreate:
		return vm.viewCreate()
	case envViewDelete:
		return vm.viewDelete()
	case envViewDetails:
		return vm.viewDetails()
	}
	return ""
}

// updateList handles updates for list view
func (vm *EnvironmentViewModel) updateList(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "c":
			// Create new environment
			vm.mode = envViewCreate
			vm.createForm = vm.initCreateForm()
			return textinput.Blink
		case "d":
			// Delete environment
			if len(vm.environments) > 0 {
				selectedRow := vm.table.Cursor()
				if selectedRow < len(vm.environments) {
					vm.selectedEnv = &vm.environments[selectedRow]
					vm.mode = envViewDelete
					vm.deleteConfirm = &DeleteConfirmDialog{
						env:        *vm.selectedEnv,
						focusOnYes: false,
					}
				}
			}
		case "enter":
			// View environment details
			if len(vm.environments) > 0 {
				selectedRow := vm.table.Cursor()
				if selectedRow < len(vm.environments) {
					vm.selectedEnv = &vm.environments[selectedRow]
					vm.mode = envViewDetails
					vm.detailsView = &EnvironmentDetails{
						env: *vm.selectedEnv,
					}
				}
			}
		case "r":
			// Refresh environment list
			return vm.loadEnvironments()
		}

	case environmentsLoadedMsg:
		vm.environments = msg.environments
		vm.err = msg.err
		if msg.err == nil {
			vm.updateTableData()
			vm.message = "Environments loaded successfully"
		} else {
			vm.message = fmt.Sprintf("Error loading environments: %v", msg.err)
		}
	}

	var cmd tea.Cmd
	vm.table, cmd = vm.table.Update(msg)
	return cmd
}

// updateCreate handles updates for create view
func (vm *EnvironmentViewModel) updateCreate(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			vm.mode = envViewList
			vm.createForm = nil
			return nil
		case "tab", "shift+tab", "up", "down":
			// Cycle through inputs
			if msg.String() == "up" || msg.String() == "shift+tab" {
				vm.createForm.focusIndex--
			} else {
				vm.createForm.focusIndex++
			}

			if vm.createForm.focusIndex > len(vm.createForm.inputs) {
				vm.createForm.focusIndex = 0
			} else if vm.createForm.focusIndex < 0 {
				vm.createForm.focusIndex = len(vm.createForm.inputs)
			}

			cmds := make([]tea.Cmd, len(vm.createForm.inputs))
			for i := 0; i <= len(vm.createForm.inputs)-1; i++ {
				if i == vm.createForm.focusIndex {
					cmds[i] = vm.createForm.inputs[i].Focus()
				} else {
					vm.createForm.inputs[i].Blur()
				}
			}
			return tea.Batch(cmds...)

		case "enter":
			if vm.createForm.focusIndex == len(vm.createForm.inputs) {
				// Submit button focused
				vm.createForm.submitAction = true
				return vm.createEnvironment()
			} else if vm.createForm.focusIndex < len(vm.createForm.inputs) {
				// Move to next field on enter
				vm.createForm.focusIndex++
				if vm.createForm.focusIndex >= len(vm.createForm.inputs) {
					vm.createForm.focusIndex = len(vm.createForm.inputs)
				}
				cmds := make([]tea.Cmd, len(vm.createForm.inputs))
				for i := 0; i <= len(vm.createForm.inputs)-1; i++ {
					if i == vm.createForm.focusIndex {
						cmds[i] = vm.createForm.inputs[i].Focus()
					} else {
						vm.createForm.inputs[i].Blur()
					}
				}
				return tea.Batch(cmds...)
			}

		case "left", "right":
			// Cycle engine type if engine field is focused
			if vm.createForm.focusIndex == 1 {
				if msg.String() == "right" {
					vm.createForm.engineType++
					if vm.createForm.engineType >= len(vm.createForm.engineTypes) {
						vm.createForm.engineType = 0
					}
				} else {
					vm.createForm.engineType--
					if vm.createForm.engineType < 0 {
						vm.createForm.engineType = len(vm.createForm.engineTypes) - 1
					}
				}
				return nil
			}
		}

	case environmentCreatedMsg:
		vm.err = msg.err
		if msg.err == nil {
			vm.message = "Environment created successfully"
			vm.mode = envViewList
			vm.createForm = nil
			return vm.loadEnvironments()
		} else {
			vm.message = fmt.Sprintf("Error creating environment: %v", msg.err)
		}
		return nil
	}

	// Update text inputs
	if vm.createForm.focusIndex < len(vm.createForm.inputs) {
		cmd := vm.updateFormInput(msg, vm.createForm.focusIndex)
		return cmd
	}

	return nil
}

// updateDelete handles updates for delete view
func (vm *EnvironmentViewModel) updateDelete(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "n":
			vm.mode = envViewList
			vm.deleteConfirm = nil
			return nil
		case "left", "right", "tab":
			vm.deleteConfirm.focusOnYes = !vm.deleteConfirm.focusOnYes
		case "y", "enter":
			if vm.deleteConfirm.focusOnYes || msg.String() == "y" {
				vm.deleteConfirm.confirmed = true
				return vm.deleteEnvironment()
			} else {
				vm.mode = envViewList
				vm.deleteConfirm = nil
			}
		}

	case environmentDeletedMsg:
		vm.err = msg.err
		if msg.err == nil {
			vm.message = "Environment deleted successfully"
			vm.mode = envViewList
			vm.deleteConfirm = nil
			return vm.loadEnvironments()
		} else {
			vm.message = fmt.Sprintf("Error deleting environment: %v", msg.err)
		}
	}
	return nil
}

// updateDetails handles updates for details view
func (vm *EnvironmentViewModel) updateDetails(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "q":
			vm.mode = envViewList
			vm.detailsView = nil
			return nil
		}
	}
	return nil
}

// viewList renders the list view
func (vm *EnvironmentViewModel) viewList() string {
	var s strings.Builder

	// Title
	title := vm.styles.TitleStyle.Bold(true).Render("Environment Management")
	s.WriteString(title)
	s.WriteString("\n\n")

	// Message/Error display
	if vm.message != "" {
		if vm.err != nil {
			s.WriteString(vm.styles.ErrorStyle.Render(vm.message))
		} else {
			s.WriteString(vm.styles.SuccessStyle.Render(vm.message))
		}
		s.WriteString("\n\n")
	}

	// Table
	vm.table.SetWidth(vm.width - 4)
	vm.table.SetHeight(vm.height - 12)
	s.WriteString(vm.table.View())
	s.WriteString("\n\n")

	// Help text
	help := vm.styles.HelpStyle.Render(
		"[c] Create  [d] Delete  [enter] Details  [r] Refresh  [esc] Back",
	)
	s.WriteString(help)

	return s.String()
}

// viewCreate renders the create form view
func (vm *EnvironmentViewModel) viewCreate() string {
	var s strings.Builder

	title := vm.styles.TitleStyle.Bold(true).Render("Create New Environment")
	s.WriteString(title)
	s.WriteString("\n\n")

	if vm.message != "" && vm.err != nil {
		s.WriteString(vm.styles.ErrorStyle.Render(vm.message))
		s.WriteString("\n\n")
	}

	// Name input
	s.WriteString(vm.styles.TextStyle.Render("Environment Name:"))
	s.WriteString("\n")
	s.WriteString(vm.createForm.inputs[0].View())
	s.WriteString("\n\n")

	// Engine type selector
	s.WriteString(vm.styles.TextStyle.Render("Engine Type:"))
	s.WriteString("\n")
	engineDisplay := vm.createForm.engineTypes[vm.createForm.engineType]
	if vm.createForm.focusIndex == 1 {
		engineDisplay = vm.styles.SelectedItemStyle.Render(
			fmt.Sprintf("< %s >", engineDisplay),
		)
	} else {
		engineDisplay = vm.styles.MenuItemStyle.Render(engineDisplay)
	}
	s.WriteString(engineDisplay)
	s.WriteString("\n")
	if vm.createForm.focusIndex == 1 {
		s.WriteString(vm.styles.MutedTextStyle.Render("  Use ← → to change"))
	}
	s.WriteString("\n\n")

	// HTTP Port input
	s.WriteString(vm.styles.TextStyle.Render("HTTP Port:"))
	s.WriteString("\n")
	s.WriteString(vm.createForm.inputs[2].View())
	s.WriteString("\n\n")

	// HTTPS Port input
	s.WriteString(vm.styles.TextStyle.Render("HTTPS Port:"))
	s.WriteString("\n")
	s.WriteString(vm.createForm.inputs[3].View())
	s.WriteString("\n\n")

	// Submit button
	submitBtn := "[ Create Environment ]"
	if vm.createForm.focusIndex == len(vm.createForm.inputs) {
		submitBtn = vm.styles.SelectedItemStyle.Render(submitBtn)
	} else {
		submitBtn = vm.styles.MenuItemStyle.Render(submitBtn)
	}
	s.WriteString(submitBtn)
	s.WriteString("\n\n")

	// Help
	help := vm.styles.HelpStyle.Render("[tab] Next field  [enter] Submit  [esc] Cancel")
	s.WriteString(help)

	return s.String()
}

// viewDelete renders the delete confirmation dialog
func (vm *EnvironmentViewModel) viewDelete() string {
	var s strings.Builder

	title := vm.styles.TitleStyle.Bold(true).Render("Delete Environment")
	s.WriteString(title)
	s.WriteString("\n\n")

	warning := vm.styles.WarningStyle.Render(
		fmt.Sprintf("Are you sure you want to delete environment '%s'?", vm.deleteConfirm.env.Name),
	)
	s.WriteString(warning)
	s.WriteString("\n")
	s.WriteString(vm.styles.ErrorStyle.Render("This action cannot be undone!"))
	s.WriteString("\n\n")

	// Yes/No buttons
	yesBtn := "[ Yes ]"
	noBtn := "[ No ]"

	if vm.deleteConfirm.focusOnYes {
		yesBtn = vm.styles.ErrorStyle.Render(yesBtn)
		noBtn = vm.styles.MenuItemStyle.Render(noBtn)
	} else {
		yesBtn = vm.styles.MenuItemStyle.Render(yesBtn)
		noBtn = vm.styles.SelectedItemStyle.Render(noBtn)
	}

	s.WriteString(yesBtn)
	s.WriteString("  ")
	s.WriteString(noBtn)
	s.WriteString("\n\n")

	if vm.message != "" && vm.err != nil {
		s.WriteString(vm.styles.ErrorStyle.Render(vm.message))
		s.WriteString("\n\n")
	}

	help := vm.styles.HelpStyle.Render("[tab/←/→] Switch  [enter/y] Confirm  [n/esc] Cancel")
	s.WriteString(help)

	return s.String()
}

// viewDetails renders the environment details view
func (vm *EnvironmentViewModel) viewDetails() string {
	var s strings.Builder

	title := vm.styles.TitleStyle.Bold(true).Render("Environment Details")
	s.WriteString(title)
	s.WriteString("\n\n")

	// Environment info
	detailStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(vm.styles.Theme.Border).
		Padding(1, 2).
		Width(vm.width - 4)

	details := fmt.Sprintf(
		"%s: %s\n%s: %s\n%s: %s",
		vm.styles.KeyStyle.Render("Name"),
		vm.styles.ValueStyle.Render(vm.detailsView.env.Name),
		vm.styles.KeyStyle.Render("Type"),
		vm.styles.ValueStyle.Render(vm.detailsView.env.Type),
		vm.styles.KeyStyle.Render("Status"),
		vm.styles.ValueStyle.Render(vm.detailsView.env.Status),
	)

	s.WriteString(detailStyle.Render(details))
	s.WriteString("\n\n")

	help := vm.styles.HelpStyle.Render("[esc] Back")
	s.WriteString(help)

	return s.String()
}

// initTable initializes the table model
func (vm *EnvironmentViewModel) initTable() {
	columns := []table.Column{
		{Title: "Name", Width: 30},
		{Title: "Type", Width: 15},
		{Title: "Status", Width: 15},
	}

	rows := []table.Row{}

	t := table.New(
		table.WithColumns(columns),
		table.WithRows(rows),
		table.WithFocused(true),
		table.WithHeight(10),
	)

	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(vm.styles.Theme.Border).
		BorderBottom(true).
		Bold(true).
		Foreground(vm.styles.Theme.Primary)
	s.Selected = s.Selected.
		Foreground(vm.styles.Theme.SelectedFg).
		Background(vm.styles.Theme.SelectedBg).
		Bold(false)

	t.SetStyles(s)
	vm.table = t
}

// updateTableData updates the table with current environments
func (vm *EnvironmentViewModel) updateTableData() {
	rows := []table.Row{}
	for _, env := range vm.environments {
		rows = append(rows, table.Row{env.Name, env.Type, env.Status})
	}
	vm.table.SetRows(rows)
}

// initCreateForm initializes the create environment form
func (vm *EnvironmentViewModel) initCreateForm() *CreateEnvironmentForm {
	form := &CreateEnvironmentForm{
		inputs:      make([]textinput.Model, 4),
		focusIndex:  0,
		engineType:  0,
		engineTypes: []string{"kind", "k3s", "k3d"},
	}

	// Name input
	form.inputs[0] = textinput.New()
	form.inputs[0].Placeholder = "my-environment"
	form.inputs[0].Focus()
	form.inputs[0].CharLimit = 50
	form.inputs[0].Width = 40

	// Engine type (handled separately, this is placeholder)
	form.inputs[1] = textinput.New()
	form.inputs[1].Placeholder = "kind"
	form.inputs[1].CharLimit = 10
	form.inputs[1].Width = 20

	// HTTP Port
	form.inputs[2] = textinput.New()
	form.inputs[2].Placeholder = "80"
	form.inputs[2].CharLimit = 5
	form.inputs[2].Width = 20

	// HTTPS Port
	form.inputs[3] = textinput.New()
	form.inputs[3].Placeholder = "443"
	form.inputs[3].CharLimit = 5
	form.inputs[3].Width = 20

	return form
}

// updateFormInput updates a specific form input
func (vm *EnvironmentViewModel) updateFormInput(msg tea.Msg, index int) tea.Cmd {
	var cmd tea.Cmd
	vm.createForm.inputs[index], cmd = vm.createForm.inputs[index].Update(msg)
	return cmd
}

// loadEnvironments loads the list of environments
func (vm *EnvironmentViewModel) loadEnvironments() tea.Cmd {
	return func() tea.Msg {
		environments, err := vm.fetchEnvironments()
		return environmentsLoadedMsg{
			environments: environments,
			err:          err,
		}
	}
}

// fetchEnvironments retrieves environments from kubeconfig
func (vm *EnvironmentViewModel) fetchEnvironments() ([]Environment, error) {
	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig == "" {
		kubeconfig = filepath.Join(os.Getenv("HOME"), ".kube", "config")
	}

	configFile, err := clientcmd.LoadFromFile(kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("failed to load kubeconfig: %w", err)
	}

	var environments []Environment
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

			status := "Running"
			environments = append(environments, Environment{
				Name:   name,
				Type:   envType,
				Status: status,
			})
		}
	}

	return environments, nil
}

// createEnvironment creates a new environment
func (vm *EnvironmentViewModel) createEnvironment() tea.Cmd {
	return func() tea.Msg {
		// Get form values
		name := vm.createForm.inputs[0].Value()
		if name == "" {
			return environmentCreatedMsg{
				err: fmt.Errorf("environment name is required"),
			}
		}

		engineType := vm.createForm.engineTypes[vm.createForm.engineType]

		// Parse port values with defaults
		httpPort := 80
		httpsPort := 443
		if vm.createForm.inputs[2].Value() != "" {
			fmt.Sscanf(vm.createForm.inputs[2].Value(), "%d", &httpPort)
		}
		if vm.createForm.inputs[3].Value() != "" {
			fmt.Sscanf(vm.createForm.inputs[3].Value(), "%d", &httpsPort)
		}

		vm.logger.Infof("Creating environment: %s with engine: %s", name, engineType)

		// TODO: Implement actual environment creation
		// This requires context and proper environment setup
		// For now, return success to demonstrate the UI flow
		// In real implementation, you would call:
		// ctx := context.Background()
		// err := environment.New(engineType, name).
		//     WithHttpPort(httpPort).
		//     WithHttpsPort(httpsPort).
		//     Create(ctx, vm.logger)

		return environmentCreatedMsg{
			err: nil,
		}
	}
}

// deleteEnvironment deletes an environment
func (vm *EnvironmentViewModel) deleteEnvironment() tea.Cmd {
	return func() tea.Msg {
		vm.logger.Infof("Deleting environment: %s (type: %s)",
			vm.deleteConfirm.env.Name,
			strings.ToLower(vm.deleteConfirm.env.Type))

		// TODO: Implement actual environment deletion
		// This requires proper engine type extraction from the environment
		// For now, return success to demonstrate the UI flow
		// In real implementation, you would call:
		// engineType := strings.ToLower(vm.deleteConfirm.env.Type)
		// err := environment.New(engineType, vm.deleteConfirm.env.Name).Delete(true, vm.logger)

		return environmentDeletedMsg{
			err: nil,
		}
	}
}
