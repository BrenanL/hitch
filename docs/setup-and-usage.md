# Setup and Usage

## Prerequisites

- Go 1.24+ installed
- Claude Code installed and configured

## Build

```bash
go build -o ht ./cmd/ht
```

Or with version info baked in:

```bash
go build -ldflags "-X github.com/BrenanL/hitch/internal/cli.Version=0.1.0" -o ht ./cmd/ht
```

## Initialize

```bash
# Create ~/.hitch/ directories, state.db, and install system hooks
./ht init --global
```

This creates:
- `~/.hitch/state.db` — SQLite database (WAL mode)
- `~/.hitch/deny-lists/` — custom deny list directory
- System hooks in `~/.claude/settings.json` for session tracking

## Add Notification Channels

```bash
# ntfy (simplest — no auth required for public topics)
./ht channel add ntfy my-topic-name

# Discord webhook
./ht channel add discord https://discord.com/api/webhooks/...

# Slack webhook
./ht channel add slack https://hooks.slack.com/services/...

# Desktop notifications (uses OS-native: notify-send, osascript, or PowerShell on WSL)
./ht channel add desktop default

# Test that a channel works
./ht channel test ntfy

# List channels
./ht channel list
```

## Add Rules

Rules use the hitch DSL. The format is:

```
on <event> -> <action> [-> <action>...] [if <condition>]
```

### Examples

```bash
# Notify when Claude stops (only if session was long)
./ht rule add 'on stop -> notify ntfy if elapsed > 30s'

# Block destructive commands
./ht rule add 'on pre-bash -> deny "blocked by safety rule" if command matches deny-list:destructive'

# Guard .env files
./ht rule add 'on pre-edit -> deny "protected file" if file matches "\.env"'

# Chain actions: summarize then notify
./ht rule add 'on stop -> summarize -> notify slack if elapsed > 5m'

# Notify on permission prompts when idle
./ht rule add 'on notification:permission -> notify ntfy if idle > 60s'

# Log all tool use
./ht rule add 'on pre-tool -> log'
```

### Events

| DSL Event | Claude Code Event | Default Matcher |
|---|---|---|
| `stop` | Stop | — |
| `pre-tool` | PreToolUse | — |
| `post-tool` | PostToolUse | — |
| `pre-bash` | PreToolUse | Bash |
| `post-bash` | PostToolUse | Bash |
| `pre-edit` | PreToolUse | Edit\|Write |
| `post-edit` | PostToolUse | Edit\|Write |
| `notification` | Notification | — |
| `session-start` | SessionStart | — |
| `session-end` | SessionEnd | — |
| `prompt` | UserPromptSubmit | — |
| `permission` | PermissionRequest | — |

### Conditions

- `elapsed > 30s` / `elapsed > 5m` — time since session start
- `away` / `focused` / `idle > 60s` — focus state
- `matches "pattern"` — regex match against default target
- `command matches "pattern"` — regex match against command
- `file matches "pattern"` — regex match against file path
- `matches deny-list:name` — check against a deny list
- `not <condition>` — negation
- `<condition> and <condition>` — both must be true
- `<condition> or <condition>` — either must be true

## Sync to Claude Code

```bash
# Write hook entries to ~/.claude/settings.json
./ht sync

# Preview without writing
./ht sync --dry-run
```

Sync reads all enabled rules from the database, generates settings.json hook entries with ownership markers (`# ht:rule-<id>`), and merges them into the settings file. It never modifies hooks it didn't create.

## Manage Rules

```bash
./ht rule list                   # List all rules
./ht rule disable <id>           # Disable without removing
./ht rule enable <id>            # Re-enable
./ht rule remove <id>            # Delete permanently
```

## Built-in Packages

Packages are pre-built rule bundles:

```bash
./ht package list                # Show available packages
./ht package show notifier       # Show rules in a package
./ht package enable notifier     # Install package rules
./ht package disable notifier    # Remove package rules
```

Available packages:
- **notifier** — stop notify, permission alert, idle alert
- **safety** — destructive blocker, .env guard, force-push blocker
- **quality** — test gate, lint gate
- **observer** — event logging

## Other Commands

```bash
./ht status                      # Dashboard: channels, rules, mute state, recent events
./ht log                         # View event log
./ht log --session <id>          # Filter by session
./ht log --event Stop            # Filter by event type
./ht mute 30m                    # Silence notifications for 30 minutes
./ht unmute                      # Resume notifications
./ht notify ntfy "test message"  # Send a direct notification
./ht export                      # Export rules as DSL
./ht import rules.hitch          # Import rules from a .hitch file
./ht deny-list list              # Show deny lists
./ht deny-list show destructive  # Show patterns in a list
./ht deny-list add custom "pattern"  # Add a custom pattern
```

## How It Works

1. You write rules in the hitch DSL
2. `ht sync` generates hook entries in `~/.claude/settings.json`
3. When Claude Code fires a hook, it runs `ht hook exec <rule-id>`, piping JSON on stdin
4. Hitch evaluates conditions against the input
5. If conditions pass, hitch executes actions (notify, deny, run, etc.)
6. Hitch writes JSON to stdout (allow/deny/context)
7. Exit code 0 = allow, exit code 2 = block
8. Events are logged to SQLite for `ht log` and `ht status`

## Directory Layout

```
~/.hitch/
  state.db              # SQLite database
  deny-lists/           # Custom deny list .txt files
  credentials.enc       # age-encrypted credentials (optional)
  manifest.json         # Tracks owned settings.json entries
  sync.lock             # File lock for concurrent access

~/.claude/
  settings.json         # Claude Code settings (hitch writes hook entries here)
```
