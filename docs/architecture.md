# Architecture

## System overview

```
+------------------------------------------+
|             CLI Interface                 |
|   ht init, ht channel, ht rule, ht hook  |
+------------------------------------------+
|             DSL Parser                    |
|   Parse .hitch rule files                |
|   Validate syntax and semantics          |
+------------------------------------------+
|           Hook Generator                  |
|   Produce settings.json entries          |
|   Manage manifest (ownership tracking)   |
+------------------------------------------+
|         Core Engine                       |
|   Hook I/O (stdin/stdout JSON)           |
|   Condition evaluator                    |
|   State manager (SQLite)                 |
|   Transcript reader                      |
+------------------------------------------+
|         Channel Adapters                  |
|   ntfy | Discord | Slack | Desktop       |
|   (Telegram | Pushover | SMS | Email)    |
+------------------------------------------+
|         Claude Code Hooks API             |
|   stdin JSON | stdout JSON | exit codes  |
+------------------------------------------+
```

## Directory structure (project)

```
hitch/
  cmd/
    ht/
      main.go                # CLI entrypoint
  internal/
    cli/                     # Cobra command definitions
      init.go
      channel.go
      rule.go
      hook.go
      status.go
      sync.go
    dsl/
      parser.go              # DSL parser
      ast.go                 # AST types
      validator.go           # Semantic validation
    engine/
      executor.go            # Hook execution logic
      conditions.go          # Condition evaluation
      actions.go             # Action handlers (notify, deny, run, etc.)
    generator/
      settings.go            # settings.json read/merge/write
      manifest.go            # Ownership tracking
      scripts.go             # Custom hook script management
    adapters/
      adapter.go             # Interface definition
      ntfy.go
      discord.go
      slack.go
      desktop.go
    state/
      db.go                  # SQLite connection, migrations
      events.go              # Event logging
      sessions.go            # Session tracking
      kv.go                  # Key-value state
      mute.go                # Mute state
    credentials/
      store.go               # age encryption, env var fallback
    platform/
      detect.go              # OS/WSL detection
      notify.go              # Platform-specific notifications
      sound.go               # Platform-specific sound playback
  pkg/
    hookio/
      input.go               # Parse Claude Code hook stdin JSON
      output.go              # Build hook stdout JSON responses
      types.go               # Shared types for hook I/O
  go.mod
  go.sum
  README.md
  LICENSE
  docs/
```

The `internal/` packages are not importable by external tools. The `pkg/hookio/` package IS importable — it's the library face of hitch that other tools can use to parse hook input and build hook output.

---

## DSL specification

### Grammar

```ebnf
file        = { rule | comment | blank } ;
rule        = "on" event [ matcher ] "->" actions [ condition ] ;
comment     = "#" { any character except newline } ;
blank       = newline ;

event       = "stop" | "pre-tool" | "post-tool" | "tool-failure"
            | "pre-bash" | "post-bash" | "pre-edit" | "post-edit"
            | "notification" | "permission"
            | "session-start" | "session-end"
            | "pre-compact"
            | "subagent-start" | "subagent-stop" ;

matcher     = ":" match_value ;
match_value = identifier | quoted_string ;

actions     = action { "->" action } ;
action      = notify_action | run_action | deny_action
            | require_action | summarize_action | log_action ;

notify_action   = "notify" channel_name [ message ] ;
run_action      = "run" quoted_string [ "async" ] ;
deny_action     = "deny" [ quoted_string ] ;
require_action  = "require" check_name ;
summarize_action = "summarize" ;
log_action      = "log" [ target ] ;

channel_name = identifier ;
check_name   = identifier | identifier "-" identifier ;
message      = quoted_string ;
target       = identifier ;

condition    = "if" expr ;
expr         = simple_expr { ("and" | "or") simple_expr } ;
simple_expr  = time_expr | focus_expr | match_expr | not_expr ;
time_expr    = "elapsed" comparison duration ;
focus_expr   = "away" | "focused" | "idle" comparison duration ;
match_expr   = "matches" pattern
             | "file" "matches" pattern
             | "command" "matches" pattern ;
not_expr     = "not" simple_expr ;

comparison   = ">" | "<" | ">=" | "<=" | "=" ;
duration     = number unit ;
unit         = "s" | "m" | "h" ;
number       = digit { digit } ;
pattern      = quoted_string | "deny-list:" identifier ;
identifier   = letter { letter | digit | "-" | "_" } ;
quoted_string = '"' { any character except '"' | '\\"' } '"' ;
```

