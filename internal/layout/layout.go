package layout

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/kndpio/kndp/internal/resources"
)

type LayoutModel struct {
	lg     *lipgloss.Renderer
	styles *Styles
	width  int
	header,
	menu,
	body,
	status tea.Model
}

type Styles struct {
	Base,
	Menu,
	Highlight,
	ErrorHeaderText,
	Help lipgloss.Style
}

const maxWidth = 200
const minHeight = 7

var (
	appStyle = lipgloss.NewStyle().Padding(1, 2)
)

// Model
func CreateLayoutModel() LayoutModel {

	m := LayoutModel{width: maxWidth}
	m.lg = lipgloss.DefaultRenderer()
	m.styles = initStyles(m.lg)
	m.header = CreateHeader()
	m.menu = CreateMenu()
	m.body = resources.CreateResource()
	m.status = CreateStatusBar()
	return m
}

// Styles
func initStyles(lg *lipgloss.Renderer) *Styles {
	s := Styles{}
	s.Base = lg.NewStyle().
		Padding(0, 0, 0, 0)
	return &s
}

// Init
func (m LayoutModel) Init() tea.Cmd {
	var cmds []tea.Cmd
	return tea.Batch(cmds...)
}

// Update
func (m LayoutModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		}
	}
	var cmds []tea.Cmd

	headerModel, cmd := m.header.Update(msg)
	m.header = headerModel
	cmds = append(cmds, cmd)

	menuModel, cmd := m.menu.Update(msg)
	m.menu = menuModel
	cmds = append(cmds, cmd)

	statusModel, cmd := m.status.Update(msg)
	m.status = statusModel
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

// View
func (m LayoutModel) View() string {
	styles := m.styles
	body := lipgloss.JoinHorizontal(lipgloss.Top, m.body.View())
	return styles.Base.Render(lipgloss.JoinVertical(
		lipgloss.Top,
		m.header.View(),
		m.menu.View(),
		body,
		m.status.View(),
	))
}
