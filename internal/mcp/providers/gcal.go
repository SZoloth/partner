package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/szoloth/partner/internal/mcp"
)

// GCalProvider reads from Google Calendar via MCP
type GCalProvider struct {
	client *mcp.Client
}

// NewGCalProvider creates a new Google Calendar provider
func NewGCalProvider(client *mcp.Client) *GCalProvider {
	return &GCalProvider{client: client}
}

// GetTodayEvents returns events for today
func (p *GCalProvider) GetTodayEvents(ctx context.Context) ([]CalendarEvent, error) {
	now := time.Now()
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	endOfDay := startOfDay.Add(24 * time.Hour)

	return p.GetEventsInRange(ctx, startOfDay, endOfDay)
}

// GetUpcomingEvents returns events for the next N days
func (p *GCalProvider) GetUpcomingEvents(ctx context.Context, days int) ([]CalendarEvent, error) {
	now := time.Now()
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	endDate := startOfDay.Add(time.Duration(days) * 24 * time.Hour)

	return p.GetEventsInRange(ctx, startOfDay, endDate)
}

// GetEventsInRange returns events between two dates
func (p *GCalProvider) GetEventsInRange(ctx context.Context, start, end time.Time) ([]CalendarEvent, error) {
	// Format times for Google Calendar API
	timeMin := start.Format("2006-01-02T15:04:05")
	timeMax := end.Format("2006-01-02T15:04:05")

	// Call list-events with "primary" calendar
	args := map[string]interface{}{
		"calendarId": "primary",
		"timeMin":    timeMin,
		"timeMax":    timeMax,
	}

	result, err := p.client.CallTool(ctx, "list-events", args)
	if err != nil {
		return nil, fmt.Errorf("list-events failed: %w", err)
	}

	return p.parseEvents(result)
}

// parseEvents converts MCP tool result to CalendarEvents
func (p *GCalProvider) parseEvents(result *mcp.ToolResult) ([]CalendarEvent, error) {
	if len(result.Content) == 0 {
		return []CalendarEvent{}, nil
	}

	// Get the text content
	var text string
	for _, block := range result.Content {
		if block.Type == "text" {
			text = block.Text
			break
		}
	}

	if text == "" {
		return []CalendarEvent{}, nil
	}

	// Google Calendar MCP returns JSON array of events
	var gcalEvents []gcalEvent
	if err := json.Unmarshal([]byte(text), &gcalEvents); err != nil {
		// Try to parse as wrapped response
		var wrapped struct {
			Events []gcalEvent `json:"events"`
		}
		if err2 := json.Unmarshal([]byte(text), &wrapped); err2 != nil {
			return nil, fmt.Errorf("failed to parse calendar events: %w (text: %s)", err, truncate(text, 200))
		}
		gcalEvents = wrapped.Events
	}

	// Convert to CalendarEvent
	events := make([]CalendarEvent, 0, len(gcalEvents))
	for _, ge := range gcalEvents {
		event := CalendarEvent{
			ID:       ge.ID,
			Title:    ge.Summary,
			Location: ge.Location,
		}

		// Parse start time
		if ge.Start.DateTime != "" {
			t, err := time.Parse(time.RFC3339, ge.Start.DateTime)
			if err == nil {
				event.StartTime = t
			}
		} else if ge.Start.Date != "" {
			// All-day event
			t, err := time.Parse("2006-01-02", ge.Start.Date)
			if err == nil {
				event.StartTime = t
				event.AllDay = true
			}
		}

		// Parse end time
		if ge.End.DateTime != "" {
			t, err := time.Parse(time.RFC3339, ge.End.DateTime)
			if err == nil {
				event.EndTime = t
			}
		} else if ge.End.Date != "" {
			t, err := time.Parse("2006-01-02", ge.End.Date)
			if err == nil {
				event.EndTime = t
			}
		}

		// Extract calendar name from organizer or set default
		if ge.Organizer.DisplayName != "" {
			event.Calendar = ge.Organizer.DisplayName
		} else if ge.Organizer.Email != "" {
			// Use email prefix as calendar name
			parts := strings.Split(ge.Organizer.Email, "@")
			event.Calendar = parts[0]
		} else {
			event.Calendar = "Primary"
		}

		events = append(events, event)
	}

	return events, nil
}

// Close closes the provider
func (p *GCalProvider) Close() error {
	return p.client.Close()
}

// gcalEvent represents a Google Calendar event from the API
type gcalEvent struct {
	ID        string         `json:"id"`
	Summary   string         `json:"summary"`
	Location  string         `json:"location,omitempty"`
	Start     gcalDateTime   `json:"start"`
	End       gcalDateTime   `json:"end"`
	Status    string         `json:"status"`
	HTMLLink  string         `json:"htmlLink"`
	Organizer gcalOrganizer  `json:"organizer,omitempty"`
	Attendees []gcalAttendee `json:"attendees,omitempty"`
}

type gcalDateTime struct {
	DateTime string `json:"dateTime,omitempty"`
	Date     string `json:"date,omitempty"`
	TimeZone string `json:"timeZone,omitempty"`
}

type gcalOrganizer struct {
	Email       string `json:"email,omitempty"`
	DisplayName string `json:"displayName,omitempty"`
	Self        bool   `json:"self,omitempty"`
}

type gcalAttendee struct {
	Email          string `json:"email"`
	DisplayName    string `json:"displayName,omitempty"`
	ResponseStatus string `json:"responseStatus,omitempty"`
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