### Event mapping

DSL events map to Claude Code hook events:

| DSL Event | Claude Code Event | Default Matcher |
|---|---|---|
| `stop` | `Stop` | (none — always fires) |
| `pre-tool` | `PreToolUse` | `*` |
| `post-tool` | `PostToolUse` | `*` |
| `tool-failure` | `PostToolUseFailure` | `*` |
| `pre-bash` | `PreToolUse` | `Bash` |
| `post-bash` | `PostToolUse` | `Bash` |
| `pre-edit` | `PreToolUse` | `Edit\|Write` |
| `post-edit` | `PostToolUse` | `Edit\|Write` |
| `notification` | `Notification` | `*` |
| `notification:permission` | `Notification` | `permission_prompt` |
| `notification:idle` | `Notification` | `idle_prompt` |
| `permission` | `PermissionRequest` | `*` |
| `session-start` | `SessionStart` | `*` |
| `session-end` | `SessionEnd` | `*` |
| `pre-compact` | `PreCompact` | `*` |
| `subagent-start` | `SubagentStart` | `*` |
| `subagent-stop` | `SubagentStop` | `*` |

The shorthand events (`pre-bash`, `post-edit`, etc.) are sugar that sets both the Claude Code event and the matcher. `pre-tool:"Glob"` sets `PreToolUse` with matcher `Glob`.

### Examples with parse results

```
on stop -> notify discord if elapsed > 30s
```
Parses to: event=Stop, action=notify(discord), condition=elapsed>30s

```
on pre-bash -> deny if matches "rm -rf"
```
Parses to: event=PreToolUse, matcher=Bash, action=deny, condition=command_matches("rm -rf")

```
on post-edit -> run "npm test" async
```
Parses to: event=PostToolUse, matcher=Edit|Write, action=run("npm test", async=true)

```
on stop -> summarize -> notify slack
```
Parses to: event=Stop, actions=[summarize, notify(slack)] (chained)

```
on notification:permission -> notify sms "Claude needs permission"
```
Parses to: event=Notification, matcher=permission_prompt, action=notify(sms, "Claude needs permission")

```
on pre-bash -> deny if matches deny-list:destructive
```
Parses to: event=PreToolUse, matcher=Bash, action=deny, condition=command_matches(builtin_deny_list("destructive"))

### Error handling

The parser should produce clear error messages with line numbers:

```
rules.hitch:3: unknown event 'pre-read' — did you mean 'pre-tool'?
rules.hitch:7: unknown channel 'discrod' — configured channels: ntfy, discord, slack
rules.hitch:12: condition 'elapsed > 30' missing time unit (use s, m, or h)
```

Unknown channels should be a warning, not an error — the channel might be configured later.

### Built-in deny lists

Hitch ships with curated deny lists. `deny-list:destructive` includes:

```
rm -rf /
rm -rf ~
rm -rf .
rm -rf /*
sudo rm
mkfs
dd if=
:(){ :|:& };:
chmod -R 777 /
chown -R
git push --force origin main
git push --force origin master
DROP DATABASE
DROP TABLE
TRUNCATE TABLE
shutdown
reboot
init 0
halt
killall
```

Users can extend or create custom deny lists in `~/.hitch/deny-lists/` as plain text files (one pattern per line).

---

## Hook execution

### How Claude Code invokes hitch

When hitch syncs a rule, it writes a hook entry into settings.json whose command calls the `ht` binary:

```json
{
  "hooks": {
    "Stop": [
      {
        "matcher": "",
        "hooks": [
          {
            "type": "command",
            "command": "ht hook exec rule-a1b2c3 # ht:rule-a1b2c3"
          }
        ]
      }
    ]
  }
}
```

The comment `# ht:rule-a1b2c3` is the ownership marker that lets hitch identify its own entries during sync.

### Execution flow

When Claude Code fires a hook:

1. Claude Code pipes JSON to stdin and runs `ht hook exec rule-a1b2c3`
2. `ht` reads stdin JSON, parses it into a typed `HookInput` struct
3. `ht` looks up rule `a1b2c3` in SQLite
4. `ht` evaluates the rule's conditions against the input (elapsed time, pattern matches, focus state, etc.)
5. If conditions pass, `ht` executes the rule's actions (notify, deny, run, etc.)
6. `ht` writes JSON response to stdout (decision, reason, context)
7. `ht` logs the event to SQLite
8. `ht` exits with appropriate exit code (0 = allow, 2 = block)

