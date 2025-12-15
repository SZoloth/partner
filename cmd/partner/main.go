package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/szoloth/partner/internal/app"

	tea "github.com/charmbracelet/bubbletea"
)

var (
	version = "0.4.0"

	// CLI flags
	jsonOutput  bool
	showVersion bool
	paneFlag    string
	refreshFlag bool
)

func init() {
	flag.BoolVar(&jsonOutput, "json", false, "Output in JSON format (headless mode)")
	flag.BoolVar(&showVersion, "version", false, "Show version")
	flag.StringVar(&paneFlag, "pane", "tasks", "Initial pane to display (tasks, calendar, email, knowledge, crm, projects)")
	flag.BoolVar(&refreshFlag, "refresh", false, "Refresh data and exit (use with --json)")
}

func main() {
	flag.Parse()

	if showVersion {
		fmt.Printf("partner v%s\n", version)
		os.Exit(0)
	}

	// Headless mode for automation
	if jsonOutput {
		runHeadless()
		return
	}

	// Interactive TUI mode
	runInteractive()
}

func runHeadless() {
	// Create app in headless mode
	model := app.NewModel(app.WithHeadless(true), app.WithInitialPane(paneFlag))

	// Fetch data
	data, err := model.FetchCurrentPaneData()
	if err != nil {
		output := map[string]interface{}{
			"error": err.Error(),
			"pane":  paneFlag,
		}
		json.NewEncoder(os.Stdout).Encode(output)
		os.Exit(1)
	}

	// Output JSON
	output := map[string]interface{}{
		"pane": paneFlag,
		"data": data,
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	enc.Encode(output)
}

func runInteractive() {
	model := app.NewModel(app.WithInitialPane(paneFlag))

	p := tea.NewProgram(
		model,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running partner: %v\n", err)
		os.Exit(1)
	}
}
