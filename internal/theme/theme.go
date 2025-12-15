package theme

import "github.com/charmbracelet/lipgloss"

// Theme defines the color palette for the TUI
type Theme struct {
	Name        string
	Primary     lipgloss.Color
	Secondary   lipgloss.Color
	Background  lipgloss.Color
	Surface     lipgloss.Color
	Text        lipgloss.Color
	TextMuted   lipgloss.Color
	Border      lipgloss.Color
	BorderFocus lipgloss.Color
	Success     lipgloss.Color
	Warning     lipgloss.Color
	Error       lipgloss.Color
}

// CatppuccinMocha theme
var CatppuccinMocha = Theme{
	Name:        "catppuccin_mocha",
	Primary:     lipgloss.Color("#cba6f7"), // Mauve
	Secondary:   lipgloss.Color("#89b4fa"), // Blue
	Background:  lipgloss.Color("#1e1e2e"), // Base
	Surface:     lipgloss.Color("#313244"), // Surface0
	Text:        lipgloss.Color("#cdd6f4"), // Text
	TextMuted:   lipgloss.Color("#6c7086"), // Overlay0
	Border:      lipgloss.Color("#45475a"), // Surface1
	BorderFocus: lipgloss.Color("#cba6f7"), // Mauve
	Success:     lipgloss.Color("#a6e3a1"), // Green
	Warning:     lipgloss.Color("#f9e2af"), // Yellow
	Error:       lipgloss.Color("#f38ba8"), // Red
}

// TeenageEngineering - inspired by EP-133 K.O. II
var TeenageEngineering = Theme{
	Name:        "teenage_engineering",
	Primary:     lipgloss.Color("#FF6B35"), // TE Orange (active/selected)
	Secondary:   lipgloss.Color("#4ECDC4"), // TE Cyan/Teal
	Background:  lipgloss.Color("#0D0D0D"), // Deep black
	Surface:     lipgloss.Color("#1A1A1A"), // Slightly lighter black
	Text:        lipgloss.Color("#FFFFFF"), // Pure white
	TextMuted:   lipgloss.Color("#666666"), // Gray
	Border:      lipgloss.Color("#333333"), // Dark gray border
	BorderFocus: lipgloss.Color("#FF6B35"), // TE Orange
	Success:     lipgloss.Color("#4ECDC4"), // Teal
	Warning:     lipgloss.Color("#FFE66D"), // Yellow
	Error:       lipgloss.Color("#FF4757"), // Red
}

// Current is the active theme
var Current = TeenageEngineering

// Styles provides pre-configured lipgloss styles
type Styles struct {
	// Base styles
	Base       lipgloss.Style
	Title      lipgloss.Style
	Subtitle   lipgloss.Style
	Muted      lipgloss.Style

	// Status indicators
	Success    lipgloss.Style
	Warning    lipgloss.Style
	Error      lipgloss.Style

	// Pane styles
	PaneBorder       lipgloss.Style
	PaneBorderFocus  lipgloss.Style
	PaneTitle        lipgloss.Style

	// List styles
	ListItem         lipgloss.Style
	ListItemSelected lipgloss.Style
	ListItemDone     lipgloss.Style

	// Status bar
	StatusBar        lipgloss.Style
	StatusKey        lipgloss.Style
	StatusValue      lipgloss.Style
}

// NewStyles creates styles from the current theme
func NewStyles() *Styles {
	t := Current

	return &Styles{
		Base: lipgloss.NewStyle().
			Foreground(t.Text),

		Title: lipgloss.NewStyle().
			Foreground(t.Primary).
			Bold(true),

		Subtitle: lipgloss.NewStyle().
			Foreground(t.Secondary),

		Muted: lipgloss.NewStyle().
			Foreground(t.TextMuted),

		Success: lipgloss.NewStyle().
			Foreground(t.Success),

		Warning: lipgloss.NewStyle().
			Foreground(t.Warning),

		Error: lipgloss.NewStyle().
			Foreground(t.Error),

		PaneBorder: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(t.Border).
			Padding(0, 1),

		PaneBorderFocus: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(t.BorderFocus).
			Padding(0, 1),

		PaneTitle: lipgloss.NewStyle().
			Foreground(t.Primary).
			Bold(true).
			Padding(0, 1),

		ListItem: lipgloss.NewStyle().
			Foreground(t.Text).
			PaddingLeft(2),

		ListItemSelected: lipgloss.NewStyle().
			Foreground(t.Primary).
			Bold(true).
			PaddingLeft(2),

		ListItemDone: lipgloss.NewStyle().
			Foreground(t.TextMuted).
			Strikethrough(true).
			PaddingLeft(2),

		StatusBar: lipgloss.NewStyle().
			Background(t.Surface).
			Foreground(t.Text).
			Padding(0, 1),

		StatusKey: lipgloss.NewStyle().
			Foreground(t.Primary).
			Bold(true),

		StatusValue: lipgloss.NewStyle().
			Foreground(t.Text),
	}
}
