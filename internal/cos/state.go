package cos

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// DefaultStatePath is the standard location for CoS state
const DefaultStatePath = "~/.claude/state/cos-state.json"

// State represents the Chief of Staff system state
type State struct {
	Version     string    `json:"version"`
	LastUpdated time.Time `json:"last_updated"`

	Briefings Briefings `json:"briefings"`
	Streaks   Streaks   `json:"streaks"`
	Patterns  Patterns  `json:"patterns"`

	ActionQueue       ActionQueue       `json:"action_queue"`
	PreparedMaterials PreparedMaterials `json:"prepared_materials"`
	Thresholds        Thresholds        `json:"thresholds"`
}

// Briefings tracks when each briefing type was last run
type Briefings struct {
	Morning    BriefingState `json:"morning"`
	PreMeeting BriefingState `json:"pre_meeting"`
	EOD        BriefingState `json:"eod"`
	WeekAhead  BriefingState `json:"week_ahead"`
}

// BriefingState tracks a single briefing type
type BriefingState struct {
	LastRun           *time.Time `json:"last_run"`
	LastDelivered     *time.Time `json:"last_delivered"`
	MeetingsPrepped   []string   `json:"meetings_prepped_today,omitempty"`
}

// Streaks tracks various streak metrics
type Streaks struct {
	NeedleMover NeedleMoverStreak `json:"needle_mover"`
	Outreach    OutreachStreak    `json:"outreach"`
	Training    TrainingStreak    `json:"training"`
}

// NeedleMoverStreak tracks daily needle-mover completion
type NeedleMoverStreak struct {
	Current       int    `json:"current"`
	LastCompleted string `json:"last_completed"` // YYYY-MM-DD
	Longest       int    `json:"longest"`
}

// OutreachStreak tracks weekly outreach metrics
type OutreachStreak struct {
	CurrentWeek        int    `json:"current_week"`
	WeekStart          string `json:"week_start"` // YYYY-MM-DD
	WeeklyTarget       int    `json:"weekly_target"`
	WeeksHittingTarget int    `json:"weeks_hitting_target"`
}

// TrainingStreak tracks training activity
type TrainingStreak struct {
	DaysThisWeek int    `json:"days_this_week"`
	LastActivity string `json:"last_activity"` // YYYY-MM-DD
}

// Patterns tracks behavioral patterns
type Patterns struct {
	Last7Days         []string `json:"last_7_days"`
	AvoidanceFlags    int      `json:"avoidance_flags"`
	LastAvoidanceCall string   `json:"last_avoidance_call"` // YYYY-MM-DD
}

// ActionQueue holds pending items awaiting execution
type ActionQueue struct {
	Pending        []PendingAction `json:"pending"`
	CompletedToday []string        `json:"completed_today"`
	SkippedToday   []string        `json:"skipped_today"`
}

