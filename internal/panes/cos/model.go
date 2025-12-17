package cos

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	cosstate "github.com/szoloth/partner/internal/cos"
	"github.com/szoloth/partner/internal/panes"
	"github.com/szoloth/partner/internal/theme"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Model is the Chief of Staff pane model
type Model struct {
	provider *cosstate.Provider
	styles   *theme.Styles

	// State
	state   *cosstate.State
	cursor  int
	loading bool
	err     error

	// Dimensions
	width   int
	height  int
	focused bool
}

// New creates a new CoS pane
func New() *Model {
	return &Model{
		provider: cosstate.NewProvider(),
		styles:   theme.NewStyles(),
	}
}

// Init initializes the pane
func (m *Model) Init() tea.Cmd {
	return m.Refresh()
}

// Update handles messages
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if !m.focused {
			return m, nil
		}

		switch msg.String() {
		// Navigation
		case "j", "down":
			if m.state != nil && m.cursor < len(m.state.ActionQueue.Pending)-1 {
				m.cursor++
			}
		case "k", "up":
			if m.cursor > 0 {
				m.cursor--
			}

		// Actions
		case "s":
			// Send/execute selected action
			if m.state != nil && len(m.state.ActionQueue.Pending) > 0 {
				return m, m.executeAction(m.cursor)
			}
		case "x":
			// Skip selected action
			if m.state != nil && len(m.state.ActionQueue.Pending) > 0 {
				return m, m.skipAction(m.cursor)
			}
		case "r":
			// Refresh
			return m, m.Refresh()
		case "o":
			// Open draft file
			if m.state != nil && len(m.state.ActionQueue.Pending) > m.cursor {
				return m, m.openDraft(m.cursor)
			}
		}

	case StateLoadedMsg:
		m.loading = false
		if msg.Err != nil {
			m.err = msg.Err
		} else {
			m.state = msg.State
			m.err = nil
		}

	case ActionExecutedMsg:
		if msg.Err != nil {
			m.err = msg.Err
		} else {
			// Refresh to show updated state
			return m, m.Refresh()
		}
	}

	return m, nil
}

// View renders the pane
func (m *Model) View() string {
	var b strings.Builder

	if m.loading {
		b.WriteString(m.styles.Muted.Render("\n  Loading CoS state..."))
		return b.String()
	}

	if m.err != nil {
		b.WriteString(m.styles.Error.Render(fmt.Sprintf("\n  Error: %v", m.err)))
		return b.String()
	}

	if m.state == nil {
		b.WriteString(m.styles.Muted.Render("\n  No state loaded"))
		return b.String()
	}

	// Needle Mover section
	b.WriteString(m.renderNeedleMover())
	b.WriteString("\n")

	// Streaks section
	b.WriteString(m.renderStreaks())
	b.WriteString("\n")

	// Action Queue section
	b.WriteString(m.renderActionQueue())

	// Alerts section (avoidance detection, cold outreach)
	alerts := m.renderAlerts()
	if alerts != "" {
		b.WriteString("\n")
		b.WriteString(alerts)
	}

	// Footer
	b.WriteString("\n")
	b.WriteString(m.renderFooter())

	return b.String()
}

func (m *Model) renderNeedleMover() string {
	var b strings.Builder

	header := m.styles.Title.Render("  NEEDLE MOVER")
	b.WriteString(header)
	b.WriteString("\n")

	needle := m.provider.GetNeedleMover(m.state)
	if needle == nil {
		b.WriteString(m.styles.Muted.Render("  No needle-mover set"))
		return b.String()
	}

	// Show the primary action
	targetStyle := lipgloss.NewStyle().
		Foreground(theme.Current.Primary).
		Bold(true)

	actionLine := fmt.Sprintf("  %s: %s", needle.Type, needle.Company)
	if needle.Contact != "" {
		actionLine += fmt.Sprintf(" (%s)", needle.Contact)
	}
	b.WriteString(targetStyle.Render(actionLine))

	if needle.DraftPath != "" {
		b.WriteString("\n")
		b.WriteString(m.styles.Muted.Render(fmt.Sprintf("  Draft: %s", truncatePath(needle.DraftPath, m.width-10))))
	}

	return b.String()
}

