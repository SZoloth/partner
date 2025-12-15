package app

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/szoloth/partner/internal/claude"
	"github.com/szoloth/partner/internal/mcp"
	"github.com/szoloth/partner/internal/mcp/providers"
	"github.com/szoloth/partner/internal/mcp/transport"
	"github.com/szoloth/partner/internal/panes"
	"github.com/szoloth/partner/internal/panes/calendar"
	"github.com/szoloth/partner/internal/panes/tasks"
	"github.com/szoloth/partner/internal/theme"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// LayoutMode determines pane arrangement
type LayoutMode int

const (
	LayoutSingle LayoutMode = iota
	LayoutSplitH // Horizontal split (side by side)
	LayoutSplitV // Vertical split (stacked)
	LayoutGrid   // 2x2 grid
)

// Option configures the app
type Option func(*Model)

// WithHeadless sets headless mode
func WithHeadless(headless bool) Option {
	return func(m *Model) {
		m.headless = headless
	}
}

// WithInitialPane sets the initial pane
func WithInitialPane(paneName string) Option {
	return func(m *Model) {
		m.initialPane = panes.ParsePaneType(paneName)
	}
}

// Model is the root application model
type Model struct {
	// Layout state
	layout      LayoutMode
	activePanes []panes.Pane
	focusedPane int

	// All panes (lazily initialized)
	paneInstances map[panes.PaneType]panes.Pane

	// MCP providers
	thingsProvider   *providers.ThingsProvider
	calendarProvider providers.CalendarProviderInterface

	// Global state
	width             int
	height            int
	ready             bool
	status            string
	headless          bool
	initialPane       panes.PaneType
	awaitingWindowCmd bool
	previousLayout    LayoutMode // For maximize/restore

	// AI state
	claudeClient   *claude.Client
	aiModalVisible bool
	aiResponse     string
	aiAction       *claude.Action
	aiLoading      bool
	aiUsage        *claude.Usage // Token usage from last call

	// Styles
	styles *theme.Styles
}

// NewModel creates a new app model
func NewModel(opts ...Option) *Model {
	m := &Model{
		layout:        LayoutSingle,
		paneInstances: make(map[panes.PaneType]panes.Pane),
		styles:        theme.NewStyles(),
		initialPane:   panes.PaneTasks,
		claudeClient:  claude.NewClient(),
	}

	for _, opt := range opts {
		opt(m)
	}

	return m
}

// Init initializes the app
func (m *Model) Init() tea.Cmd {
	return tea.Batch(
		m.initMCPProviders(),
		tea.SetWindowTitle("Partner"),
	)
}

// initMCPProviders initializes MCP server connections
func (m *Model) initMCPProviders() tea.Cmd {
	return func() tea.Msg {
		// Initialize Things 3 MCP
		// Use the local Python Things MCP server via wrapper script
thingsTransport, err := transport.NewStdioTransport("/Users/samuelz/partner/scripts/things-mcp.sh", nil)
		if err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to create Things transport: %w", err)}
		}

		thingsClient := mcp.NewClient(thingsTransport, "things")
		m.thingsProvider = providers.NewThingsProvider(thingsClient)

		// Initialize Google Calendar MCP provider
		gcalTransport, err := transport.NewStdioTransport("npx", []string{"-y", "@cocal/google-calendar-mcp"},
			transport.WithEnv(`GOOGLE_OAUTH_CREDENTIALS=/Users/samuelz/Documents/LLM CONTEXT/credentials.json`))
		if err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to create Google Calendar transport: %w", err)}
		}
		gcalClient := mcp.NewClient(gcalTransport, "google-calendar")
		m.calendarProvider = providers.NewGCalProvider(gcalClient)

		// Create panes
		tasksPane := tasks.New(m.thingsProvider)
		m.paneInstances[panes.PaneTasks] = tasksPane

		calendarPane := calendar.New(m.calendarProvider)
		m.paneInstances[panes.PaneCalendar] = calendarPane

		// Start with tasks focused
		m.activePanes = []panes.Pane{tasksPane.Focus().(panes.Pane)}

		return MCPInitializedMsg{}
	}
}

