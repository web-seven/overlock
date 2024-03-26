package resources

import (
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	width  = 40
	indigo = lipgloss.AdaptiveColor{Light: "#5A56E0", Dark: "#7571F9"}
)

type SchemaTreeModel struct {
	styles   *SchemaTreeStyles
	width    int
	margin   int
	renderer *lipgloss.Renderer
	list     list.Model
}

type SchemaTreeStyles struct {
	SchemaTree,
	List lipgloss.Style
}

type item struct {
	title, desc string
}

func (i item) Title() string       { return i.title }
func (i item) Description() string { return i.desc }
func (i item) FilterValue() string { return i.title }

// Model
func CreateSchemaTree() SchemaTreeModel {
	m := SchemaTreeModel{
		width: width,
	}
	items := []list.Item{
		item{title: "2", desc: "I have ’em all over my house"},
		item{title: "3", desc: "It's good on toast"},
		item{title: "4", desc: "It cools you down"},
		item{title: "5", desc: "And by that I mean socks without holes"},
	}
	m.list = list.New(items, list.NewDefaultDelegate(), 0, 10)
	m.list.Title = "Lock"
	m.list.DisableQuitKeybindings()
	m.list.SetShowStatusBar(false)
	m.list.SetShowPagination(false)
	m.list.SetShowHelp(false)
	m.list.SetShowHelp(false)
	m.renderer = lipgloss.DefaultRenderer()
	m.styles = m.initStyles(m.renderer)
	return m
}

// Styles
func (m SchemaTreeModel) initStyles(lg *lipgloss.Renderer) *SchemaTreeStyles {
	s := SchemaTreeStyles{}

	s.SchemaTree = lg.NewStyle().
		Border(lipgloss.NormalBorder(), false, true, false, false).
		BorderForeground(indigo).
		PaddingRight(1).
		PaddingLeft(1)

	return &s
}

// Margin
func (m SchemaTreeModel) WidthMargin(h int) SchemaTreeModel {
	m.margin = h
	return m
}

// Init
func (m SchemaTreeModel) Init() tea.Cmd {
	var cmds []tea.Cmd

	return tea.Batch(cmds...)
}

// Update
func (m SchemaTreeModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.list.SetSize(m.width, msg.Height-m.margin)
	case tea.KeyMsg:
		switch keypress := msg.String(); keypress {

		case "enter":
			i, ok := m.list.SelectedItem().(item)
			if ok {
				m.list.Title = i.title
			}
		}
	}

	newListModel, cmd := m.list.Update(msg)
	m.list = newListModel
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

// View
func (m SchemaTreeModel) View() string {
	return m.styles.SchemaTree.Width(m.width).Render(m.list.View())
}
