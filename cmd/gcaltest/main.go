package main

import (
	"context"
	"fmt"
	"time"

	"github.com/szoloth/partner/internal/mcp"
	"github.com/szoloth/partner/internal/mcp/providers"
	"github.com/szoloth/partner/internal/mcp/transport"
)

func main() {
	fmt.Println("Testing Google Calendar MCP provider...")

	// Create transport with credentials
	gcalTransport, err := transport.NewStdioTransport("npx", []string{"-y", "@cocal/google-calendar-mcp"},
		transport.WithEnv(`GOOGLE_OAUTH_CREDENTIALS=/Users/samuelz/Documents/LLM CONTEXT/credentials.json`))
	if err != nil {
		fmt.Printf("Failed to create transport: %v\n", err)
		return
	}

	gcalClient := mcp.NewClient(gcalTransport, "google-calendar")
	provider := providers.NewGCalProvider(gcalClient)
	defer provider.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	fmt.Println("Fetching today's events...")
	start := time.Now()
	events, err := provider.GetTodayEvents(ctx)
	elapsed := time.Since(start)

	fmt.Printf("Fetch took: %v\n", elapsed)

	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Got %d events:\n", len(events))
	for _, e := range events {
		timeStr := e.StartTime.Format("3:04 PM")
		if e.AllDay {
			timeStr = "All day"
		}
		fmt.Printf("  - %s at %s\n", e.Title, timeStr)
	}

	if len(events) == 0 {
		fmt.Println("  (no events today)")
	}

	fmt.Println("\nGoogle Calendar provider works correctly!")
}