// PendingAction represents a queued action
type PendingAction struct {
	ID          int       `json:"id"`
	Type        string    `json:"type"`
	Company     string    `json:"company,omitempty"`
	Contact     string    `json:"contact,omitempty"`
	Role        string    `json:"role,omitempty"`
	DraftPath   string    `json:"draft_path,omitempty"`
	Description string    `json:"description,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

// PreparedMaterials holds ready-to-send items
type PreparedMaterials struct {
	OutreachDrafts []OutreachDraft `json:"outreach_drafts"`
	MeetingPrep    []MeetingPrep   `json:"meeting_prep"`
}

// OutreachDraft represents a prepared outreach message
type OutreachDraft struct {
	ID      int    `json:"id"`
	Company string `json:"company"`
	Contact string `json:"contact"`
	Path    string `json:"path"`
}

// MeetingPrep represents prepared meeting materials
type MeetingPrep struct {
	ID        int    `json:"id"`
	EventName string `json:"event_name"`
	Path      string `json:"path"`
}

// Thresholds defines warning thresholds
type Thresholds struct {
	OutreachColdDays      int `json:"outreach_cold_days"`
	DeadlineWarningDays   int `json:"deadline_warning_days"`
	AvoidancePlanningDays int `json:"avoidance_planning_days"`
}

// Provider reads and writes CoS state
type Provider struct {
	path string
}

// NewProvider creates a new CoS state provider
func NewProvider() *Provider {
	return &Provider{
		path: expandPath(DefaultStatePath),
	}
}

// NewProviderWithPath creates a provider with a custom path
func NewProviderWithPath(path string) *Provider {
	return &Provider{
		path: expandPath(path),
	}
}

// Load reads the current state from disk
func (p *Provider) Load() (*State, error) {
	data, err := os.ReadFile(p.path)
	if err != nil {
		if os.IsNotExist(err) {
			// Return default state if file doesn't exist
			return p.defaultState(), nil
		}
		return nil, fmt.Errorf("failed to read state file: %w", err)
	}

	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("failed to parse state file: %w", err)
	}

	return &state, nil
}

// Save writes the state to disk
func (p *Provider) Save(state *State) error {
	state.LastUpdated = time.Now()

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	// Ensure directory exists
	dir := filepath.Dir(p.path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create state directory: %w", err)
	}

	if err := os.WriteFile(p.path, data, 0644); err != nil {
		return fmt.Errorf("failed to write state file: %w", err)
	}

	return nil
}

// GetNeedleMover returns the current needle-mover if any
func (p *Provider) GetNeedleMover(state *State) *PendingAction {
	if len(state.ActionQueue.Pending) == 0 {
		return nil
	}
	return &state.ActionQueue.Pending[0]
}

// IsOutreachCold returns true if outreach has been cold too long
func (p *Provider) IsOutreachCold(state *State) bool {
	if state.Streaks.Outreach.CurrentWeek > 0 {
		return false
	}
	// Check days since last outreach (simplified - just check if week count is 0)
	return true
}

// DaysSinceLastOutreach calculates days since last outreach
func (p *Provider) DaysSinceLastOutreach(state *State) int {
	// This is a simplified calculation
	// In practice, you'd parse the date and calculate properly
	if state.Streaks.Outreach.CurrentWeek > 0 {
		return 0
	}
	return state.Thresholds.OutreachColdDays + 1
}

// IsAvoidanceDetected checks if avoidance pattern is active
func (p *Provider) IsAvoidanceDetected(state *State) bool {
	return state.Patterns.AvoidanceFlags > 0
}

// MarkActionComplete moves an action from pending to completed
func (p *Provider) MarkActionComplete(state *State, actionID int) {
	var remaining []PendingAction
	for _, a := range state.ActionQueue.Pending {
		if a.ID == actionID {
			state.ActionQueue.CompletedToday = append(
				state.ActionQueue.CompletedToday,
				fmt.Sprintf("%d:%s", a.ID, a.Type),
			)
		} else {
			remaining = append(remaining, a)
		}
	}
	state.ActionQueue.Pending = remaining
}

// MarkActionSkipped moves an action from pending to skipped
func (p *Provider) MarkActionSkipped(state *State, actionID int) {
	var remaining []PendingAction
	for _, a := range state.ActionQueue.Pending {
		if a.ID == actionID {
			state.ActionQueue.SkippedToday = append(
				state.ActionQueue.SkippedToday,
				fmt.Sprintf("%d:%s", a.ID, a.Type),
			)
		} else {
			remaining = append(remaining, a)
		}
	}
	state.ActionQueue.Pending = remaining
}

// defaultState returns a new default state
func (p *Provider) defaultState() *State {
	return &State{
		Version:     "1.0",
		LastUpdated: time.Now(),
		Briefings: Briefings{
			Morning:    BriefingState{},
			PreMeeting: BriefingState{},
			EOD:        BriefingState{},
			WeekAhead:  BriefingState{},
		},
		Streaks: Streaks{
			NeedleMover: NeedleMoverStreak{},
			Outreach:    OutreachStreak{WeeklyTarget: 10},
			Training:    TrainingStreak{},
		},
		Patterns: Patterns{
			Last7Days:      []string{},
			AvoidanceFlags: 0,
		},
		ActionQueue: ActionQueue{
			Pending:        []PendingAction{},
			CompletedToday: []string{},
			SkippedToday:   []string{},
		},
		PreparedMaterials: PreparedMaterials{
			OutreachDrafts: []OutreachDraft{},
			MeetingPrep:    []MeetingPrep{},
		},
		Thresholds: Thresholds{
			OutreachColdDays:      3,
			DeadlineWarningDays:   3,
			AvoidancePlanningDays: 3,
		},
	}
}

// expandPath expands ~ to home directory
func expandPath(path string) string {
	if len(path) > 0 && path[0] == '~' {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return filepath.Join(home, path[1:])
	}
	return path
}
