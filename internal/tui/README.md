# internal/tui

Bubbletea TUI for `ht settings`. Renders a five-tab terminal interface for inspecting Claude Code configuration.

## Launching

```go
m := tui.New()
p := tea.NewProgram(m, tea.WithAltScreen())
p.Run()
```

`New()` creates the top-level `Model` using the current working directory to locate project-scoped configuration files.

## Tabs

| # | Title | Type | Purpose |
|---|-------|------|---------|
| 1 | Settings | `SettingsTab` | All Claude Code settings keys grouped by category with effective values and scope badges |
| 2 | Env Vars | `*EnvVarsTab` | All Claude Code environment variables from the `pkg/envvars` registry, showing set/unset state and masking sensitive values |
| 3 | Hooks | `*HooksTab` | Hooks configured across all settings scopes (user, project, local, managed), grouped by event name |
| 4 | Memory | `*MemoryTab` | Contents of the project MEMORY.md file at `~/.claude/projects/<encoded-cwd>/memory/MEMORY.md` |
| 5 | Explorer | `*ExplorerTab` | Project structure overview: key directories, `~/.claude/` tree, and `.claude/` (project-scoped) tree |

## tabModel Interface

Every tab implements:

```go
type tabModel interface {
    Init() tea.Cmd
    Update(tea.Msg) (tabModel, tea.Cmd)
    View() string
    Title() string
}
```

`Init` returns startup commands (all tabs return nil). `Update` handles keyboard navigation. `View` renders the tab content as a string. `Title` returns the display name shown in the tab bar.

## Key Bindings

### Global (handled by Model)

| Key | Action |
|-----|--------|
| `Tab` | Next tab (wraps) |
| `Shift+Tab` | Previous tab (wraps) |
| `1`–`5` | Jump to tab by number |
| `?` | Toggle help overlay |
| `q` / `Ctrl+C` | Quit |

### Settings tab (Tab 1)

| Key | Action |
|-----|--------|
| `j` / `↓` | Move cursor down |
| `k` / `↑` | Move cursor up |
| `PgDn` / `Ctrl+D` | Page down (10 rows) |
| `PgUp` / `Ctrl+U` | Page up (10 rows) |
| `g` | Go to top |
| `G` | Go to bottom |
| `/` | Enter filter mode |
| `Esc` | Clear filter |
| `Backspace` | Delete last filter character |
| `Enter` | Collapse/expand category |

### Env Vars tab (Tab 2)

| Key | Action |
|-----|--------|
| `j` / `↓` | Move cursor down (skips category headers) |
| `k` / `↑` | Move cursor up (skips category headers) |
| `/` | Enter filter mode |
| `Enter` | Commit filter, exit filter mode |
| `Esc` | Clear filter |
| `Backspace` | Delete last filter character |

### Hooks tab (Tab 3)

| Key | Action |
|-----|--------|
| `j` / `↓` | Move cursor down |
| `k` / `↑` | Move cursor up |
| `/` | Enter filter mode |
| `Enter` | Commit filter, exit filter mode |
| `Esc` | Clear filter |
| `Backspace` | Delete last filter character |

### Memory tab (Tab 4)

| Key | Action |
|-----|--------|
| `j` / `↓` | Scroll down one line |
| `k` / `↑` | Scroll up one line |
| `g` | Scroll to top |
| `G` | Scroll to bottom |

### Explorer tab (Tab 5)

| Key | Action |
|-----|--------|
| `j` / `↓` | Scroll down one line |
| `k` / `↑` | Scroll up one line |
| `g` | Scroll to top |
| `G` | Scroll to bottom |

## Scope Badges (Settings Tab)

Keys that have a value set in a settings file show a one-letter scope badge:

| Badge | Scope | Source file |
|-------|-------|-------------|
| `[U]` | User | `~/.claude/settings.json` |
| `[P]` | Project | `.claude/settings.json` |
| `[L]` | Local | `.claude/settings.local.json` |
| `[M]` | Managed | `~/.claude/settings.managed.json` |
| `(default)` | — | Schema default, no file |