### Custom hook execution

For custom hooks (`run hook:my-custom-check`), step 4-6 change:

1. `ht` looks up `my-custom-check` in the hooks manifest
2. `ht` finds the script at `.hitch/hooks/my-custom-check.sh` (or `~/.hitch/hooks/`)
3. `ht` pipes the original stdin JSON to the script
4. `ht` captures the script's stdout and exit code
5. `ht` passes through the script's output to Claude Code

Custom scripts receive the same JSON stdin as built-in hooks and are expected to produce the same JSON stdout format. They can use `ht` subcommands for convenience (e.g., `ht notify discord "message"` from within a script).

### Custom hooks manifest

`~/.hitch/hooks/manifest.json` (global) and `.hitch/hooks/manifest.json` (project):

```json
{
  "hooks": {
    "my-custom-check": {
      "path": "my-custom-check.sh",
      "description": "Custom validation before stopping",
      "created_at": "2025-01-15T10:30:00Z",
      "created_by": "user"
    },
    "deploy-staging": {
      "path": "deploy-staging.sh",
      "description": "Deploy to staging after tests pass",
      "created_at": "2025-01-15T11:00:00Z",
      "created_by": "claude-agent"
    }
  }
}
```

The `created_by` field tracks whether a human or an agent created the hook. Scripts must be executable (`chmod +x`).

---

## settings.json sync

### The problem

Claude Code reads hooks from `~/.claude/settings.json` (global) and `.claude/settings.json` (project). These files contain more than hooks — permissions, preferences, MCP servers, etc. Hitch must modify only the hooks it owns without disturbing anything else.

### Manifest format

`~/.hitch/manifest.json`:

```json
{
  "version": 1,
  "scope": "global",
  "settings_path": "~/.claude/settings.json",
  "rules": {
    "a1b2c3": {
      "dsl": "on stop -> notify discord if elapsed > 30s",
      "event": "Stop",
      "matcher": "",
      "marker": "# ht:rule-a1b2c3",
      "generated_at": "2025-01-15T10:30:00Z"
    },
    "d4e5f6": {
      "dsl": "on pre-bash -> deny if matches deny-list:destructive",
      "event": "PreToolUse",
      "matcher": "Bash",
      "marker": "# ht:rule-d4e5f6",
      "generated_at": "2025-01-15T10:30:00Z"
    }
  }
}
```

### Sync algorithm

```
function sync(scope):
  manifest = read_manifest(scope)
  settings = read_settings_json(scope)
  if settings is malformed:
    warn("settings.json has invalid JSON, refusing to modify")
    return error

  hooks = settings["hooks"] or {}

  # Remove old hitch entries
  for each event_type in hooks:
    for each matcher_group in hooks[event_type]:
      remove hook handlers whose command contains any marker from manifest

  # Remove empty matcher groups and event types after cleanup
  prune_empty(hooks)

  # Add current entries
  rules = read_rules(scope)
  for each rule in rules:
    if rule.enabled:
      entry = generate_hook_entry(rule)
      hooks[rule.event][rule.matcher_group].hooks.append(entry)

  # Write back
  settings["hooks"] = hooks
  write_settings_json(settings, scope)
  update_manifest(rules, scope)
```

### Identifying owned hooks

Each hook command hitch writes includes a trailing comment: `ht hook exec rule-a1b2c3 # ht:rule-a1b2c3`. During sync, hitch scans all hook commands for the `# ht:` prefix to identify its own entries. The manifest provides a cross-check.

If a hook has a hitch marker but isn't in the manifest (user manually edited it), hitch warns but doesn't touch it.

### File locking

Use `flock` (Linux/macOS) on a lockfile at `~/.hitch/sync.lock` to prevent concurrent sync operations. Lock is held only during the read-modify-write cycle.

---

## Channel adapter interface

```go
// adapters/adapter.go

type Message struct {
    Title   string            // Short title (e.g., "Claude finished")
    Body    string            // Main message content
    Level   Level             // Info, Warning, Error
    Fields  map[string]string // Structured key-value pairs
    Event   string            // Hook event name
    Session string            // Session ID
}

type Level int

const (
    Info Level = iota
    Warning
    Error
)

type SendResult struct {
    Success   bool
    Error     error
    Retryable bool
}

type Adapter interface {
    // Name returns the adapter identifier (e.g., "ntfy", "discord")
    Name() string

    // Send delivers a message through this channel.
    // Returns immediately — no retries. Caller decides retry policy.
    Send(ctx context.Context, msg Message) SendResult

    // Test sends a test message to verify configuration.
    Test(ctx context.Context) SendResult

    // ValidateConfig checks that the adapter's configuration is complete
    // and well-formed, without actually sending anything.
    ValidateConfig() error
}
```

