package panes

import tea "github.com/charmbracelet/bubbletea"

// PaneType identifies different pane types
type PaneType int

const (
	PaneTasks PaneType = iota
	PaneCalendar
	PaneEmail
	PaneKnowledge
	PaneCRM
	PaneProjects
	PaneCoS // Chief of Staff pane
)

// String returns the pane name
func (p PaneType) String() string {
	switch p {
	case PaneTasks:
		return "tasks"
	case PaneCalendar:
		return "calendar"
	case PaneEmail:
		return "email"
	case PaneKnowledge:
		return "knowledge"
	case PaneCRM:
		return "crm"
	case PaneProjects:
		return "projects"
	case PaneCoS:
		return "cos"
	default:
		return "unknown"
	}
}

// ParsePaneType converts a string to PaneType
func ParsePaneType(s string) PaneType {
	switch s {
	case "tasks":
		return PaneTasks
	case "calendar":
		return PaneCalendar
	case "email":
		return PaneEmail
	case "knowledge":
		return PaneKnowledge
	case "crm":
		return PaneCRM
	case "projects":
		return PaneProjects
	case "cos":
		return PaneCoS
	default:
		return PaneTasks
	}
}

// Pane is the interface all panes must implement
type Pane interface {
	tea.Model

	// Focus management
	Focus() Pane
	Blur() Pane
	IsFocused() bool

	// Dimensions
	SetSize(width, height int) Pane

	// Identity
	Type() PaneType
	Title() string

	// Data operations
	Refresh() tea.Cmd
	GetData() interface{}
}
