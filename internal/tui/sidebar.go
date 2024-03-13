package tui

import (
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	width = 40
)

type SidebarModel struct {
	styles   *SidebarStyles
	width    int
	margin   int
	renderer *lipgloss.Renderer
	list     list.Model
}

type SidebarStyles struct {
	Sidebar,
	List lipgloss.Style
}

type itemDelegate struct{}

func (i item) Title() string       { return i.title }
func (i item) Description() string { return i.desc }
func (i item) FilterValue() string { return i.title }

func CreateSideBar() SidebarModel {
	m := SidebarModel{
		width: width,
	}
	items := []list.Item{
		item{title: "2", desc: "I have ’em all over my house"},
		item{title: "3", desc: "It's good on toast"},
		item{title: "4", desc: "It cools you down"},
		item{title: "5", desc: "And by that I mean socks without holes"},
	}
	m.list = list.New(items, list.DefaultDelegate{}, 0, 10)
	m.renderer = lipgloss.DefaultRenderer()
	m.styles = m.initStyles(m.renderer)
	return m
}

func (m SidebarModel) initStyles(lg *lipgloss.Renderer) *SidebarStyles {
	s := SidebarStyles{}

	s.Sidebar = lg.NewStyle().
		Border(lipgloss.NormalBorder(), false, true, false, false).
		BorderForeground(indigo).
		PaddingRight(1).
		PaddingLeft(1)

	return &s
}

func (m SidebarModel) WidthMargin(h int) SidebarModel {
	m.margin = h
	return m
}

func (m SidebarModel) Init() tea.Cmd {
	var cmds []tea.Cmd

	return tea.Batch(cmds...)
}

func (m SidebarModel) Update(msg tea.Msg) (SidebarModel, tea.Cmd) {
	var cmds []tea.Cmd
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.list.SetSize(m.width, msg.Height-m.margin)
	case tea.KeyMsg:
		if m.list.FilterState() == list.Filtering {
			break
		}
	}
	return m, tea.Batch(cmds...)
}

func (m SidebarModel) View() string {
	return m.styles.Sidebar.Width(m.width).Render(m.list.View())
}
