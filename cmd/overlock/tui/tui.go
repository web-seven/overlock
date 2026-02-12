package tui

import (
	"context"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/web-seven/overlock/internal/tui"
	"go.uber.org/zap"
)

// TUICmd represents the TUI command
type TUICmd struct{}

// Run executes the TUI command
func (c *TUICmd) Run(ctx context.Context, logger *zap.SugaredLogger) error {
	// Create the app model
	model := tui.NewAppModel(logger)

	// Create the Bubble Tea program
	p := tea.NewProgram(model, tea.WithAltScreen())

	// Set the program reference for log messages
	model.SetProgram(p)

	// Run the program
	finalModel, err := p.Run()
	if err != nil {
		return fmt.Errorf("failed to run TUI: %w", err)
	}

	// Check if a menu item was selected
	if m, ok := finalModel.(*tui.AppModel); ok {
		selectedItem := m.GetSelectedItem()
		if selectedItem != "" {
			// Print the selected command for the user to see
			fmt.Fprintf(os.Stderr, "\nSelected: %s\n", selectedItem)
			fmt.Fprintf(os.Stderr, "To use this feature, run: overlock %s --help\n", selectedItem)
		}
	}

	return nil
}
