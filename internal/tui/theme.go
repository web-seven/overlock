package tui

import (
	"github.com/charmbracelet/lipgloss"
)

// Theme defines the color scheme and styling for the TUI
type Theme struct {
	Primary    lipgloss.Color
	Secondary  lipgloss.Color
	Accent     lipgloss.Color
	Success    lipgloss.Color
	Warning    lipgloss.Color
	Error      lipgloss.Color
	Muted      lipgloss.Color
	Background lipgloss.Color
	Foreground lipgloss.Color
	Border     lipgloss.Color
	SelectedBg lipgloss.Color
	SelectedFg lipgloss.Color
	DisabledFg lipgloss.Color
}

// DefaultTheme returns the default Overlock theme
func DefaultTheme() Theme {
	return Theme{
		Primary:    lipgloss.Color("#5FAFFF"), // Light blue
		Secondary:  lipgloss.Color("#3A7CA5"), // Medium blue
		Accent:     lipgloss.Color("#5FAFFF"), // Light blue accent
		Success:    lipgloss.Color("#5FAF87"), // Green
		Warning:    lipgloss.Color("#D7AF5F"), // Yellow
		Error:      lipgloss.Color("#D75F5F"), // Red
		Muted:      lipgloss.Color("#6C7086"), // Gray
		Background: lipgloss.Color("#0D1117"), // Dark blue background
		Foreground: lipgloss.Color("#C9D1D9"), // Light gray text
		Border:     lipgloss.Color("#7D56F4"), // Purple highlight border (like lipgloss tabs)
		SelectedBg: lipgloss.Color("#1F4A6F"), // Selected blue background
		SelectedFg: lipgloss.Color("#FFFFFF"), // White text when selected
		DisabledFg: lipgloss.Color("#484F58"), // Dark gray for disabled
	}
}

// Styles contains all pre-configured lipgloss styles for the TUI
type Styles struct {
	Theme Theme

	// Title styles
	TitleStyle    lipgloss.Style
	SubtitleStyle lipgloss.Style
	HeaderStyle   lipgloss.Style

	// Menu styles
	MenuStyle         lipgloss.Style
	MenuItemStyle     lipgloss.Style
	SelectedItemStyle lipgloss.Style
	DisabledItemStyle lipgloss.Style

	// Container styles
	BoxStyle     lipgloss.Style
	BorderStyle  lipgloss.Style
	PanelStyle   lipgloss.Style
	ContentStyle lipgloss.Style

	// Text styles
	TextStyle      lipgloss.Style
	MutedTextStyle lipgloss.Style
	ErrorStyle     lipgloss.Style
	SuccessStyle   lipgloss.Style
	WarningStyle   lipgloss.Style

	// Status styles
	StatusBarStyle lipgloss.Style
	HelpStyle      lipgloss.Style
	KeyStyle       lipgloss.Style
	ValueStyle     lipgloss.Style

	// Badge styles
	BadgeStyle        lipgloss.Style
	BadgePrimaryStyle lipgloss.Style
	BadgeSuccessStyle lipgloss.Style
}

// NewStyles creates a new Styles instance with the default theme
func NewStyles() Styles {
	theme := DefaultTheme()

	return Styles{
		Theme: theme,

		// Title styles
		TitleStyle: lipgloss.NewStyle().
			Foreground(theme.Foreground).
			Bold(false),

		SubtitleStyle: lipgloss.NewStyle().
			Foreground(theme.Foreground).
			Bold(false),

		HeaderStyle: lipgloss.NewStyle().
			Foreground(theme.Primary).
			Bold(true).
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(theme.Border).
			Padding(0, 2).
			MarginBottom(1).
			Align(lipgloss.Center),

		// Menu styles
		MenuStyle: lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(theme.Border).
			Padding(1, 1),

		MenuItemStyle: lipgloss.NewStyle().
			Foreground(theme.Foreground).
			Padding(0, 2),

		SelectedItemStyle: lipgloss.NewStyle().
			Foreground(theme.SelectedFg).
			Background(theme.SelectedBg).
			Bold(true).
			Padding(0, 2),

		DisabledItemStyle: lipgloss.NewStyle().
			Foreground(theme.DisabledFg).
			Padding(0, 2),

		// Container styles
		BoxStyle: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(theme.Border).
			Padding(1, 2),

		BorderStyle: lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(theme.Border),

		PanelStyle: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(theme.Border).
			Padding(1, 2).
			MarginBottom(1),

		ContentStyle: lipgloss.NewStyle().
			Padding(0, 2),

		// Text styles
		TextStyle: lipgloss.NewStyle().
			Foreground(theme.Foreground),

		MutedTextStyle: lipgloss.NewStyle().
			Foreground(theme.Muted),

		ErrorStyle: lipgloss.NewStyle().
			Foreground(theme.Error).
			Bold(true),

		SuccessStyle: lipgloss.NewStyle().
			Foreground(theme.Success).
			Bold(true),

		WarningStyle: lipgloss.NewStyle().
			Foreground(theme.Warning).
			Bold(true),

		// Status styles
		StatusBarStyle: lipgloss.NewStyle().
			Foreground(theme.Muted).
			Padding(0, 1),

		HelpStyle: lipgloss.NewStyle().
			Foreground(theme.Muted).
			Padding(1, 2),

		KeyStyle: lipgloss.NewStyle().
			Foreground(theme.Primary).
			Bold(true),

		ValueStyle: lipgloss.NewStyle().
			Foreground(theme.Foreground),

		// Badge styles
		BadgeStyle: lipgloss.NewStyle().
			Foreground(theme.Foreground).
			Background(theme.Muted).
			Padding(0, 1).
			Bold(true),

		BadgePrimaryStyle: lipgloss.NewStyle().
			Foreground(theme.SelectedFg).
			Background(theme.Primary).
			Padding(0, 1).
			Bold(true),

		BadgeSuccessStyle: lipgloss.NewStyle().
			Foreground(theme.SelectedFg).
			Background(theme.Success).
			Padding(0, 1).
			Bold(true),
	}
}
