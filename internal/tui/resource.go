package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
)

type ResourceModel struct {
	lg      *lipgloss.Renderer
	styles  *Styles
	groups  map[string]*huh.Group
	view    *huh.Group
	theme   *huh.Theme
	keymap  *huh.KeyMap
	width   int
	sidebar list.Model
	menu    MenuModel
}

type Styles struct {
	Base,
	HeaderText,
	Menu,
	Sidebar,
	StatusHeader,
	Highlight,
	ErrorHeaderText,
	Help lipgloss.Style
}

type item struct {
	title, desc string
}

type itemDelegate struct{}

const maxWidth = 200

var (
	appStyle = lipgloss.NewStyle().Padding(1, 2)

	titleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFDF5")).
			Background(lipgloss.Color("#25A065")).
			Padding(0, 1)

	statusMessageStyle = lipgloss.NewStyle().
				Foreground(lipgloss.AdaptiveColor{Light: "#04B575", Dark: "#04B575"}).
				Render
	indigo = lipgloss.AdaptiveColor{Light: "#5A56E0", Dark: "#7571F9"}
)

func (i item) Title() string       { return i.title }
func (i item) Description() string { return i.desc }
func (i item) FilterValue() string { return i.title }

func initStyles(lg *lipgloss.Renderer) *Styles {
	s := Styles{}
	s.Base = lg.NewStyle().
		Padding(0, 0, 0, 0)
	s.HeaderText = lg.NewStyle().
		Foreground(indigo).
		Bold(true).
		Padding(0, 1, 0, 2)
	s.Menu = lg.NewStyle().
		Foreground(indigo).
		Bold(true).
		Padding(0, 1, 0, 2)
	s.Sidebar = lg.NewStyle().
		Border(lipgloss.ThickBorder(), false, true, false, false).
		BorderForeground(indigo).
		PaddingRight(1)

	return &s
}

func Create() ResourceModel {
	m := ResourceModel{width: maxWidth}
	m.lg = lipgloss.DefaultRenderer()
	m.styles = initStyles(m.lg)
	m.theme = huh.ThemeCharm()
	m.keymap = huh.NewDefaultKeyMap()

	items := []list.Item{
		item{title: "2", desc: "I have ’em all over my house"},
		item{title: "3", desc: "It's good on toast"},
		item{title: "4", desc: "It cools you down"},
		item{title: "5", desc: "And by that I mean socks without holes"},
	}

	m.sidebar = list.New(items, list.DefaultDelegate{}, 0, 14)
	m.menu = CreateMenu(4)
	return m
}

func (m ResourceModel) Init() tea.Cmd {
	var cmds []tea.Cmd

	for _, group := range m.groups {
		group.WithTheme(m.theme)
		group.WithKeyMap(m.keymap)
		cmd := group.Init()
		cmds = append(cmds, cmd)
	}
	return tea.Batch(cmds...)
}

func (m ResourceModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {

	// huh.Form
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		h, v := appStyle.GetFrameSize()
		m.sidebar.SetSize(msg.Width-h, msg.Height-v-m.menu.Height)
	case tea.KeyMsg:
		// Don't match any of the keys below if we're actively filtering.
		if m.sidebar.FilterState() == list.Filtering {
			break
		}
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		}
	}
	var cmds []tea.Cmd

	newListModel, cmd := m.sidebar.Update(msg)
	m.sidebar = newListModel
	cmds = append(cmds, cmd)

	// for groupPath, group := range m.groups {
	// 	if groupPath == path {
	// 		model, cmd := group.Update(msg)
	// 		if g, ok := model.(*huh.Group); ok {
	// 			m.view = g
	// 			cmds = append(cmds, cmd)
	// 		}
	// 	}
	// }

	return m, tea.Batch(cmds...)
}

func (m ResourceModel) View() string {
	styles := m.styles
	v := ""
	if m.view != nil {
		v = strings.TrimSuffix(m.view.View(), "\n\n")
	}

	form := m.lg.NewStyle().Render(v)

	var sidebar string
	{
		sidebar = styles.Sidebar.Copy().
			Height(lipgloss.Height(form)).
			Width(28).
			MarginRight(1).
			Render(m.sidebar.View())
	}

	header := styles.HeaderText.Copy().Render("KNDP")
	menu := styles.HeaderText.Copy().Render(m.menu.View())
	body := lipgloss.JoinHorizontal(lipgloss.Top, sidebar, form)
	footer := lipgloss.JoinHorizontal(lipgloss.Bottom, "Footer")
	return styles.Base.Render(lipgloss.JoinVertical(lipgloss.Top, header, menu, body, footer))
}