### Adapter configuration

Each adapter reads its config from the credential store. Config keys follow the pattern `<adapter>.<field>`:

| Adapter | Config Keys | Example |
|---|---|---|
| ntfy | `ntfy.topic`, `ntfy.server` (optional, defaults to ntfy.sh) | `ntfy.topic=my-alerts` |
| discord | `discord.webhook_url` | `discord.webhook_url=https://discord.com/api/webhooks/...` |
| slack | `slack.webhook_url` | `slack.webhook_url=https://hooks.slack.com/...` |
| desktop | (none — uses OS-native) | |

### Message formatting

Each adapter formats the `Message` struct appropriately for its platform:

- **ntfy:** Title becomes the notification title, Body becomes the message, Level maps to priority (info=default, warning=high, error=urgent)
- **Discord:** Formats as an embed with title, description (body), color (green/yellow/red by level), and fields as embed fields
- **Slack:** Formats as a Block Kit message with header, body section, and fields as context blocks
- **Desktop:** Uses `notify-send` (Linux), `osascript` (macOS), or `powershell.exe` (WSL)

---

## SQLite schema

```sql
-- Schema version tracking
CREATE TABLE schema_version (
    version INTEGER PRIMARY KEY
);

-- Configured notification channels
CREATE TABLE channels (
    id TEXT PRIMARY KEY,              -- e.g., "ntfy", "discord", "my-slack"
    adapter TEXT NOT NULL,            -- adapter name: "ntfy", "discord", "slack"
    name TEXT NOT NULL,               -- display name
    config TEXT NOT NULL DEFAULT '{}', -- JSON: non-secret config (server URL, etc.)
    enabled BOOLEAN NOT NULL DEFAULT 1,
    created_at TEXT NOT NULL,
    last_used_at TEXT
);

-- DSL rules (source of truth for what's synced to settings.json)
CREATE TABLE rules (
    id TEXT PRIMARY KEY,              -- short hash, e.g., "a1b2c3"
    dsl TEXT NOT NULL,                -- original DSL string
    scope TEXT NOT NULL,              -- "global" or "project:/path/to/project"
    enabled BOOLEAN NOT NULL DEFAULT 1,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

-- Hook event log (every hook invocation)
CREATE TABLE events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    session_id TEXT NOT NULL,
    hook_event TEXT NOT NULL,          -- Stop, PreToolUse, etc.
    rule_id TEXT,                      -- which rule handled it (NULL for unmatched)
    tool_name TEXT,                    -- for tool events
    action_taken TEXT,                 -- "notified:discord", "denied", "allowed", etc.
    duration_ms INTEGER,              -- how long the hook took to execute
    timestamp TEXT NOT NULL DEFAULT (datetime('now'))
);

-- Session tracking
CREATE TABLE sessions (
    session_id TEXT PRIMARY KEY,
    project_dir TEXT,
    started_at TEXT,
    ended_at TEXT,
    event_count INTEGER DEFAULT 0,
    files_modified TEXT,               -- JSON array
    summary TEXT
);

-- Key-value state for cross-hook communication
CREATE TABLE kv_state (
    key TEXT NOT NULL,
    value TEXT,
    session_id TEXT,                   -- NULL = global, session_id = session-scoped
    expires_at TEXT,                   -- NULL = no expiry
    PRIMARY KEY (key, COALESCE(session_id, ''))
);

-- Mute state (singleton row)
CREATE TABLE mute (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    muted_until TEXT                   -- NULL = not muted, ISO timestamp = muted until
);

-- Indexes for common queries
CREATE INDEX idx_events_session ON events(session_id);
CREATE INDEX idx_events_timestamp ON events(timestamp);
CREATE INDEX idx_events_hook_event ON events(hook_event);
CREATE INDEX idx_rules_scope ON rules(scope);
CREATE INDEX idx_kv_session ON kv_state(session_id);
CREATE INDEX idx_kv_expires ON kv_state(expires_at);
```

### Migration strategy

Schema version tracked in `schema_version` table. On startup, `ht` checks the version and runs any pending migrations sequentially. Migrations are Go functions compiled into the binary (not external SQL files).

