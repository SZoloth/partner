package app

import (
	"github.com/szoloth/partner/internal/panes"
	tea "github.com/charmbracelet/bubbletea"
)

// Navigation messages
type SwitchPaneMsg struct {
	Target panes.PaneType
}

type ChangeLayoutMsg struct {
	Layout LayoutMode
}

type FocusNextMsg struct{}
type FocusPrevMsg struct{}

// Data messages
type DataLoadedMsg struct {
	Pane panes.PaneType
	Data interface{}
	Err  error
}

type RefreshMsg struct {
	Pane panes.PaneType
}

// Error messages
type ErrorMsg struct {
	Err error
}

// Status messages
type StatusMsg struct {
	Text string
}

// Command helpers
func switchPane(target panes.PaneType) tea.Cmd {
	return func() tea.Msg {
		return SwitchPaneMsg{Target: target}
	}
}

func setStatus(text string) tea.Cmd {
	return func() tea.Msg {
		return StatusMsg{Text: text}
	}
}
