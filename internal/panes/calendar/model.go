package calendar

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/szoloth/partner/internal/mcp/providers"
	"github.com/szoloth/partner/internal/panes"
	"github.com/szoloth/partner/internal/theme"

	tea "github.com/charmbracelet/bubbletea"
)

// ViewMode determines calendar display
type ViewMode int

const (
	ViewToday ViewMode = iota
	ViewWeek
	ViewAgenda
)

// Model represents the calendar pane
type Model struct {
	provider providers.CalendarProviderInterface
	events   []providers.CalendarEvent
	viewMode ViewMode
	cursor   int
	focused  bool
	width    int
	height   int
	loading  bool
	err      error
	styles   *theme.Styles
}

// EventsLoadedMsg is sent when events are loaded
type EventsLoadedMsg struct {
	Events []providers.CalendarEvent
	Err    error
}

// New creates a new calendar pane
func New(provider providers.CalendarProviderInterface) *Model {
	return &Model{
		provider: provider,
		viewMode: ViewToday,
		styles:   theme.NewStyles(),
	}
}

// Init implements tea.Model
func (m *Model) Init() tea.Cmd {
	m.loading = true
	return m.loadEvents()
}

// Update implements tea.Model
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if !m.focused {
			return m, nil
		}

		switch msg.String() {
		case "j", "down":
			if m.cursor < len(m.events)-1 {
				m.cursor++
			}
		case "k", "up":
			if m.cursor > 0 {
				m.cursor--
			}
		case "g":
			m.cursor = 0
		case "G":
			if len(m.events) > 0 {
				m.cursor = len(m.events) - 1
			}
		case "r":
			m.loading = true
			return m, m.loadEvents()
		case "1":
			m.viewMode = ViewToday
			m.loading = true
			return m, m.loadEvents()
		case "2":
			m.viewMode = ViewWeek
			m.loading = true
			return m, m.loadEvents()
		case "3":
			m.viewMode = ViewAgenda
			m.loading = true
			return m, m.loadEvents()
		}

	case EventsLoadedMsg:
		m.loading = false
		if msg.Err != nil {
			m.err = msg.Err
		} else {
			m.events = msg.Events
			m.err = nil
		}
	}

	return m, nil
}

// View implements tea.Model
func (m *Model) View() string {
	var b strings.Builder

	// View mode tabs
	tabs := m.renderTabs()
	b.WriteString(tabs)
	b.WriteString("\n")

	if m.loading {
		b.WriteString(m.styles.Muted.Render("  Loading events..."))
		return b.String()
	}

	if m.err != nil {
		b.WriteString(m.styles.Error.Render(fmt.Sprintf("  Error: %v", m.err)))
		return b.String()
	}

	if len(m.events) == 0 {
		b.WriteString(m.styles.Muted.Render("  No events"))
		return b.String()
	}

	// Group events by date for agenda view
	eventsByDate := m.groupEventsByDate()

	linesUsed := 1 // tabs line
	for date, events := range eventsByDate {
		if linesUsed >= m.height-3 {
			break
		}

		// Date header
		dateStr := m.formatDateHeader(date)
		b.WriteString(m.styles.Subtitle.Render("  " + dateStr))
		b.WriteString("\n")
		linesUsed++

		for _, event := range events {
			if linesUsed >= m.height-3 {
				break
			}

			line := m.renderEvent(event)
			b.WriteString(line)
			b.WriteString("\n")
			linesUsed++
		}
	}

	// Help
	b.WriteString("\n")
	b.WriteString(m.styles.Muted.Render("  j/k:nav  r:refresh"))

	return b.String()
}

func (m *Model) renderTabs() string {
	var tabs []string

	modes := []struct {
		mode  ViewMode
		label string
	}{
		{ViewToday, "1:Today"},
		{ViewWeek, "2:Week"},
		{ViewAgenda, "3:Agenda"},
	}

	for _, mode := range modes {
		if m.viewMode == mode.mode {
			tabs = append(tabs, m.styles.ListItemSelected.Render(mode.label))
		} else {
			tabs = append(tabs, m.styles.Muted.Render(mode.label))
		}
	}

	return "  " + strings.Join(tabs, "  ")
}