// MCPInitializedMsg indicates MCP providers are ready
type MCPInitializedMsg struct{}

// AIResponseMsg carries Claude's response
type AIResponseMsg struct {
	Text      string
	Action    *claude.Action
	Err       error
	SessionID string
	Usage     *claude.Usage
}

// Update handles messages
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Global keybindings
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit

		case "tab":
			m.focusNext()
			return m, nil

		case "shift+tab":
			m.focusPrev()
			return m, nil

		// Pane number shortcuts (direct, no modifier needed)
		case "1":
			return m, m.switchToPane(panes.PaneTasks)
		case "2":
			return m, m.switchToPane(panes.PaneCalendar)
		case "3":
			return m, m.switchToPane(panes.PaneEmail)
		case "4":
			return m, m.switchToPane(panes.PaneKnowledge)
		case "5":
			return m, m.switchToPane(panes.PaneCRM)
		case "6":
			return m, m.switchToPane(panes.PaneProjects)

		// Layout toggles
		case "\\":
			return m, m.toggleSplit()
		case "|":
			return m, m.toggleSplit()

		// Maximize current pane (Ctrl+w o)
		case "ctrl+w":
			m.awaitingWindowCmd = true
			return m, nil
		case "o":
			if m.awaitingWindowCmd {
				m.awaitingWindowCmd = false
				return m, m.maximizePane()
			}

		// AI assist
		case "a":
			if m.aiModalVisible {
				// Close modal
				m.aiModalVisible = false
				return m, nil
			}
			// Trigger AI assist based on current pane
			return m, m.triggerAIAssist()

		// AI modal actions
		case "enter":
			if m.aiModalVisible && m.aiAction != nil {
				// Execute suggested action
				m.aiModalVisible = false
				return m, m.executeAIAction()
			}
		case "c":
			if m.aiModalVisible {
				// Continue conversation - prompt for follow-up
				m.aiModalVisible = false
				m.status = "Type follow-up and press 'a' again (session preserved)"
				return m, nil
			}
		case "esc":
			if m.aiModalVisible {
				m.aiModalVisible = false
				// Clear session when closing modal
				m.claudeClient.ClearSession()
				m.status = "AI session cleared"
				return m, nil
			}
		}

		// Route to focused pane
		if len(m.activePanes) > 0 && m.focusedPane < len(m.activePanes) {
			pane := m.activePanes[m.focusedPane]
			updated, cmd := pane.Update(msg)
			m.activePanes[m.focusedPane] = updated.(panes.Pane)
			cmds = append(cmds, cmd)
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true
		m.redistributeSpace()

	case MCPInitializedMsg:
		m.status = "Connected"
		// Refresh the initial pane
		if len(m.activePanes) > 0 {
			cmds = append(cmds, m.activePanes[0].Refresh())
		}

	case ErrorMsg:
		m.status = fmt.Sprintf("Error: %v", msg.Err)

	case StatusMsg:
		m.status = msg.Text

	// Route data messages to appropriate panes
	case tasks.TasksLoadedMsg, tasks.TaskCompletedMsg:
		if pane, ok := m.paneInstances[panes.PaneTasks]; ok {
			updated, cmd := pane.Update(msg)
			m.paneInstances[panes.PaneTasks] = updated.(panes.Pane)
			// Update in active panes too
			for i, ap := range m.activePanes {
				if ap.Type() == panes.PaneTasks {
					m.activePanes[i] = updated.(panes.Pane)
				}
			}
			cmds = append(cmds, cmd)
		}

	case calendar.EventsLoadedMsg:
		if pane, ok := m.paneInstances[panes.PaneCalendar]; ok {
			updated, cmd := pane.Update(msg)
			m.paneInstances[panes.PaneCalendar] = updated.(panes.Pane)
			// Update in active panes too
			for i, ap := range m.activePanes {
				if ap.Type() == panes.PaneCalendar {
					m.activePanes[i] = updated.(panes.Pane)
				}
			}
			cmds = append(cmds, cmd)
		}

	case AIResponseMsg:
		m.aiLoading = false
		if msg.Err != nil {
			m.aiResponse = fmt.Sprintf("Error: %v", msg.Err)
			m.aiAction = nil
			m.aiUsage = nil
		} else {
			m.aiResponse = msg.Text
			m.aiAction = msg.Action
			m.aiUsage = msg.Usage
		}
		m.aiModalVisible = true
	}

	return m, tea.Batch(cmds...)
}

