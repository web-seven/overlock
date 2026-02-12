package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/web-seven/overlock/cmd/overlock/version"
	"go.uber.org/zap"
)

const (
	menuWidth   = 30
	logWidth    = 50
	minLogLines = 10
)

// AppModel represents the main TUI application model
type AppModel struct {
	list         list.Model
	viewport     viewport.Model
	logViewport  viewport.Model
	ready        bool
	width        int
	height       int
	styles       Styles
	quitting     bool
	selectedItem string
	envModel     *EnvironmentModel
	logger       *zap.SugaredLogger
	logBuffer    *LogBuffer
	tuiLogger    *zap.SugaredLogger
	inSubView    bool
	showLogs     bool
}

// NewAppModel creates a new app model with the given items
func NewAppModel(logger *zap.SugaredLogger) *AppModel {
	items := DefaultMenuItems()

	// Create list with default delegate
	delegate := list.NewDefaultDelegate()
	l := list.New(items, delegate, 0, 0)
	l.Title = ""
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.SetShowHelp(false)
	l.SetShowTitle(false)

	styles := NewStyles()

	// Create log buffer and TUI logger
	logBuffer := NewLogBuffer()
	tuiLogger := CreateTUILogger(logBuffer)

	return &AppModel{
		list:      l,
		styles:    styles,
		quitting:  false,
		ready:     false,
		logger:    logger,
		logBuffer: logBuffer,
		tuiLogger: tuiLogger,
		inSubView: false,
		showLogs:  false,
	}
}

// Init initializes the app model
func (m *AppModel) Init() tea.Cmd {
	return nil
}

