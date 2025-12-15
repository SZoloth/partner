package tasks

import (
	"context"
	"fmt"
	"strings"

	"github.com/szoloth/partner/internal/mcp/providers"
	"github.com/szoloth/partner/internal/panes"
	"github.com/szoloth/partner/internal/theme"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// View mode for the tasks pane
type ViewMode int

const (
	ViewToday ViewMode = iota
	ViewInbox
	ViewUpcoming
	ViewAnytime
)

func (v ViewMode) String() string {
	switch v {
	case ViewToday:
		return "Today"
	case ViewInbox:
		return "Inbox"
	case ViewUpcoming:
		return "Upcoming"
	case ViewAnytime:
		return "Anytime"
	default:
		return "Unknown"
	}
}

// Model is the Tasks pane model
type Model struct {
	provider *providers.ThingsProvider
	styles   *theme.Styles

	// State
	tasks    []providers.Task
	cursor   int
	selected map[string]bool
	loading  bool
	err      error
	viewMode ViewMode

	// Dimensions
	width   int
	height  int
	focused bool
}

// New creates a new Tasks pane
func New(provider *providers.ThingsProvider) *Model {
	return &Model{
		provider: provider,
		styles:   theme.NewStyles(),
		selected: make(map[string]bool),
		viewMode: ViewToday,
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
			if m.cursor < len(m.tasks)-1 {
				m.cursor++
			}
		case "k", "up":
			if m.cursor > 0 {
				m.cursor--
			}
		case "g":
			m.cursor = 0
		case "G":
			if len(m.tasks) > 0 {
				m.cursor = len(m.tasks) - 1
			}

		// Selection
		case " ", "x":
			if len(m.tasks) > 0 {
				task := m.tasks[m.cursor]
				m.selected[task.UUID] = !m.selected[task.UUID]
			}

		// Actions
		case "d":
			// Mark complete
			if len(m.tasks) > 0 {
				return m, m.markComplete(m.tasks[m.cursor].UUID)
			}
		case "r":
			// Refresh
			return m, m.Refresh()

		// View switching
		case "1":
			m.viewMode = ViewToday
			return m, m.Refresh()
		case "2":
			m.viewMode = ViewInbox
			return m, m.Refresh()
		case "3":
			m.viewMode = ViewUpcoming
			return m, m.Refresh()
		case "4":
			m.viewMode = ViewAnytime
			return m, m.Refresh()
		}

	case TasksLoadedMsg:
		m.loading = false
		if msg.Err != nil {
			m.err = msg.Err
		} else {
			m.tasks = msg.Tasks
			m.err = nil
			// Reset cursor if out of bounds
			if m.cursor >= len(m.tasks) {
				m.cursor = max(0, len(m.tasks)-1)
			}
		}

	case TaskCompletedMsg:
		if msg.Err != nil {
			m.err = msg.Err
		} else {
			// Refresh to get updated list
			return m, m.Refresh()
		}
	}

	return m, nil
}

// View renders the pane
func (m *Model) View() string {
	var b strings.Builder

	// Header
	header := m.renderHeader()
	b.WriteString(header)
	b.WriteString("\n")

	// Content area height
	contentHeight := m.height - 4 // header + footer

	if m.loading {
		b.WriteString(m.styles.Muted.Render("\n  Loading..."))
	} else if m.err != nil {
		b.WriteString(m.styles.Error.Render(fmt.Sprintf("\n  Error: %v", m.err)))
	} else if len(m.tasks) == 0 {
		b.WriteString(m.styles.Muted.Render("\n  No tasks"))
	} else {
		// Render visible tasks
		start := 0
		if m.cursor >= contentHeight {
			start = m.cursor - contentHeight + 1
		}
		end := min(start+contentHeight, len(m.tasks))

		for i := start; i < end; i++ {
			task := m.tasks[i]
			line := m.renderTask(task, i == m.cursor, m.selected[task.UUID])
			b.WriteString(line)
			b.WriteString("\n")
		}
	}

	// Footer with shortcuts
	footer := m.renderFooter()

	// Pad to fill height
	lines := strings.Count(b.String(), "\n")
	for i := lines; i < m.height-1; i++ {
		b.WriteString("\n")
	}
	b.WriteString(footer)

	return b.String()
}

func (m *Model) renderHeader() string {
	// View mode tabs
	tabs := []string{"1:Today", "2:Inbox", "3:Upcoming", "4:Anytime"}
	var tabParts []string

	for i, tab := range tabs {
		if ViewMode(i) == m.viewMode {
			tabParts = append(tabParts, m.styles.Title.Render(tab))
		} else {
			tabParts = append(tabParts, m.styles.Muted.Render(tab))
		}
	}

	return lipgloss.JoinHorizontal(lipgloss.Left, "  ", strings.Join(tabParts, "  "))
}

func (m *Model) renderTask(task providers.Task, isCursor, isSelected bool) string {
	// Status indicator
	var status string
	if task.Status == "completed" {
		status = "[x]"
	} else if isSelected {
		status = "[*]"
	} else {
		status = "[ ]"
	}

	// Cursor indicator
	cursor := "  "
	if isCursor {
		cursor = "> "
	}

	// Title
	title := task.Title
	if len(title) > m.width-10 {
		title = title[:m.width-13] + "..."
	}

	// Build line
	line := fmt.Sprintf("%s%s %s", cursor, status, title)

	// Style based on state
	var style lipgloss.Style
	switch {
	case task.Status == "completed":
		style = m.styles.ListItemDone
	case isCursor:
		style = m.styles.ListItemSelected
	default:
		style = m.styles.ListItem
	}

	return style.Render(line)
}

func (m *Model) renderFooter() string {
	shortcuts := "j/k:nav  d:done  space:select  r:refresh"
	return m.styles.Muted.Render("  " + shortcuts)
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
	return panes.PaneTasks
}

// Title returns the pane title
func (m *Model) Title() string {
	return "Tasks"
}

// Refresh fetches fresh data
func (m *Model) Refresh() tea.Cmd {
	m.loading = true
	viewMode := m.viewMode

	return func() tea.Msg {
		ctx := context.Background()
		var tasks []providers.Task
		var err error

		switch viewMode {
		case ViewToday:
			tasks, err = m.provider.GetToday(ctx)
		case ViewInbox:
			tasks, err = m.provider.GetInbox(ctx)
		case ViewUpcoming:
			tasks, err = m.provider.GetUpcoming(ctx)
		case ViewAnytime:
			tasks, err = m.provider.GetAnytime(ctx)
		}

		return TasksLoadedMsg{Tasks: tasks, Err: err}
	}
}

// GetData returns the current tasks for headless mode
func (m *Model) GetData() interface{} {
	return map[string]interface{}{
		"view":  m.viewMode.String(),
		"tasks": m.tasks,
		"count": len(m.tasks),
	}
}

// markComplete marks a task as complete
func (m *Model) markComplete(id string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		err := m.provider.MarkComplete(ctx, id)
		return TaskCompletedMsg{ID: id, Err: err}
	}
}

// Messages
type TasksLoadedMsg struct {
	Tasks []providers.Task
	Err   error
}

type TaskCompletedMsg struct {
	ID  string
	Err error
}

// Helper functions
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