func (m *Model) renderEvent(event providers.CalendarEvent) string {
	var timeStr string
	if event.AllDay {
		timeStr = "All day"
	} else {
		timeStr = event.StartTime.Format("3:04 PM")
	}

	// Truncate title if needed
	title := event.Title
	maxTitleLen := m.width - 25
	if maxTitleLen > 0 && len(title) > maxTitleLen {
		title = title[:maxTitleLen-3] + "..."
	}

	// Format: "  10:00 AM  Meeting title [Cal]"
	timeStyle := m.styles.Muted
	titleStyle := m.styles.ListItem

	line := fmt.Sprintf("  %s  %s",
		timeStyle.Render(fmt.Sprintf("%-8s", timeStr)),
		titleStyle.Render(title),
	)

	// Add calendar name indicator
	if event.Calendar != "" {
		// Short calendar indicator
		calShort := event.Calendar
		if len(calShort) > 10 {
			calShort = calShort[:10]
		}
		line += m.styles.Muted.Render(" [" + calShort + "]")
	}

	// Add location if present
	if event.Location != "" {
		loc := event.Location
		if len(loc) > 20 {
			loc = loc[:17] + "..."
		}
		line += m.styles.Muted.Render(" @ " + loc)
	}

	return line
}

func (m *Model) groupEventsByDate() map[string][]providers.CalendarEvent {
	result := make(map[string][]providers.CalendarEvent)

	for _, event := range m.events {
		dateKey := event.StartTime.Format("2006-01-02")
		result[dateKey] = append(result[dateKey], event)
	}

	return result
}

func (m *Model) formatDateHeader(dateStr string) string {
	t, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return dateStr
	}

	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	tomorrow := today.Add(24 * time.Hour)

	eventDate := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())

	if eventDate.Equal(today) {
		return "Today - " + t.Format("Mon, Jan 2")
	} else if eventDate.Equal(tomorrow) {
		return "Tomorrow - " + t.Format("Mon, Jan 2")
	}

	return t.Format("Mon, Jan 2")
}

func (m *Model) loadEvents() tea.Cmd {
	viewMode := m.viewMode
	provider := m.provider

	return func() tea.Msg {
		ctx := context.Background()

		var events []providers.CalendarEvent
		var err error

		switch viewMode {
		case ViewToday:
			events, err = provider.GetTodayEvents(ctx)
		case ViewWeek:
			events, err = provider.GetUpcomingEvents(ctx, 7)
		case ViewAgenda:
			events, err = provider.GetUpcomingEvents(ctx, 14)
		}

		return EventsLoadedMsg{Events: events, Err: err}
	}
}

// Pane interface implementation

func (m *Model) Type() panes.PaneType {
	return panes.PaneCalendar
}

func (m *Model) Title() string {
	return "Calendar"
}

func (m *Model) Focus() panes.Pane {
	m.focused = true
	return m
}

func (m *Model) Blur() panes.Pane {
	m.focused = false
	return m
}

func (m *Model) IsFocused() bool {
	return m.focused
}

func (m *Model) SetSize(width, height int) panes.Pane {
	m.width = width
	m.height = height
	return m
}

func (m *Model) GetData() interface{} {
	return m.events
}

func (m *Model) Refresh() tea.Cmd {
	m.loading = true
	return m.loadEvents()
}

func (m *Model) ShortHelp() []string {
	return []string{"j/k:nav", "1-3:view", "r:refresh"}
}

func (m *Model) FullHelp() [][]string {
	return [][]string{
		{"j/k", "Navigate"},
		{"1/2/3", "Today/Week/Agenda"},
		{"r", "Refresh"},
	}
}

// Ensure Model implements panes.Pane
var _ panes.Pane = (*Model)(nil)