// Update handles messages and updates the model
func (m *AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		headerHeight := lipgloss.Height(m.headerView())
		footerHeight := lipgloss.Height(m.footerView())
		verticalMarginHeight := headerHeight + footerHeight

		// Calculate content width based on whether logs are shown
		contentWidth := m.width - menuWidth - 1
		if m.showLogs {
			contentWidth = contentWidth - logWidth - 1
		}
		if contentWidth < 20 {
			contentWidth = 20
		}
		contentHeight := m.height - verticalMarginHeight
		if contentHeight < 1 {
			contentHeight = 1
		}

		// Update list size
		m.list.SetSize(menuWidth-2, contentHeight)

		if !m.ready {
			// Initialize viewports
			m.viewport = viewport.New(contentWidth, contentHeight)
			m.viewport.SetContent("")
			m.logViewport = viewport.New(logWidth-2, contentHeight)
			m.logViewport.SetContent("")
			m.ready = true
		} else {
			m.viewport.Width = contentWidth
			m.viewport.Height = contentHeight
			m.logViewport.Width = logWidth - 2
			m.logViewport.Height = contentHeight
		}

		return m, nil

	case LogMsg:
		// Update log viewport with new log entry
		m.updateLogView()
		return m, nil

	case tea.KeyMsg:
		// If we're in a sub-view, handle escape to go back
		if m.inSubView {
			switch msg.String() {
			case "esc":
				// Only go back if we're in the list view of the sub-model
				if m.envModel != nil && m.envModel.currentView == ViewList {
					m.inSubView = false
					m.showLogs = false
					m.envModel = nil
					m.selectedItem = ""
					// Recalculate layout without logs
					m.viewport.Width = m.width - menuWidth - 1
					return m, nil
				}
			case "q", "ctrl+c":
				m.quitting = true
				return m, tea.Quit
			}

			// Forward to sub-view
			if m.envModel != nil {
				var envCmd tea.Cmd
				m.envModel, envCmd = m.envModel.Update(msg)
				if envCmd != nil {
					cmds = append(cmds, envCmd)
				}
				// Update viewport with environment view
				m.viewport.SetContent(m.envModel.View())
			}

			return m, tea.Batch(cmds...)
		}

		// Main menu navigation
		switch msg.String() {
		case "q", "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		case "enter":
			if item, ok := m.list.SelectedItem().(MenuItem); ok {
				m.selectedItem = item.Title()
				// Handle different menu items
				switch item.Title() {
				case "Environment":
					m.inSubView = true
					m.showLogs = true
					m.logBuffer.Clear() // Clear previous logs
					m.envModel = NewEnvironmentModel(m.tuiLogger, m.styles)
					m.envModel.width = m.viewport.Width
					m.envModel.height = m.viewport.Height
					var envCmd tea.Cmd
					envCmd = m.envModel.Init()
					if envCmd != nil {
						cmds = append(cmds, envCmd)
					}
					m.viewport.SetContent(m.envModel.View())
					// Recalculate layout with logs visible
					m.viewport.Width = m.width - menuWidth - logWidth - 2
					m.logViewport.Width = logWidth - 2
				default:
					m.viewport.SetContent(item.Title())
				}
			}
		}
	}

	// Update sub-models if active
	if m.inSubView && m.envModel != nil {
		var envCmd tea.Cmd
		m.envModel, envCmd = m.envModel.Update(msg)
		if envCmd != nil {
			cmds = append(cmds, envCmd)
		}
		m.viewport.SetContent(m.envModel.View())
	} else {
		// Update list
		var cmd tea.Cmd
		m.list, cmd = m.list.Update(msg)
		cmds = append(cmds, cmd)

		// Update viewport
		m.viewport, cmd = m.viewport.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

// View renders the app
func (m *AppModel) View() string {
	if m.quitting {
		return ""
	}

	if !m.ready {
		return "\n  Initializing..."
	}

	doc := strings.Builder{}

	// Header
	doc.WriteString(m.headerView())
	doc.WriteString("\n")

	// Main content area (menu + content + optional logs)
	{
		borderStyle := lipgloss.NewStyle().Foreground(m.styles.Theme.Border)

		// Menu sidebar style (no border, we add it manually)
		menuStyle := lipgloss.NewStyle().
			Width(menuWidth - 2).
			Height(m.viewport.Height)

		// Content area style
		contentStyle := lipgloss.NewStyle().
			Width(m.viewport.Width).
			Height(m.viewport.Height)

		// Log area style
		logStyle := lipgloss.NewStyle().
			Width(logWidth - 2).
			Height(m.viewport.Height)

		// Render menu and content
		menuRendered := menuStyle.Render(m.list.View())
		contentRendered := contentStyle.Render(m.viewport.View())

		// Render logs if enabled
		var logRendered string
		if m.showLogs {
			m.updateLogView()
			logTitle := m.styles.TextStyle.Bold(true).Foreground(m.styles.Theme.Primary).Render("Logs")
			logContent := m.logViewport.View()
			logRendered = logStyle.Render(logTitle + "\n" + logContent)
		}

		// Split into lines
		menuLines := strings.Split(menuRendered, "\n")
		contentLines := strings.Split(contentRendered, "\n")
		var logLines []string
		if m.showLogs {
			logLines = strings.Split(logRendered, "\n")
		}

		// Ensure same number of lines
		maxLines := len(menuLines)
		if len(contentLines) > maxLines {
			maxLines = len(contentLines)
		}
		if m.showLogs && len(logLines) > maxLines {
			maxLines = len(logLines)
		}

		// Build bordered lines with proper alignment
		var borderedLines []string
		for i := 0; i < maxLines; i++ {
			menuLine := ""
			if i < len(menuLines) {
				menuLine = menuLines[i]
			}
			contentLine := ""
			if i < len(contentLines) {
				contentLine = contentLines[i]
			}
			// Pad menu line to exact width
			menuLine = padRight(menuLine, menuWidth-2)

			line := borderStyle.Render("│") + menuLine + borderStyle.Render("│") + contentLine

			// Add log section if enabled
			if m.showLogs {
				logLine := ""
				if i < len(logLines) {
					logLine = logLines[i]
				}
				logLine = padRight(logLine, logWidth-2)
				line = line + borderStyle.Render("│") + logLine
			}

			line = line + borderStyle.Render("│")
			borderedLines = append(borderedLines, line)
		}
		doc.WriteString(strings.Join(borderedLines, "\n"))
		doc.WriteString("\n")
	}

	// Footer
	doc.WriteString(m.footerView())

	return doc.String()
}

// headerView renders the header section
func (m *AppModel) headerView() string {
	borderStyle := lipgloss.NewStyle().Foreground(m.styles.Theme.Border)

	// Top border with corners
	topLine := borderStyle.Render("╭" + strings.Repeat("─", max(0, m.width-2)) + "╮")

	// Title with side borders
	titleText := "Overlock - Crossplane Environment CLI"
	titleWidth := max(0, m.width-2)
	titleLine := borderStyle.Render("│") +
		lipgloss.NewStyle().Foreground(m.styles.Theme.Foreground).
			Width(titleWidth).
			Align(lipgloss.Center).
			Render(titleText) +
		borderStyle.Render("│")

	// Line below title with junctions (menuWidth-2 to align with menu separator)
	innerWidth := m.width - 2
	leftPart := strings.Repeat("─", menuWidth-2)
	junction := "┬"

	var bottomLine string
	if m.showLogs {
		// Calculate middle part width (content area)
		middleWidth := innerWidth - menuWidth - logWidth + 2
		if middleWidth < 0 {
			middleWidth = 0
		}
		middlePart := strings.Repeat("─", middleWidth)
		rightPart := strings.Repeat("─", logWidth-2)
		bottomLine = borderStyle.Render("├" + leftPart + junction + middlePart + junction + rightPart + "┤")
	} else {
		rightPart := strings.Repeat("─", max(0, innerWidth-menuWidth+1))
		bottomLine = borderStyle.Render("├" + leftPart + junction + rightPart + "┤")
	}

	return lipgloss.JoinVertical(lipgloss.Left, topLine, titleLine, bottomLine)
}

// updateLogView updates the log viewport with current log entries
func (m *AppModel) updateLogView() {
	if m.logBuffer == nil {
		return
	}

	entries := m.logBuffer.GetEntries()
	var logContent strings.Builder

	// Render log entries
	for _, entry := range entries {
		var levelStyle lipgloss.Style
		switch entry.Level.String() {
		case "ERROR", "error":
			levelStyle = m.styles.ErrorStyle
		case "WARN", "warn":
			levelStyle = m.styles.WarningStyle
		case "INFO", "info":
			levelStyle = m.styles.SuccessStyle
		default:
			levelStyle = m.styles.MutedTextStyle
		}

		logLine := lipgloss.JoinHorizontal(
			lipgloss.Left,
			m.styles.MutedTextStyle.Render("["+entry.Time+"] "),
			levelStyle.Render(entry.Level.CapitalString()+" "),
			m.styles.TextStyle.Render(entry.Message),
		)
		logContent.WriteString(logLine)
		logContent.WriteString("\n")
	}

	m.logViewport.SetContent(logContent.String())
	// Auto-scroll to bottom
	m.logViewport.GotoBottom()
}

// footerView renders the footer with shortcuts and status
func (m *AppModel) footerView() string {
	w := lipgloss.Width

	// Status bar base style
	statusBarStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#C1C6B2")).
		Background(lipgloss.Color("#353533"))

	// Nugget style (colored sections)
	statusNugget := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FFFDF5")).
		Padding(0, 1)

	// Shortcuts style (left side with pink background)
	shortcutsStyle := statusNugget.
		Background(lipgloss.Color("#FF5F87")).
		MarginRight(1)

	// Namespace style (purple background)
	namespaceStyle := statusNugget.
		Background(lipgloss.Color("#A550DF"))

	// Release style (deep purple background)
	releaseStyle := statusNugget.
		Background(lipgloss.Color("#6124DF"))

	// Version style
	versionStyle := statusNugget.
		Background(lipgloss.Color("#874BFD"))

	// Build shortcuts
	shortcuts := "[a] Actions  [/] Search  [?] Help  [q] Quit"
	shortcutsContent := shortcutsStyle.Render(shortcuts)

	// Status info
	versionNum := version.Version
	if versionNum == "" {
		versionNum = "dev"
	}

	namespaceContent := namespaceStyle.Render("Namespace: demo")
	releaseContent := releaseStyle.Render("Release: crossplane")
	versionContent := versionStyle.Render("Version: " + versionNum)

	// Status text (flexible middle section)
	statusText := lipgloss.NewStyle().
		Inherit(statusBarStyle).
		Padding(0, 1)

	// Calculate remaining width for middle section
	middleWidth := m.width - w(shortcutsContent) - w(namespaceContent) - w(releaseContent) - w(versionContent)
	if middleWidth < 0 {
		middleWidth = 0
	}

	middleContent := statusText.Width(middleWidth).Render("")

	// Join all parts horizontally
	bar := lipgloss.JoinHorizontal(
		lipgloss.Top,
		shortcutsContent,
		middleContent,
		namespaceContent,
		releaseContent,
		versionContent,
	)

	// Create border line with junction where menu border connects (menuWidth-2 to align)
	borderStyle := lipgloss.NewStyle().Foreground(m.styles.Theme.Border)
	innerWidth := m.width - 2
	leftPart := strings.Repeat("─", menuWidth-2)
	junction := "┴"

	var topLine string
	if m.showLogs {
		// Calculate middle part width (content area)
		middleWidth := innerWidth - menuWidth - logWidth + 2
		if middleWidth < 0 {
			middleWidth = 0
		}
		middlePart := strings.Repeat("─", middleWidth)
		rightPart := strings.Repeat("─", logWidth-2)
		topLine = borderStyle.Render("├" + leftPart + junction + middlePart + junction + rightPart + "┤")
	} else {
		rightPart := strings.Repeat("─", max(0, innerWidth-menuWidth+1))
		topLine = borderStyle.Render("├" + leftPart + junction + rightPart + "┤")
	}

	// Status bar with side borders - ensure single line with exact width
	statusBarContent := statusBarStyle.Width(m.width - 2).MaxHeight(1).Render(bar)
	statusLine := borderStyle.Render("│") + statusBarContent + borderStyle.Render("│")

	// Bottom border with corners
	bottomLine := borderStyle.Render("╰" + strings.Repeat("─", max(0, m.width-2)) + "╯")

	return lipgloss.JoinVertical(lipgloss.Left, topLine, statusLine, bottomLine)
}

// GetSelectedItem returns the selected menu item name (empty if none)
func (m *AppModel) GetSelectedItem() string {
	return m.selectedItem
}

// SetViewportContent sets the content of the viewport
func (m *AppModel) SetViewportContent(content string) {
	m.viewport.SetContent(content)
}

// SetProgram sets the Bubble Tea program reference for sending log messages
func (m *AppModel) SetProgram(p *tea.Program) {
	m.logBuffer.SetProgram(p)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func padRight(s string, width int) string {
	// Get visible width (accounting for ANSI escape codes)
	visibleWidth := lipgloss.Width(s)
	if visibleWidth >= width {
		return s
	}
	return s + strings.Repeat(" ", width-visibleWidth)
}