// View renders the app
func (m *Model) View() string {
	if !m.ready {
		return "\n  Initializing Partner..."
	}

	var b strings.Builder

	// Status bar at top
	statusBar := m.renderStatusBar()
	b.WriteString(statusBar)
	b.WriteString("\n")

	// Main content area
	contentHeight := m.height - 3 // status bar + help line
	content := m.renderPanes(contentHeight)
	b.WriteString(content)

	// Help line at bottom
	helpLine := m.renderHelpLine()
	b.WriteString("\n")
	b.WriteString(helpLine)

	// Overlay AI modal if visible
	if m.aiLoading || m.aiModalVisible {
		return m.overlayAIModal(b.String())
	}

	return b.String()
}

func (m *Model) renderStatusBar() string {
	// Left side: Partner title
	left := m.styles.Title.Render(" Partner ")

	// Center: status
	center := m.styles.Muted.Render(m.status)

	// Right side: time
	now := time.Now().Format("Mon Jan 2 3:04 PM")
	right := m.styles.Muted.Render(now + " ")

	// Calculate spacing
	leftWidth := lipgloss.Width(left)
	centerWidth := lipgloss.Width(center)
	rightWidth := lipgloss.Width(right)
	totalWidth := m.width

	// Distribute space
	leftPad := (totalWidth - centerWidth) / 2 - leftWidth
	rightPad := totalWidth - leftWidth - leftPad - centerWidth - rightWidth

	if leftPad < 0 {
		leftPad = 1
	}
	if rightPad < 0 {
		rightPad = 1
	}

	bar := left + strings.Repeat(" ", leftPad) + center + strings.Repeat(" ", rightPad) + right

	return m.styles.StatusBar.Width(m.width).Render(bar)
}

func (m *Model) renderPanes(height int) string {
	if len(m.activePanes) == 0 {
		return m.styles.Muted.Render("\n  No panes active")
	}

	switch m.layout {
	case LayoutSingle:
		return m.renderSinglePane(height)
	case LayoutSplitH:
		return m.renderSplitH(height)
	case LayoutSplitV:
		return m.renderSplitV(height)
	case LayoutGrid:
		return m.renderGrid(height)
	default:
		return m.renderSinglePane(height)
	}
}

func (m *Model) renderSinglePane(height int) string {
	if len(m.activePanes) == 0 {
		return ""
	}

	pane := m.activePanes[0]
	// Account for title line in height calculation
	pane = pane.SetSize(m.width-4, height-3).(panes.Pane)
	m.activePanes[0] = pane

	style := m.styles.PaneBorder
	if pane.IsFocused() {
		style = m.styles.PaneBorderFocus
	}

	// Title above the pane content
	title := m.styles.PaneTitle.Render(" " + pane.Title() + " ")
	content := pane.View()

	// Combine title and content
	fullContent := title + "\n" + content

	// Create bordered pane
	return style.Width(m.width - 2).Height(height).Render(fullContent)
}

