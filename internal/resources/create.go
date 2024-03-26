package resources

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	schema "github.com/kndpio/kndp/internal/resources/schema"
)

type ResourceModel struct {
	styles   *ResourceStyles
	width    int
	renderer *lipgloss.Renderer
	tree     schema.SchemaTreeModel
	form     schema.SchemaFormModel
}

type ResourceStyles struct {
	AppStyle,
	FormStyle,
	TreeStyle lipgloss.Style
}

var ()

// Model
func CreateResource() ResourceModel {

	m := ResourceModel{}
	m.renderer = lipgloss.DefaultRenderer()
	m.styles = m.initStyles(m.renderer)
	w, _ := m.styles.AppStyle.GetFrameSize()
	m.width = w
	m.tree = schema.CreateSchemaTree()
	m.form = schema.CreateSchemaForm()
	return m
}

// Styles
func (m ResourceModel) initStyles(lg *lipgloss.Renderer) *ResourceStyles {
	s := ResourceStyles{}

	s.AppStyle = lipgloss.NewStyle().Padding(1, 2)

	s.TreeStyle = lipgloss.NewStyle().
		Foreground(lipgloss.AdaptiveColor{Light: "#343433", Dark: "#C1C6B2"}).
		Background(lipgloss.AdaptiveColor{Light: "#D9DCCF", Dark: "#353533"}).
		MarginBottom(1)

	s.FormStyle = lipgloss.NewStyle().
		Foreground(lipgloss.AdaptiveColor{Light: "#343433", Dark: "#C1C6B2"}).
		Background(lipgloss.AdaptiveColor{Light: "#D9DCCF", Dark: "#353533"}).
		MarginBottom(1)

	return &s
}

// Init
func (m ResourceModel) Init() tea.Cmd {
	var cmds []tea.Cmd
	return tea.Batch(cmds...)
}

// Update
func (m ResourceModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
	}
	return m, tea.Batch(cmds...)
}

// View
func (m ResourceModel) View() string {
	return m.styles.AppStyle.Width(m.width).Render(
		lipgloss.JoinHorizontal(lipgloss.Top),
		m.tree.View(),
		m.form.View(),
	)
}
