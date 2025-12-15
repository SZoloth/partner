# Partner

A keyboard-driven TUI personal operating system built with Go + Bubbletea. Integrates tasks, calendar, email, knowledge, CRM, and projects into a unified interface with Claude AI assistance.

![Partner TUI](https://img.shields.io/badge/TUI-Bubbletea-blue) ![Go](https://img.shields.io/badge/Go-1.22+-00ADD8) ![Claude](https://img.shields.io/badge/AI-Claude-orange)

## Features

- **Multi-pane layout** - Single, split, or 2x2 grid views
- **Things 3 integration** - View and complete today's tasks
- **Google Calendar** - See your schedule at a glance
- **Claude AI assist** - Get needle-mover recommendations with session persistence
- **Keyboard-driven** - Vim-style navigation throughout

## Installation

```bash
go install github.com/szoloth/partner/cmd/partner@latest
```

Or build from source:

```bash
git clone https://github.com/szoloth/partner.git
cd partner
go build ./cmd/partner
```

## Requirements

- Go 1.22+
- Claude CLI (`claude`) installed and authenticated
- Things 3 (macOS) with [things-mcp](https://github.com/nicholascloud/things-mcp)
- Google Calendar credentials for calendar integration

## Usage

```bash
# Interactive TUI
partner

# Start with specific pane
partner --pane calendar

# Headless mode (for automation)
partner --json --pane tasks
```

## Keybindings

### Global
| Key | Action |
|-----|--------|
| `q` | Quit |
| `Tab` | Cycle pane focus |
| `1-6` | Jump to pane |
| `\` | Cycle layouts (single → split-h → split-v → grid) |
| `Ctrl+w o` | Maximize/restore current pane |
| `a` | AI assist (Claude) |

### Within Panes
| Key | Action |
|-----|--------|
| `j/k` | Navigate up/down |
| `d` | Mark task done |
| `r` | Refresh data |
| `Space` | Select/toggle |

### AI Modal
| Key | Action |
|-----|--------|
| `c` | Continue conversation |
| `Enter` | Execute suggested action |
| `Esc` | Close and clear session |

## Architecture

```
partner/
├── cmd/partner/          # Entry point
├── internal/
│   ├── app/              # Root Bubbletea model
│   ├── panes/            # Tasks, Calendar, etc.
│   ├── mcp/              # MCP client + providers
│   ├── claude/           # Claude CLI wrapper (session-based)
│   └── theme/            # Lipgloss themes
└── scripts/              # MCP wrapper scripts
```

## Configuration

Partner uses MCP (Model Context Protocol) servers for data integration:

- **Things 3**: Local Python MCP server
- **Google Calendar**: `@cocal/google-calendar-mcp`

See `scripts/things-mcp.sh` for the Things 3 setup.

## Roadmap

See [BACKLOG.md](BACKLOG.md) for planned features including:
- Email integration (Gmail)
- Knowledge pane (Apple Notes, Readwise)
- CRM with "losing touch" alerts
- Multi-model AI support (Gemini, Codex)

## License

MIT