func (m *Model) renderSplitH(height int) string {
	if len(m.activePanes) < 2 {
		return m.renderSinglePane(height)
	}

	halfWidth := (m.width - 2) / 2

	left := m.renderPaneBox(m.activePanes[0], halfWidth, height, m.focusedPane == 0)
	right := m.renderPaneBox(m.activePanes[1], halfWidth, height, m.focusedPane == 1)

	return lipgloss.JoinHorizontal(lipgloss.Top, left, right)
}

func (m *Model) renderSplitV(height int) string {
	if len(m.activePanes) < 2 {
		return m.renderSinglePane(height)
	}

	halfHeight := height / 2

	top := m.renderPaneBox(m.activePanes[0], m.width-2, halfHeight, m.focusedPane == 0)
	bottom := m.renderPaneBox(m.activePanes[1], m.width-2, halfHeight, m.focusedPane == 1)

	return lipgloss.JoinVertical(lipgloss.Left, top, bottom)
}

func (m *Model) renderGrid(height int) string {
	if len(m.activePanes) < 4 {
		// Fall back to split if not enough panes
		if len(m.activePanes) < 2 {
			return m.renderSinglePane(height)
		}
		return m.renderSplitH(height)
	}

	halfWidth := (m.width - 2) / 2
	halfHeight := height / 2

	// Top row
	topLeft := m.renderPaneBox(m.activePanes[0], halfWidth, halfHeight, m.focusedPane == 0)
	topRight := m.renderPaneBox(m.activePanes[1], halfWidth, halfHeight, m.focusedPane == 1)
	topRow := lipgloss.JoinHorizontal(lipgloss.Top, topLeft, topRight)

	// Bottom row
	bottomLeft := m.renderPaneBox(m.activePanes[2], halfWidth, halfHeight, m.focusedPane == 2)
	bottomRight := m.renderPaneBox(m.activePanes[3], halfWidth, halfHeight, m.focusedPane == 3)
	bottomRow := lipgloss.JoinHorizontal(lipgloss.Top, bottomLeft, bottomRight)

	return lipgloss.JoinVertical(lipgloss.Left, topRow, bottomRow)
}

func (m *Model) renderPaneBox(p panes.Pane, width, height int, focused bool) string {
	// Account for title line in height
	p = p.SetSize(width-4, height-4).(panes.Pane)

	style := m.styles.PaneBorder
	if focused {
		style = m.styles.PaneBorderFocus
	}

	// Title + content like single pane mode
	title := m.styles.PaneTitle.Render(" " + p.Title() + " ")
	content := p.View()
	fullContent := title + "\n" + content

	return style.Width(width).Height(height).Render(fullContent)
}

func (m *Model) renderHelpLine() string {
	help := "q:quit  tab:focus  \\:split  1-6:panes  ^wo:maximize  a:ai"
	return m.styles.Muted.Render("  " + help)
}

