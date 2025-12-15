package main

import (
	"context"
	"fmt"
	"time"

	"github.com/szoloth/partner/internal/mcp/providers"
)

func main() {
	fmt.Println("Testing Apple Calendar provider...")

	provider := providers.NewAppleCalendarProvider()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	fmt.Println("Fetching today's events (may take a moment if Calendar needs to launch)...")
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
		fmt.Printf("  - %s at %s [%s]\n", e.Title, e.StartTime.Format("3:04 PM"), e.Calendar)
	}

	if len(events) == 0 {
		fmt.Println("  (no events today)")
	}

	fmt.Println("\nCalendar provider works correctly!")
}
