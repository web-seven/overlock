package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type HeaderModel struct {
	renderer *lipgloss.Renderer
	styles   *HeaderStyles
}

type HeaderStyles struct {
	HeaderText lipgloss.Style
}

func CreateHeaderModel() HeaderModel {
	m := HeaderModel{}
	m.renderer = lipgloss.DefaultRenderer()
	m.styles = m.initStyles(m.renderer)
	return m
}

func (m HeaderModel) initStyles(lg *lipgloss.Renderer) *HeaderStyles {
	s := HeaderStyles{}
	s.HeaderText = lg.NewStyle().
		Foreground(indigo).
		Bold(true).
		Padding(0, 1, 0, 2)
	return &s
}

func (m HeaderModel) Init() tea.Cmd {
	var cmds []tea.Cmd
	return tea.Batch(cmds...)
}

func (m HeaderModel) Update(tea.Msg) (HeaderModel, tea.Cmd) {
	var cmds []tea.Cmd
	return m, tea.Batch(cmds...)
}

func (m HeaderModel) View() string {
	return m.styles.HeaderText.Copy().Render("KNDP")
}