// overlayAIModal renders a centered modal over the existing content
func (m *Model) overlayAIModal(background string) string {
	// Modal dimensions
	modalWidth := min(m.width-10, 60)
	modalHeight := min(m.height-6, 20)

	// Modal styles - use theme's primary color for accent
	accentColor := theme.Current.Primary

	modalBorder := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(accentColor).
		Padding(1, 2).
		Width(modalWidth).
		Height(modalHeight)

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(accentColor)

	// Build modal content
	var content strings.Builder

	if m.aiLoading {
		content.WriteString(titleStyle.Render("🤖 Asking Claude..."))
		content.WriteString("\n\n")
		content.WriteString(m.styles.Muted.Render("Please wait..."))
	} else {
		content.WriteString(titleStyle.Render("🤖 Claude Says"))
		content.WriteString("\n\n")

		// Word-wrap the response
		wrapped := wordWrap(m.aiResponse, modalWidth-6)
		content.WriteString(wrapped)

		// Show action hint if there's a suggested action
		if m.aiAction != nil {
			content.WriteString("\n\n")
			actionHint := fmt.Sprintf("Suggested: %s", m.aiAction.Description)
			accentStyle := lipgloss.NewStyle().Foreground(accentColor)
			content.WriteString(accentStyle.Render(actionHint))
		}

		// Show usage stats if available
		if m.aiUsage != nil {
			content.WriteString("\n\n")
			usageText := fmt.Sprintf("tokens: %d in / %d out  cost: $%.4f  time: %dms",
				m.aiUsage.InputTokens, m.aiUsage.OutputTokens,
				m.aiUsage.CostUSD, m.aiUsage.DurationMs)
			content.WriteString(m.styles.Muted.Render(usageText))
		}

		// Help line
		content.WriteString("\n\n")
		helpText := "c:continue  enter:execute  esc:close"
		content.WriteString(m.styles.Muted.Render(helpText))
	}

	modal := modalBorder.Render(content.String())

	// Center the modal
	modalLines := strings.Split(modal, "\n")
	bgLines := strings.Split(background, "\n")

	// Calculate vertical offset
	startY := (m.height - len(modalLines)) / 2
	startX := (m.width - modalWidth - 4) / 2

	// Overlay modal on background
	result := make([]string, len(bgLines))
	for i, line := range bgLines {
		if i >= startY && i < startY+len(modalLines) {
			modalLine := modalLines[i-startY]
			// Pad to position
			if startX > 0 && startX < len(line) {
				// Simple overlay - replace middle portion
				result[i] = padRight(line[:startX], startX) + modalLine
			} else {
				result[i] = modalLine
			}
		} else {
			result[i] = line
		}
	}

	return strings.Join(result, "\n")
}

// wordWrap wraps text at word boundaries
func wordWrap(text string, width int) string {
	if width <= 0 {
		return text
	}

	var result strings.Builder
	lines := strings.Split(text, "\n")

	for _, line := range lines {
		words := strings.Fields(line)
		if len(words) == 0 {
			result.WriteString("\n")
			continue
		}

		currentLine := words[0]
		for _, word := range words[1:] {
			if len(currentLine)+1+len(word) > width {
				result.WriteString(currentLine)
				result.WriteString("\n")
				currentLine = word
			} else {
				currentLine += " " + word
			}
		}
		result.WriteString(currentLine)
		result.WriteString("\n")
	}

	return strings.TrimSuffix(result.String(), "\n")
}

// padRight pads a string to a given width
func padRight(s string, width int) string {
	if len(s) >= width {
		return s
	}
	return s + strings.Repeat(" ", width-len(s))
}

// Navigation helpers
func (m *Model) focusNext() {
	if len(m.activePanes) == 0 {
		return
	}

	// Blur current
	m.activePanes[m.focusedPane] = m.activePanes[m.focusedPane].Blur().(panes.Pane)

	// Move to next
	m.focusedPane = (m.focusedPane + 1) % len(m.activePanes)

	// Focus new
	m.activePanes[m.focusedPane] = m.activePanes[m.focusedPane].Focus().(panes.Pane)
}

func (m *Model) focusPrev() {
	if len(m.activePanes) == 0 {
		return
	}

	// Blur current
	m.activePanes[m.focusedPane] = m.activePanes[m.focusedPane].Blur().(panes.Pane)

	// Move to prev
	m.focusedPane--
	if m.focusedPane < 0 {
		m.focusedPane = len(m.activePanes) - 1
	}

	// Focus new
	m.activePanes[m.focusedPane] = m.activePanes[m.focusedPane].Focus().(panes.Pane)
}

func (m *Model) switchToPane(target panes.PaneType) tea.Cmd {
	pane, ok := m.paneInstances[target]
	if !ok {
		return nil
	}

	// If in split mode, replace the focused pane
	if m.layout != LayoutSingle && len(m.activePanes) > 1 {
		m.activePanes[m.focusedPane] = m.activePanes[m.focusedPane].Blur().(panes.Pane)
		m.activePanes[m.focusedPane] = pane.Focus().(panes.Pane)
		m.redistributeSpace()
		return pane.Refresh()
	}

	// Single pane mode - replace the only pane
	if len(m.activePanes) > 0 {
		m.activePanes[m.focusedPane] = m.activePanes[m.focusedPane].Blur().(panes.Pane)
	}

	m.activePanes = []panes.Pane{pane.Focus().(panes.Pane)}
	m.focusedPane = 0
	m.layout = LayoutSingle
	m.redistributeSpace()

	return pane.Refresh()
}

