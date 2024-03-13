package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/kndpio/kndp/internal/globals"
)

type HeaderModel struct {
	version  string
	styles   *HeaderStyles
	width    int
	renderer *lipgloss.Renderer
}

type HeaderStyles struct {
	HeaderNugget,
	EncodingStyle,
	FishCakeStyle,
	HeaderBarStyle,
	HeaderStyle,
	HeaderText lipgloss.Style
}

func CreateHeader() HeaderModel {
	w, _ := appStyle.GetFrameSize()
	m := HeaderModel{
		width: w,
	}
	m.renderer = lipgloss.DefaultRenderer()
	m.styles = m.initStyles(m.renderer)
	m.version = globals.Version
	return m
}

func (m HeaderModel) initStyles(lg *lipgloss.Renderer) *HeaderStyles {
	s := HeaderStyles{}
	s.HeaderNugget = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FFFDF5")).
		Padding(0, 1)

	s.HeaderBarStyle = lipgloss.NewStyle().
		Foreground(lipgloss.AdaptiveColor{Light: "#343433", Dark: "#C1C6B2"}).
		Background(lipgloss.AdaptiveColor{Light: "#D9DCCF", Dark: "#353533"}).
		MarginBottom(1)

	s.HeaderStyle = lipgloss.NewStyle().
		Inherit(s.HeaderBarStyle).
		Foreground(lipgloss.Color("#FFFDF5")).
		Background(lipgloss.Color("#FF5F87")).
		Padding(0, 1).
		MarginRight(1)

	s.EncodingStyle = s.HeaderNugget.Copy().
		Background(lipgloss.Color("#A550DF")).
		Align(lipgloss.Right)

	s.HeaderText = lipgloss.NewStyle().Inherit(s.HeaderBarStyle)

	s.FishCakeStyle = s.HeaderNugget.Copy().Background(lipgloss.Color("#6124DF"))

	return &s
}

func (m HeaderModel) Init() tea.Cmd {
	var cmds []tea.Cmd
	return tea.Batch(cmds...)
}

func (m HeaderModel) Update(msg tea.Msg) (HeaderModel, tea.Cmd) {
	var cmds []tea.Cmd
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
	}
	return m, tea.Batch(cmds...)
}

func (m HeaderModel) View() string {
	var bar string
	{
		w := lipgloss.Width

		HeaderKey := m.styles.HeaderStyle.Render("KNDP")
		encoding := m.styles.EncodingStyle.Render("UTF-8")
		fishCake := m.styles.FishCakeStyle.Render("🍥 Fish Cake")
		HeaderVal := m.styles.HeaderText.Copy().
			Width(m.width - w(HeaderKey) - w(encoding) - w(fishCake)).
			Render(m.version)

		bar = lipgloss.JoinHorizontal(lipgloss.Top,
			HeaderKey,
			HeaderVal,
			encoding,
			fishCake,
		)

	}

	return m.styles.HeaderBarStyle.Width(m.width).Render(bar)
}
