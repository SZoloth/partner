package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/szoloth/partner/internal/mcp"
)

// Task represents a Things 3 task
type Task struct {
	UUID          string     `json:"uuid"`
	Title         string     `json:"title"`
	Status        string     `json:"status"` // incomplete, completed, canceled
	Notes         string     `json:"notes,omitempty"`
	Tags          []string   `json:"tags,omitempty"`
	Deadline      *time.Time `json:"deadline,omitempty"`
	StartDate     *time.Time `json:"start_date,omitempty"`
	CreatedAt     *time.Time `json:"created_at,omitempty"`
	CompletedAt   *time.Time `json:"completed_at,omitempty"`
	ProjectUUID   string     `json:"project_uuid,omitempty"`
	ProjectTitle  string     `json:"project_title,omitempty"`
	AreaUUID      string     `json:"area_uuid,omitempty"`
	AreaTitle     string     `json:"area_title,omitempty"`
	ChecklistItems []ChecklistItem `json:"checklist_items,omitempty"`
}

// ChecklistItem represents a checklist item within a task
type ChecklistItem struct {
	Title  string `json:"title"`
	Status string `json:"status"`
}

// Project represents a Things 3 project
type Project struct {
	UUID      string     `json:"uuid"`
	Title     string     `json:"title"`
	Status    string     `json:"status"`
	Notes     string     `json:"notes,omitempty"`
	Tags      []string   `json:"tags,omitempty"`
	Deadline  *time.Time `json:"deadline,omitempty"`
	AreaUUID  string     `json:"area_uuid,omitempty"`
	AreaTitle string     `json:"area_title,omitempty"`
}

// Area represents a Things 3 area
type Area struct {
	UUID  string `json:"uuid"`
	Title string `json:"title"`
}

// ThingsProvider wraps the Things 3 MCP server
type ThingsProvider struct {
	client *mcp.Client
}

// NewThingsProvider creates a new Things provider
func NewThingsProvider(client *mcp.Client) *ThingsProvider {
	return &ThingsProvider{client: client}
}

// GetToday returns tasks due today
func (p *ThingsProvider) GetToday(ctx context.Context) ([]Task, error) {
	result, err := p.client.CallTool(ctx, "get_today", map[string]interface{}{})
	if err != nil {
		return nil, fmt.Errorf("get_today failed: %w", err)
	}

	return parseTasks(result)
}

// GetTodayDebug returns raw debug info for troubleshooting
func (p *ThingsProvider) GetTodayDebug(ctx context.Context) (map[string]interface{}, error) {
	result, err := p.client.CallTool(ctx, "get_today", map[string]interface{}{})
	if err != nil {
		return map[string]interface{}{
			"error": fmt.Sprintf("get_today failed: %v", err),
		}, nil
	}

	debug := map[string]interface{}{
		"content_count": len(result.Content),
		"is_error":      result.IsError,
	}

	if len(result.Content) > 0 {
		debug["first_content_type"] = result.Content[0].Type
		debug["first_content_text_len"] = len(result.Content[0].Text)
		if len(result.Content[0].Text) > 1000 {
			debug["first_content_preview"] = result.Content[0].Text[:1000]
		} else {
			debug["first_content_preview"] = result.Content[0].Text
		}
	}

	tasks, _ := parseTasks(result)
	debug["parsed_task_count"] = len(tasks)
	if len(tasks) > 0 {
		debug["first_task_title"] = tasks[0].Title
	}

	return debug, nil
}

// GetInbox returns inbox tasks
func (p *ThingsProvider) GetInbox(ctx context.Context) ([]Task, error) {
	result, err := p.client.CallTool(ctx, "get_inbox", map[string]interface{}{})
	if err != nil {
		return nil, fmt.Errorf("get_inbox failed: %w", err)
	}

	return parseTasks(result)
}

// GetUpcoming returns upcoming tasks
func (p *ThingsProvider) GetUpcoming(ctx context.Context) ([]Task, error) {
	result, err := p.client.CallTool(ctx, "get_upcoming", map[string]interface{}{})
	if err != nil {
		return nil, fmt.Errorf("get_upcoming failed: %w", err)
	}

	return parseTasks(result)
}

// GetAnytime returns anytime tasks
func (p *ThingsProvider) GetAnytime(ctx context.Context) ([]Task, error) {
	result, err := p.client.CallTool(ctx, "get_anytime", map[string]interface{}{})
	if err != nil {
		return nil, fmt.Errorf("get_anytime failed: %w", err)
	}

	return parseTasks(result)
}

// GetProjects returns all projects
func (p *ThingsProvider) GetProjects(ctx context.Context, includeItems bool) ([]Project, error) {
	args := map[string]interface{}{
		"include_items": includeItems,
	}

	result, err := p.client.CallTool(ctx, "get_projects", args)
	if err != nil {
		return nil, fmt.Errorf("get_projects failed: %w", err)
	}

	return parseProjects(result)
}

// GetAreas returns all areas
func (p *ThingsProvider) GetAreas(ctx context.Context, includeItems bool) ([]Area, error) {
	args := map[string]interface{}{
		"include_items": includeItems,
	}

	result, err := p.client.CallTool(ctx, "get_areas", args)
	if err != nil {
		return nil, fmt.Errorf("get_areas failed: %w", err)
	}

	return parseAreas(result)
}