func (m *Model) toggleSplit() tea.Cmd {
	var cmds []tea.Cmd

	switch m.layout {
	case LayoutSingle:
		// Enter split mode with Tasks + Calendar
		m.layout = LayoutSplitH

		tasksPane, hasT := m.paneInstances[panes.PaneTasks]
		calendarPane, hasC := m.paneInstances[panes.PaneCalendar]

		if !hasT || !hasC {
			return nil
		}

		m.activePanes = []panes.Pane{
			tasksPane.Focus().(panes.Pane),
			calendarPane.Blur().(panes.Pane),
		}
		m.focusedPane = 0
		m.redistributeSpace()

		cmds = append(cmds, tasksPane.Refresh(), calendarPane.Refresh())

	case LayoutSplitH:
		// Toggle to vertical split
		m.layout = LayoutSplitV
		m.redistributeSpace()

	case LayoutSplitV:
		// Toggle to grid (4 panes)
		m.layout = LayoutGrid
		// Add placeholder panes for grid if we don't have 4
		tasksPane := m.paneInstances[panes.PaneTasks]
		calendarPane := m.paneInstances[panes.PaneCalendar]

		// For now, duplicate Tasks and Calendar for grid demo
		// (Email and Knowledge panes not implemented yet)
		m.activePanes = []panes.Pane{
			tasksPane.Focus().(panes.Pane),
			calendarPane.Blur().(panes.Pane),
			tasksPane.Blur().(panes.Pane),    // Placeholder
			calendarPane.Blur().(panes.Pane), // Placeholder
		}
		m.focusedPane = 0
		m.redistributeSpace()

	case LayoutGrid:
		// Back to single pane (keep focused pane)
		m.layout = LayoutSingle
		if len(m.activePanes) > 0 {
			focused := m.activePanes[m.focusedPane]
			m.activePanes = []panes.Pane{focused.Focus().(panes.Pane)}
			m.focusedPane = 0
		}
		m.redistributeSpace()
	}

	return tea.Batch(cmds...)
}

func (m *Model) maximizePane() tea.Cmd {
	if m.layout == LayoutSingle {
		// Already maximized - restore previous layout
		if m.previousLayout != LayoutSingle {
			m.layout = m.previousLayout
			return m.toggleSplit() // Re-enter the previous split mode
		}
		return nil
	}

	// Save current layout and maximize
	m.previousLayout = m.layout
	m.layout = LayoutSingle

	if len(m.activePanes) > 0 && m.focusedPane < len(m.activePanes) {
		focused := m.activePanes[m.focusedPane]
		m.activePanes = []panes.Pane{focused.Focus().(panes.Pane)}
		m.focusedPane = 0
	}
	m.redistributeSpace()
	return nil
}

func (m *Model) redistributeSpace() {
	if len(m.activePanes) == 0 {
		return
	}

	contentHeight := m.height - 3

	switch m.layout {
	case LayoutSingle:
		m.activePanes[0] = m.activePanes[0].SetSize(m.width-4, contentHeight-2).(panes.Pane)
	case LayoutSplitH:
		halfWidth := (m.width - 2) / 2
		for i := range m.activePanes {
			m.activePanes[i] = m.activePanes[i].SetSize(halfWidth-4, contentHeight-2).(panes.Pane)
		}
	case LayoutSplitV:
		halfHeight := contentHeight / 2
		for i := range m.activePanes {
			m.activePanes[i] = m.activePanes[i].SetSize(m.width-4, halfHeight-2).(panes.Pane)
		}
	case LayoutGrid:
		halfWidth := (m.width - 2) / 2
		halfHeight := contentHeight / 2
		for i := range m.activePanes {
			m.activePanes[i] = m.activePanes[i].SetSize(halfWidth-4, halfHeight-2).(panes.Pane)
		}
	}
}

