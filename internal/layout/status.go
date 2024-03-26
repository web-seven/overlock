package layout

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type StatusModel struct {
	styles   *StatusStyles
	width    int
	renderer *lipgloss.Renderer
}

type StatusStyles struct {
	StatusNugget,
	EncodingStyle,
	FishCakeStyle,
	StatusBarStyle,
	StatusStyle,
	StatusText lipgloss.Style
}

// Model
func CreateStatusBar() StatusModel {
	w, _ := appStyle.GetFrameSize()
	m := StatusModel{
		width: w,
	}
	m.renderer = lipgloss.DefaultRenderer()
	m.styles = m.initStyles(m.renderer)
	return m
}

// Styles
func (m StatusModel) initStyles(lg *lipgloss.Renderer) *StatusStyles {
	s := StatusStyles{}
	s.StatusNugget = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FFFDF5")).
		Padding(0, 1)

	s.StatusBarStyle = lipgloss.NewStyle().
		Foreground(lipgloss.AdaptiveColor{Light: "#343433", Dark: "#C1C6B2"}).
		Background(lipgloss.AdaptiveColor{Light: "#D9DCCF", Dark: "#353533"})

	s.StatusStyle = lipgloss.NewStyle().
		Inherit(s.StatusBarStyle).
		Foreground(lipgloss.Color("#FFFDF5")).
		Background(lipgloss.Color("#FF5F87")).
		Padding(0, 1).
		MarginRight(1)

	s.EncodingStyle = s.StatusNugget.Copy().
		Background(lipgloss.Color("#A550DF")).
		Align(lipgloss.Right)

	s.StatusText = lipgloss.NewStyle().Inherit(s.StatusBarStyle)

	s.FishCakeStyle = s.StatusNugget.Copy().Background(lipgloss.Color("#6124DF"))

	return &s
}

// Init
func (m StatusModel) Init() tea.Cmd {
	var cmds []tea.Cmd
	return tea.Batch(cmds...)
}

// Update
func (m StatusModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
	}
	return m, tea.Batch(cmds...)
}

// View
func (m StatusModel) View() string {
	var bar string
	{
		w := lipgloss.Width

		statusKey := m.styles.StatusStyle.Render("CONTEXT")
		encoding := m.styles.EncodingStyle.Render("UTF-8")
		fishCake := m.styles.FishCakeStyle.Render("🍥 Fish Cake")
		statusVal := m.styles.StatusText.Copy().
			Width(m.width - w(statusKey) - w(encoding) - w(fishCake)).
			Render("kind-local (dummy)")

		bar = lipgloss.JoinHorizontal(lipgloss.Top,
			statusKey,
			statusVal,
			encoding,
			fishCake,
		)

	}

	return m.styles.StatusBarStyle.Width(m.width).Render(bar)
}
