package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// CalendarEvent represents a calendar event
type CalendarEvent struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time"`
	Location  string    `json:"location,omitempty"`
	Notes     string    `json:"notes,omitempty"`
	Calendar  string    `json:"calendar,omitempty"`
	AllDay    bool      `json:"all_day"`
}

// CalendarProviderInterface defines the calendar provider contract
type CalendarProviderInterface interface {
	GetTodayEvents(ctx context.Context) ([]CalendarEvent, error)
	GetUpcomingEvents(ctx context.Context, days int) ([]CalendarEvent, error)
	GetEventsInRange(ctx context.Context, start, end time.Time) ([]CalendarEvent, error)
	Close() error
}

// AppleCalendarProvider reads from Apple Calendar via AppleScript
type AppleCalendarProvider struct{}

// NewAppleCalendarProvider creates a new Apple Calendar provider
func NewAppleCalendarProvider() *AppleCalendarProvider {
	return &AppleCalendarProvider{}
}

// GetTodayEvents returns events for today
func (p *AppleCalendarProvider) GetTodayEvents(ctx context.Context) ([]CalendarEvent, error) {
	now := time.Now()
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	endOfDay := startOfDay.Add(24 * time.Hour)

	return p.GetEventsInRange(ctx, startOfDay, endOfDay)
}

// GetUpcomingEvents returns events for the next N days
func (p *AppleCalendarProvider) GetUpcomingEvents(ctx context.Context, days int) ([]CalendarEvent, error) {
	now := time.Now()
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	endDate := startOfDay.Add(time.Duration(days) * 24 * time.Hour)

	return p.GetEventsInRange(ctx, startOfDay, endDate)
}

// GetEventsInRange returns events between two dates
func (p *AppleCalendarProvider) GetEventsInRange(ctx context.Context, start, end time.Time) ([]CalendarEvent, error) {
	// Use icalBuddy for fast calendar access (brew install ical-buddy)
	// Fall back to simple AppleScript if not available
	cmd := exec.CommandContext(ctx, "icalBuddy",
		"-f", "-nc", "-nrd", "-npn", "-b", "",
		"-ps", "|",
		"-po", "datetime,title,location",
		"-tf", "%H:%M",
		"-df", "",
		"eventsToday")

	output, err := cmd.Output()
	if err == nil && len(output) > 0 {
		return p.parseIcalBuddyOutput(string(output))
	}

	// Fallback: AppleScript to query ALL calendars
	// First ensure Calendar is running
	launchCmd := exec.CommandContext(ctx, "open", "-ga", "Calendar")
	launchCmd.Run() // Ignore errors - Calendar might already be running

	script := `
set output to "["
set isFirst to true
set todayDate to current date
set todayStart to todayDate - (time of todayDate)
set todayEnd to todayStart + (1 * days)

tell application "Calendar"
	repeat with cal in calendars
		set calName to name of cal
		try
			set eventList to (every event of cal whose start date >= todayStart and start date < todayEnd)
			repeat with evt in eventList
				if not isFirst then set output to output & ","
				set isFirst to false
				set evtTitle to summary of evt
				set evtStart to start date of evt
				set startHour to hours of evtStart
				set startMin to minutes of evtStart
				set evtAllDay to allday event of evt
				set evtJSON to "{\"title\":\"" & evtTitle & "\",\"start_hour\":" & startHour & ",\"start_min\":" & startMin & ",\"all_day\":" & evtAllDay & ",\"calendar\":\"" & calName & "\",\"location\":\"\",\"end_hour\":0,\"end_min\":0}"
				set output to output & evtJSON
			end repeat
		end try
	end repeat
end tell
return output & "]"
`

	cmd = exec.CommandContext(ctx, "osascript", "-e", script)
	output, err = cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to run AppleScript: %w", err)
	}

	// Parse JSON output
	var rawEvents []struct {
		Title     string `json:"title"`
		Calendar  string `json:"calendar"`
		StartHour int    `json:"start_hour"`
		StartMin  int    `json:"start_min"`
		EndHour   int    `json:"end_hour"`
		EndMin    int    `json:"end_min"`
		Location  string `json:"location"`
		AllDay    bool   `json:"all_day"`
	}

	if err := json.Unmarshal(output, &rawEvents); err != nil {
		return nil, fmt.Errorf("failed to parse calendar events: %w (output: %s)", err, string(output))
	}

	// Convert to CalendarEvent
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	events := make([]CalendarEvent, len(rawEvents))
	for i, raw := range rawEvents {
		startTime := today.Add(time.Duration(raw.StartHour)*time.Hour + time.Duration(raw.StartMin)*time.Minute)
		endTime := today.Add(time.Duration(raw.EndHour)*time.Hour + time.Duration(raw.EndMin)*time.Minute)

		events[i] = CalendarEvent{
			ID:        fmt.Sprintf("%s-%d", raw.Title, raw.StartHour*100+raw.StartMin),
			Title:     raw.Title,
			StartTime: startTime,
			EndTime:   endTime,
			Location:  raw.Location,
			Calendar:  raw.Calendar,
			AllDay:    raw.AllDay,
		}
	}

	return events, nil
}

// parseIcalBuddyOutput parses icalBuddy output format
func (p *AppleCalendarProvider) parseIcalBuddyOutput(output string) ([]CalendarEvent, error) {
	var events []CalendarEvent
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Format: "HH:MM|Title|Location" or "Title|Location" for all-day
		parts := strings.SplitN(line, "|", 3)
		if len(parts) < 2 {
			continue
		}

		var event CalendarEvent
		event.ID = fmt.Sprintf("%d-%s", len(events), parts[0])

		// Check if first part is time
		if len(parts[0]) == 5 && strings.Contains(parts[0], ":") {
			// Timed event
			timeParts := strings.Split(parts[0], ":")
			if len(timeParts) == 2 {
				hour, _ := fmt.Sscanf(timeParts[0], "%d", new(int))
				min, _ := fmt.Sscanf(timeParts[1], "%d", new(int))
				if hour > 0 && min >= 0 {
					var h, m int
					fmt.Sscanf(parts[0], "%d:%d", &h, &m)
					event.StartTime = today.Add(time.Duration(h)*time.Hour + time.Duration(m)*time.Minute)
				}
			}
			event.Title = parts[1]
			if len(parts) > 2 {
				event.Location = parts[2]
			}
		} else {
			// All-day event
			event.AllDay = true
			event.Title = parts[0]
			if len(parts) > 1 {
				event.Location = parts[1]
			}
		}

		if event.Title != "" {
			events = append(events, event)
		}
	}

	return events, nil
}

// Close is a no-op for the calendar provider
func (p *AppleCalendarProvider) Close() error {
	return nil
}
