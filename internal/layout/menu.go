package layout

import (
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"golang.org/x/term"
)

type MenuModel struct {
}

var (
	highlight = lipgloss.AdaptiveColor{Light: "#874BFD", Dark: "#7D56F4"}

	activeTabBorder = lipgloss.Border{
		Top:         "─",
		Bottom:      " ",
		Left:        "│",
		Right:       "│",
		TopLeft:     "╭",
		TopRight:    "╮",
		BottomLeft:  "┘",
		BottomRight: "└",
	}

	tabBorder = lipgloss.Border{
		Top:         "─",
		Bottom:      "─",
		Left:        "│",
		Right:       "│",
		TopLeft:     "╭",
		TopRight:    "╮",
		BottomLeft:  "┴",
		BottomRight: "┴",
	}

	tab = lipgloss.NewStyle().
		Border(tabBorder, true).
		BorderForeground(highlight).
		Padding(0, 0)

	activeTab = tab.Copy().Border(activeTabBorder, true)

	tabGap = tab.Copy().
		BorderTop(false).
		BorderLeft(false).
		BorderRight(false)
)

// Model
func CreateMenu() MenuModel {
	m := MenuModel{}
	return m
}

// Init
func (m MenuModel) Init() tea.Cmd {
	var cmds []tea.Cmd
	return tea.Batch(cmds...)
}

// Update
func (m MenuModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	return m, tea.Batch(cmds...)
}

// View
func (m MenuModel) View() string {

	physicalWidth, _, _ := term.GetSize(int(os.Stdout.Fd()))

	row := lipgloss.JoinHorizontal(
		lipgloss.Top,
		activeTab.Render("Resources"),
		tab.Render("Configurations"),
		tab.Render("Functions"),
		tab.Render("Providers"),
		tab.Render("About"),
	)
	gap := tabGap.Render(strings.Repeat(" ", max(0, physicalWidth-lipgloss.Width(row))))
	row = lipgloss.JoinHorizontal(lipgloss.Bottom, row, gap)
	return row
}
