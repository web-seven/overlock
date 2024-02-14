package ui

import (
	"context"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/pterm/pterm"
	"k8s.io/client-go/dynamic"
)

type Cmd struct{}

type model struct {
	err error
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {

	return m, tea.Quit
}

func (m model) View() string {
	if m.err != nil {
		return fmt.Sprintf("\nWe had some trouble: %v\n\n", m.err)
	}

	s := "No supported commands yet..."

	return "\n" + s + "\n\n"
}

func (c *Cmd) Run(ctx context.Context, p pterm.TextPrinter, client *dynamic.DynamicClient) error {
	if _, err := tea.NewProgram(model{}).Run(); err != nil {
		fmt.Printf("Uh oh, there was an error: %v\n", err)
		os.Exit(1)
	}
	return nil
}
