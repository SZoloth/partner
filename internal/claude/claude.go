package claude

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// Client wraps the Claude CLI for AI assistance with session persistence
type Client struct {
	sessionID string // Persists context across calls
}

// NewClient creates a new Claude CLI client
func NewClient() *Client {
	return &Client{}
}

// Request represents a request to Claude
type Request struct {
	Prompt     string
	Context    string // Additional context (e.g., current task, calendar)
	MaxTokens  int
	AllowTools bool
	NewSession bool // Force a new session (ignore existing session_id)
}

// Response represents Claude's response
type Response struct {
	Text      string
	Error     error
	Action    *Action // Suggested action, if any
	SessionID string  // For context persistence
	Usage     *Usage  // Token usage stats
}

// Usage tracks token consumption
type Usage struct {
	InputTokens  int
	OutputTokens int
	CostUSD      float64
	DurationMs   int
}

// Action represents a suggested action from Claude
type Action struct {
	Type        ActionType
	Description string
	Data        map[string]interface{}
}

// ActionType categorizes AI-suggested actions
type ActionType int

const (
	ActionNone ActionType = iota
	ActionCompleteTask
	ActionDraftEmail
	ActionCreateTask
	ActionScheduleEvent
	ActionSummarize
)

// CLIResponse represents the JSON output from claude CLI
type CLIResponse struct {
	Type         string  `json:"type"`
	Subtype      string  `json:"subtype"`
	IsError      bool    `json:"is_error"`
	DurationMs   int     `json:"duration_ms"`
	NumTurns     int     `json:"num_turns"`
	Result       string  `json:"result"`
	SessionID    string  `json:"session_id"`
	TotalCostUSD float64 `json:"total_cost_usd"`
	Usage        struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

// GetSessionID returns the current session ID
func (c *Client) GetSessionID() string {
	return c.sessionID
}

// ClearSession starts fresh without context
func (c *Client) ClearSession() {
	c.sessionID = ""
}

// Ask sends a prompt to Claude and returns the response with session persistence
func (c *Client) Ask(ctx context.Context, req Request) Response {
	// Build the prompt with context
	fullPrompt := req.Prompt
	if req.Context != "" {
		fullPrompt = fmt.Sprintf("Context:\n%s\n\nRequest:\n%s", req.Context, req.Prompt)
	}

	// Build args with JSON output for structured parsing
	args := []string{"-p", fullPrompt, "--output-format", "json"}

	// Use existing session for context persistence (unless new session requested)
	if c.sessionID != "" && !req.NewSession {
		args = append(args, "--session-id", c.sessionID)
	}

	cmd := exec.CommandContext(ctx, "claude", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return Response{
			Error: fmt.Errorf("claude command failed: %w (stderr: %s)", err, stderr.String()),
		}
	}

	// Parse JSON response
	var cliResp CLIResponse
	if err := json.Unmarshal(stdout.Bytes(), &cliResp); err != nil {
		// Fallback to raw text if JSON parsing fails
		return Response{
			Text:   strings.TrimSpace(stdout.String()),
			Action: c.parseAction(stdout.String()),
		}
	}

	// Check for API errors
	if cliResp.IsError {
		return Response{
			Error: fmt.Errorf("claude API error: %s", cliResp.Result),
		}
	}

	// Update session ID for next call
	if cliResp.SessionID != "" {
		c.sessionID = cliResp.SessionID
	}

	text := cliResp.Result

	return Response{
		Text:      text,
		SessionID: cliResp.SessionID,
		Action:    c.parseAction(text),
		Usage: &Usage{
			InputTokens:  cliResp.Usage.InputTokens,
			OutputTokens: cliResp.Usage.OutputTokens,
			CostUSD:      cliResp.TotalCostUSD,
			DurationMs:   cliResp.DurationMs,
		},
	}
}

// Continue sends a follow-up message in the existing session
func (c *Client) Continue(ctx context.Context, prompt string) Response {
	if c.sessionID == "" {
		return Response{
			Error: fmt.Errorf("no active session - use Ask() first"),
		}
	}
	return c.Ask(ctx, Request{Prompt: prompt})
}

// TaskBreakdown asks Claude to break down a task
func (c *Client) TaskBreakdown(ctx context.Context, taskTitle string, taskNotes string) Response {
	prompt := fmt.Sprintf(`Break down this task into 3-5 actionable subtasks:

Task: %s
Notes: %s

Provide a numbered list of concrete next steps. Keep each step small and completable in one session.`, taskTitle, taskNotes)

	return c.Ask(ctx, Request{Prompt: prompt})
}

// DraftEmail asks Claude to draft an email
func (c *Client) DraftEmail(ctx context.Context, recipient, subject, context string) Response {
	prompt := fmt.Sprintf(`Draft a brief, professional email:

To: %s
Subject: %s
Context: %s

Keep it concise (under 100 words). Include a clear call-to-action.`, recipient, subject, context)

	return c.Ask(ctx, Request{Prompt: prompt})
}

// Summarize asks Claude to summarize content
func (c *Client) Summarize(ctx context.Context, content string) Response {
	prompt := fmt.Sprintf(`Summarize the following in 2-3 bullet points:

%s`, content)

	return c.Ask(ctx, Request{Prompt: prompt})
}

// NeedleMover asks Claude to identify the most important task
func (c *Client) NeedleMover(ctx context.Context, tasks []string, goals string) Response {
	taskList := strings.Join(tasks, "\n- ")
	prompt := fmt.Sprintf(`Given these tasks and goals, which ONE task is the highest-leverage needle-mover right now?

Tasks:
- %s

Goals: %s

Identify the single most impactful task and briefly explain why (1-2 sentences).`, taskList, goals)

	return c.Ask(ctx, Request{Prompt: prompt})
}

// parseAction extracts suggested actions from Claude's response
func (c *Client) parseAction(text string) *Action {
	lower := strings.ToLower(text)

	// Simple heuristics for action detection
	if strings.Contains(lower, "i suggest completing") || strings.Contains(lower, "mark as done") {
		return &Action{
			Type:        ActionCompleteTask,
			Description: "Complete task",
		}
	}

	if strings.Contains(lower, "draft email") || strings.Contains(lower, "send an email") {
		return &Action{
			Type:        ActionDraftEmail,
			Description: "Draft email",
		}
	}

	if strings.Contains(lower, "create a task") || strings.Contains(lower, "add a task") {
		return &Action{
			Type:        ActionCreateTask,
			Description: "Create task",
		}
	}

	return nil
}

// CheckAvailable verifies the Claude CLI is installed and authenticated
func CheckAvailable() error {
	cmd := exec.Command("claude", "--version")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("claude CLI not available: %w", err)
	}
	return nil
}