// triggerAIAssist asks Claude for help based on the current pane context
func (m *Model) triggerAIAssist() tea.Cmd {
	m.aiLoading = true
	m.status = "Asking Claude..."

	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		// Build context based on current pane
		var paneContext string
		var prompt string

		if len(m.activePanes) > 0 && m.focusedPane < len(m.activePanes) {
			currentPane := m.activePanes[m.focusedPane]

			switch currentPane.Type() {
			case panes.PaneTasks:
				// Get task titles for context
				if m.thingsProvider != nil {
					tasks, err := m.thingsProvider.GetToday(ctx)
					if err == nil {
						var taskList []string
						for _, t := range tasks {
							taskList = append(taskList, t.Title)
						}
						paneContext = "Today's tasks:\n- " + strings.Join(taskList, "\n- ")
					}
				}
				prompt = "Based on my tasks for today, what's the single highest-leverage needle-mover I should focus on? Be brief (2-3 sentences)."

			case panes.PaneCalendar:
				// Get calendar events for context
				if m.calendarProvider != nil {
					events, err := m.calendarProvider.GetTodayEvents(ctx)
					if err == nil {
						var eventList []string
						for _, e := range events {
							eventList = append(eventList, fmt.Sprintf("%s at %s", e.Title, e.StartTime.Format("3:04 PM")))
						}
						paneContext = "Today's schedule:\n- " + strings.Join(eventList, "\n- ")
					}
				}
				prompt = "Looking at my schedule, what should I be aware of? Any conflicts or prep needed? Be brief."

			default:
				prompt = "What's the most important thing I should focus on right now?"
			}
		}

		// Ask Claude
		resp := m.claudeClient.Ask(ctx, claude.Request{
			Prompt:     prompt,
			Context:    paneContext,
			AllowTools: false,
		})

		return AIResponseMsg{
			Text:      resp.Text,
			Action:    resp.Action,
			Err:       resp.Error,
			SessionID: resp.SessionID,
			Usage:     resp.Usage,
		}
	}
}

// executeAIAction executes a suggested action from Claude
func (m *Model) executeAIAction() tea.Cmd {
	if m.aiAction == nil {
		return nil
	}

	switch m.aiAction.Type {
	case claude.ActionCompleteTask:
		m.status = "Completing task... (not yet implemented)"
		// TODO: Mark selected task as done via Things MCP

	case claude.ActionDraftEmail:
		m.status = "Draft email... (not yet implemented)"
		// TODO: Open email draft modal

	case claude.ActionCreateTask:
		m.status = "Create task... (not yet implemented)"
		// TODO: Create task via Things MCP

	default:
		m.status = "Action acknowledged"
	}

	return nil
}

// FetchCurrentPaneData fetches data for headless mode
func (m *Model) FetchCurrentPaneData() (interface{}, error) {
	// Initialize MCP providers synchronously for headless mode
	// Use the local Python Things MCP server via wrapper script
thingsTransport, err := transport.NewStdioTransport("/Users/samuelz/partner/scripts/things-mcp.sh", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create Things transport: %w", err)
	}

	thingsClient := mcp.NewClient(thingsTransport, "things")
	m.thingsProvider = providers.NewThingsProvider(thingsClient)
	defer m.thingsProvider.Close()

	ctx := context.Background()

	switch m.initialPane {
	case panes.PaneTasks:
		tasks, err := m.thingsProvider.GetTodayDebug(ctx)
		if err != nil {
			return nil, err
		}
		return tasks, nil

	default:
		return nil, fmt.Errorf("pane %s not yet implemented for headless mode", m.initialPane)
	}
}