// SearchTodos searches tasks by query
func (p *ThingsProvider) SearchTodos(ctx context.Context, query string) ([]Task, error) {
	args := map[string]interface{}{
		"query": query,
	}

	result, err := p.client.CallTool(ctx, "search_todos", args)
	if err != nil {
		return nil, fmt.Errorf("search_todos failed: %w", err)
	}

	return parseTasks(result)
}

// UpdateTodo updates a task
func (p *ThingsProvider) UpdateTodo(ctx context.Context, id string, updates map[string]interface{}) error {
	updates["id"] = id

	_, err := p.client.CallTool(ctx, "update_todo", updates)
	if err != nil {
		return fmt.Errorf("update_todo failed: %w", err)
	}

	return nil
}

// MarkComplete marks a task as completed
func (p *ThingsProvider) MarkComplete(ctx context.Context, id string) error {
	return p.UpdateTodo(ctx, id, map[string]interface{}{
		"completed": true,
	})
}

// Close closes the provider
func (p *ThingsProvider) Close() error {
	return p.client.Close()
}

// parseTasks parses the MCP tool result into tasks
// The Things MCP returns formatted text, not JSON
func parseTasks(result *mcp.ToolResult) ([]Task, error) {
	if len(result.Content) == 0 {
		return []Task{}, nil
	}

	var tasks []Task
	for _, block := range result.Content {
		if block.Type == "text" && block.Text != "" {
			// Split by task separator
			taskBlocks := strings.Split(block.Text, "\n---\n")
			for _, taskBlock := range taskBlocks {
				taskBlock = strings.TrimSpace(taskBlock)
				if taskBlock == "" {
					continue
				}
				task := parseTaskBlock(taskBlock)
				if task.Title != "" {
					tasks = append(tasks, task)
				}
			}
		}
	}

	return tasks, nil
}

// parseTaskBlock parses a single task from text format
func parseTaskBlock(block string) Task {
	task := Task{}
	lines := strings.Split(block, "\n")

	var notesBuilder strings.Builder
	inNotes := false
	inChecklist := false

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Handle checklist items
		if inChecklist {
			if strings.HasPrefix(line, "□") || strings.HasPrefix(line, "☑") {
				itemTitle := strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(line, "□"), "☑"))
				status := "incomplete"
				if strings.HasPrefix(line, "☑") {
					status = "completed"
				}
				task.ChecklistItems = append(task.ChecklistItems, ChecklistItem{
					Title:  itemTitle,
					Status: status,
				})
				continue
			} else if line != "" && !strings.Contains(line, ":") {
				// Continue checklist if indented
				continue
			}
			inChecklist = false
		}

		// Handle multi-line notes
		if inNotes {
			if strings.HasPrefix(line, "Project:") || strings.HasPrefix(line, "Tags:") ||
			   strings.HasPrefix(line, "Checklist:") || strings.HasPrefix(line, "Deadline:") {
				inNotes = false
				task.Notes = strings.TrimSpace(notesBuilder.String())
			} else {
				notesBuilder.WriteString(line)
				notesBuilder.WriteString("\n")
				continue
			}
		}

		// Parse key-value pairs
		if idx := strings.Index(line, ":"); idx > 0 {
			key := strings.TrimSpace(line[:idx])
			value := strings.TrimSpace(line[idx+1:])

			switch key {
			case "Title":
				task.Title = value
			case "UUID":
				task.UUID = value
			case "Status":
				task.Status = value
			case "Notes":
				inNotes = true
				notesBuilder.WriteString(value)
				notesBuilder.WriteString("\n")
			case "Project":
				task.ProjectTitle = value
			case "Tags":
				if value != "" {
					task.Tags = strings.Split(value, ", ")
				}
			case "Checklist":
				inChecklist = true
			case "Start Date":
				if t, err := time.Parse("2006-01-02", value); err == nil {
					task.StartDate = &t
				}
			case "Deadline":
				if t, err := time.Parse("2006-01-02", value); err == nil {
					task.Deadline = &t
				}
			}
		}
	}

	// Capture remaining notes
	if inNotes {
		task.Notes = strings.TrimSpace(notesBuilder.String())
	}

	return task
}

// parseProjects parses the MCP tool result into projects
func parseProjects(result *mcp.ToolResult) ([]Project, error) {
	if len(result.Content) == 0 {
		return []Project{}, nil
	}

	var projects []Project
	for _, block := range result.Content {
		if block.Type == "text" && block.Text != "" {
			if err := json.Unmarshal([]byte(block.Text), &projects); err != nil {
				continue
			}
		}
	}

	return projects, nil
}

// parseAreas parses the MCP tool result into areas
func parseAreas(result *mcp.ToolResult) ([]Area, error) {
	if len(result.Content) == 0 {
		return []Area{}, nil
	}

	var areas []Area
	for _, block := range result.Content {
		if block.Type == "text" && block.Text != "" {
			if err := json.Unmarshal([]byte(block.Text), &areas); err != nil {
				continue
			}
		}
	}

	return areas, nil
}