func (m *Model) renderStreaks() string {
	var b strings.Builder

	header := m.styles.Title.Render("  STREAKS")
	b.WriteString(header)
	b.WriteString("\n")

	// Outreach streak
	outreach := m.state.Streaks.Outreach
	outreachStatus := fmt.Sprintf("  Outreach: %d/%d this week", outreach.CurrentWeek, outreach.WeeklyTarget)
	if outreach.CurrentWeek == 0 {
		days := m.provider.DaysSinceLastOutreach(m.state)
		outreachStatus += fmt.Sprintf(" (%d days cold)", days)
		b.WriteString(m.styles.Error.Render(outreachStatus))
	} else if outreach.CurrentWeek >= outreach.WeeklyTarget {
		b.WriteString(m.styles.Success.Render(outreachStatus + " +"))
	} else {
		b.WriteString(m.styles.ListItem.Render(outreachStatus))
	}
	b.WriteString("\n")

	// Needle-mover streak
	nm := m.state.Streaks.NeedleMover
	nmStatus := fmt.Sprintf("  Needle-mover: %d days", nm.Current)
	if nm.LastCompleted != "" {
		nmStatus += fmt.Sprintf(" (last: %s)", nm.LastCompleted)
	}
	if nm.Current == 0 {
		b.WriteString(m.styles.Muted.Render(nmStatus))
	} else {
		b.WriteString(m.styles.ListItem.Render(nmStatus))
	}
	b.WriteString("\n")

	// Training streak
	tr := m.state.Streaks.Training
	trStatus := fmt.Sprintf("  Training: %d days this week", tr.DaysThisWeek)
	b.WriteString(m.styles.ListItem.Render(trStatus))

	return b.String()
}

func (m *Model) renderActionQueue() string {
	var b strings.Builder

	header := m.styles.Title.Render("  ACTION QUEUE")
	b.WriteString(header)
	b.WriteString("\n")

	if len(m.state.ActionQueue.Pending) == 0 {
		b.WriteString(m.styles.Muted.Render("  No pending actions"))
		return b.String()
	}

	for i, action := range m.state.ActionQueue.Pending {
		cursor := "  "
		if m.focused && i == m.cursor {
			cursor = "> "
		}

		actionText := fmt.Sprintf("[%d] %s", action.ID, action.Type)
		if action.Company != "" {
			actionText += fmt.Sprintf(" - %s", action.Company)
		}
		if action.Contact != "" {
			actionText += fmt.Sprintf(" (%s)", action.Contact)
		}

		var style lipgloss.Style
		if m.focused && i == m.cursor {
			style = m.styles.ListItemSelected
		} else {
			style = m.styles.ListItem
		}

		b.WriteString(style.Render(cursor + actionText))
		b.WriteString("\n")
	}

	return b.String()
}

func (m *Model) renderAlerts() string {
	var alerts []string

	// Avoidance detection
	if m.provider.IsAvoidanceDetected(m.state) {
		alert := m.styles.Warning.Render("  ++ AVOIDANCE PATTERN DETECTED")
		alerts = append(alerts, alert)
		alerts = append(alerts, m.styles.Muted.Render("  Planning without shipping. Execute the needle-mover."))
	}

	// Cold outreach
	if m.provider.IsOutreachCold(m.state) && m.state.Streaks.Outreach.CurrentWeek == 0 {
		days := m.provider.DaysSinceLastOutreach(m.state)
		if days >= m.state.Thresholds.OutreachColdDays {
			alert := m.styles.Warning.Render(fmt.Sprintf("  ++ OUTREACH COLD (%d+ days)", days))
			alerts = append(alerts, alert)
		}
	}

	if len(alerts) == 0 {
		return ""
	}

	return strings.Join(alerts, "\n")
}

func (m *Model) renderFooter() string {
	shortcuts := "j/k:nav  s:send  x:skip  o:open draft  r:refresh"
	return m.styles.Muted.Render("  " + shortcuts)
}

