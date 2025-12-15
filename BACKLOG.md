# Partner Backlog

## Inspiration
- **pkiv CEO CLI** - Claude Agent SDK-powered executive assistant
- **JFDI System** - "Just F***ing Do It" productivity methodology

---

## Phase 5: Knowledge + CRM + Projects (from plan)
- [ ] Apple Notes integration via Apple Data MCP
- [ ] Readwise highlights integration
- [ ] Supermemory integration
- [ ] Contacts + relationship tracking ("losing touch" alerts)
- [ ] Projects/goals visualization
- [ ] Global search (`/` or `?`)

## Phase 6: Polish (from plan)
- [ ] Multiple themes (Catppuccin Mocha already implemented, add more)
- [ ] Configuration file loading (`configs/default.yaml`)
- [ ] Performance optimization (parallel MCP calls)
- [ ] Error handling and offline mode
- [ ] Graceful MCP reconnection

---

## Assistant Enhancements

### Model Switcher
- [ ] Gemini support via claude-code-proxy or direct API
- [ ] Codex/OpenAI support
- [ ] Model indicator in status bar
- [ ] Keybinding to switch models (e.g., `m`)

### Chat Experience
- [ ] Full chat sidebar (not just modal)
- [ ] Multi-turn conversation history display
- [ ] Action execution from chat (not just suggestions)
- [ ] Streaming responses with live typing
- [ ] Loading/working state indicators (spinner, progress)

### AI Actions
- [ ] Actually execute suggested actions (complete task, create task, draft email)
- [ ] Confirmation before destructive actions
- [ ] Undo support for AI actions

---

## Tasks Pane

### Interactive Mode
- [ ] Inline task editing (title, notes)
- [ ] Add new task from TUI
- [ ] Move tasks between lists (Today, Inbox, Someday)
- [ ] Project/area assignment
- [ ] Tag management
- [ ] Due date picker

### Display
- [ ] Expand/collapse task details
- [ ] Show task notes inline
- [ ] Subtask display and completion
- [ ] Project grouping view

---

## Calendar Pane

### Display
- [ ] Show end times (not just start)
- [ ] Duration indicator
- [ ] Week view mode
- [ ] Month view mode
- [ ] Event details expansion

### Interactions
- [ ] Quick add event
- [ ] RSVP from TUI
- [ ] Join meeting link shortcut
- [ ] Conflict highlighting

---

## Projects Pane

- [ ] Goals tracking with progress bars
- [ ] Project health indicators
- [ ] Milestone visualization
- [ ] OKR integration
- [ ] Weekly review mode

---

## Email Pane (Phase 3 - not started)

- [ ] Gmail MCP integration
- [ ] Inbox list with preview
- [ ] Compose/reply modal
- [ ] Archive/delete shortcuts
- [ ] Label management
- [ ] AI-powered email drafting

---

## Technical Debt

- [ ] Extract hardcoded paths (Things MCP script, credentials)
- [ ] Add configuration file support
- [ ] Unit tests for MCP providers
- [ ] Integration tests for TUI
- [ ] CI/CD pipeline
- [ ] Release binaries (goreleaser)

---

## Markdown Viewer/Editor

- [ ] Browse and view `.md` files in TUI
- [ ] Rendered markdown preview (glamour)
- [ ] Edit mode with syntax highlighting
- [ ] Quick open recent files
- [ ] Integration with Knowledge pane (notes as markdown)
- [ ] BACKLOG.md / README.md quick access

---

## Ideas / Someday

- [ ] Vim mode for text input
- [ ] Plugin system for custom panes
- [ ] Webhook/notification support
- [ ] Mobile companion (via SSH?)
- [ ] Voice input via Whisper
- [ ] Daily/weekly summary generation
- [ ] Pomodoro timer integration
- [ ] Habit tracking pane