---

## Condition evaluation

Conditions determine whether a rule's actions fire. They're evaluated at hook execution time with access to:

- The hook input JSON (tool name, command, file path, etc.)
- The SQLite state database (elapsed time, session events, mute state)
- Platform state (terminal focus — best-effort)

### Condition types

| Condition | How it works |
|---|---|
| `elapsed > 30s` | Compares current time against the session start time (from `sessions` table or `kv_state`). The `SessionStart` hook (always installed) records the start time. |
| `away` | Best-effort terminal focus detection. macOS: AppleScript. Linux: xdotool. WSL: PowerShell. Falls back to `elapsed > 60s` if detection unavailable. |
| `focused` | Inverse of `away`. |
| `idle > 60s` | Time since last user interaction. Requires the `UserPromptSubmit` hook to record timestamps. |
| `matches "pattern"` | For `pre-bash`/`post-bash`: regex match against `tool_input.command`. For `pre-edit`/`post-edit`: regex match against `tool_input.file_path`. |
| `file matches "pattern"` | Regex match against `tool_input.file_path`. |
| `command matches "pattern"` | Regex match against `tool_input.command`. |
| `matches deny-list:name` | Match `tool_input.command` against every pattern in the named deny list. |
| `tests-pass` | Run the project's test command (auto-detected or configured) and check exit code. Only valid in `require` actions. |
| `not <expr>` | Logical negation. |
| `<expr> and <expr>` | Both must be true. |
| `<expr> or <expr>` | Either must be true. |

### Elapsed time tracking

Hitch always installs a `SessionStart` hook (in addition to user-defined rules) that records the session start time in `kv_state`:

```
key: "session_start:<session_id>"
value: ISO timestamp
session_id: <session_id>
```

The `elapsed` condition reads this value and compares against `time.Now()`.

Similarly, a `UserPromptSubmit` hook records the last interaction time for `idle` conditions.

These "system hooks" are always present and marked with `# ht:system:<name>` to distinguish them from user rules.

---

## CLI command reference

### ht init

```
ht init [--global]
```

- Without `--global`: creates `.hitch/` in current directory, writes system hooks to `.claude/settings.json`
- With `--global`: creates `~/.hitch/` if needed, writes system hooks to `~/.claude/settings.json`
- Creates SQLite database if it doesn't exist
- Idempotent — safe to run multiple times

### ht channel

```
ht channel add <adapter> [config...]    # Add a channel
ht channel list                          # List configured channels
ht channel test [name]                   # Send test notification
ht channel remove <name>                 # Remove a channel
```

Examples:
```bash
ht channel add ntfy my-alerts
ht channel add discord https://discord.com/api/webhooks/123/abc
ht channel add slack https://hooks.slack.com/services/T.../B.../xxx
```

### ht rule

```
ht rule add '<dsl>'                      # Add a rule from DSL string
ht rule add --file <path>                # Add rules from a .hitch file
ht rule list [--scope global|project]    # List rules
ht rule remove <id|pattern>              # Remove a rule
ht rule enable <id>                      # Enable a disabled rule
ht rule disable <id>                     # Disable without removing
```

Adding a rule automatically triggers sync.

### ht hook

```
ht hook exec <rule-id>                   # Execute a specific rule (called by Claude Code)
ht hook exec hook:<name>                 # Execute a custom hook script
ht hook list                             # List all registered custom hooks
```

### ht package

```
ht package list                          # List available packages
ht package enable <name>                 # Enable a hook package
ht package disable <name>                # Disable a hook package
ht package show <name>                   # Show package rules
```

### ht sync

```
ht sync [--scope global|project|all]     # Regenerate settings.json from rules
ht sync --dry-run                        # Show what would change without writing
```

### ht status

```
ht status                                # Overview: channels, rules, recent events, mute state
```

### ht log

```
ht log                                   # Recent hook events
ht log --session <id>                    # Events for a specific session
ht log --event <type>                    # Filter by event type
ht log --since 1h                        # Filter by time
```

### ht mute / unmute

```
ht mute                                  # Mute all notifications indefinitely
ht mute 30m                              # Mute for 30 minutes
ht mute 2h                               # Mute for 2 hours
ht unmute                                # Unmute
```

### ht notify (utility)

```
ht notify <channel> <message>            # Send a notification directly (for use in scripts)
```

### ht export / import

```
ht export [--scope global|project] > rules.hitch
ht import <file|url>
```