// executeAction sends the selected action
func (m *Model) executeAction(index int) tea.Cmd {
	if index >= len(m.state.ActionQueue.Pending) {
		return nil
	}

	action := m.state.ActionQueue.Pending[index]

	return func() tea.Msg {
		// Mark as complete in state
		m.provider.MarkActionComplete(m.state, action.ID)

		// Save updated state
		if err := m.provider.Save(m.state); err != nil {
			return ActionExecutedMsg{Err: err}
		}

		return ActionExecutedMsg{ActionID: action.ID}
	}
}

// skipAction skips the selected action
func (m *Model) skipAction(index int) tea.Cmd {
	if index >= len(m.state.ActionQueue.Pending) {
		return nil
	}

	action := m.state.ActionQueue.Pending[index]

	return func() tea.Msg {
		// Mark as skipped in state
		m.provider.MarkActionSkipped(m.state, action.ID)

		// Save updated state
		if err := m.provider.Save(m.state); err != nil {
			return ActionExecutedMsg{Err: err}
		}

		return ActionExecutedMsg{ActionID: action.ID}
	}
}

// openDraft opens the draft file in the default editor
func (m *Model) openDraft(index int) tea.Cmd {
	if index >= len(m.state.ActionQueue.Pending) {
		return nil
	}

	action := m.state.ActionQueue.Pending[index]
	if action.DraftPath == "" {
		return nil
	}

	// Check if file exists
	if _, err := os.Stat(action.DraftPath); os.IsNotExist(err) {
		return func() tea.Msg {
			return ActionExecutedMsg{Err: fmt.Errorf("draft file not found: %s", action.DraftPath)}
		}
	}

	path := action.DraftPath
	// Open with system default (macOS specific) using exec
	return func() tea.Msg {
		cmd := exec.Command("open", path)
		if err := cmd.Start(); err != nil {
			return ActionExecutedMsg{Err: err}
		}
		return nil
	}
}

// Focus sets the pane as focused
func (m *Model) Focus() panes.Pane {
	m.focused = true
	return m
}

// Blur removes focus from the pane
func (m *Model) Blur() panes.Pane {
	m.focused = false
	return m
}

// IsFocused returns whether the pane is focused
func (m *Model) IsFocused() bool {
	return m.focused
}

// SetSize sets the pane dimensions
func (m *Model) SetSize(width, height int) panes.Pane {
	m.width = width
	m.height = height
	return m
}

// Type returns the pane type
func (m *Model) Type() panes.PaneType {
	return panes.PaneCoS
}

// Title returns the pane title
func (m *Model) Title() string {
	return "Chief of Staff"
}

// Refresh fetches fresh data
func (m *Model) Refresh() tea.Cmd {
	m.loading = true

	return func() tea.Msg {
		state, err := m.provider.Load()
		return StateLoadedMsg{State: state, Err: err}
	}
}

// GetData returns the current state for headless mode
func (m *Model) GetData() interface{} {
	if m.state == nil {
		return nil
	}
	return map[string]interface{}{
		"needle_mover":   m.provider.GetNeedleMover(m.state),
		"streaks":        m.state.Streaks,
		"pending_count":  len(m.state.ActionQueue.Pending),
		"avoidance":      m.provider.IsAvoidanceDetected(m.state),
		"outreach_cold":  m.provider.IsOutreachCold(m.state),
	}
}

// GetState returns the full state (for AI context)
func (m *Model) GetState() *cosstate.State {
	return m.state
}

// Messages
type StateLoadedMsg struct {
	State *cosstate.State
	Err   error
}

type ActionExecutedMsg struct {
	ActionID int
	Err      error
}

// Helper to truncate file paths
func truncatePath(path string, maxLen int) string {
	if len(path) <= maxLen {
		return path
	}
	// Show last part of path
	parts := strings.Split(path, "/")
	if len(parts) <= 2 {
		return path[:maxLen-3] + "..."
	}
	// Show .../last-two-parts
	return ".../" + strings.Join(parts[len(parts)-2:], "/")
}
