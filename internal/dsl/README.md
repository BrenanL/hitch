# DSL Package

The `dsl` package implements the hitch rule language: lexer, recursive descent parser, AST types, and semantic validator.

## DSL Rule Syntax

```
on <event>[:<matcher>] -> <action> [-> <action> ...] [if <condition>]
```

## Event Types

There are 26 unique Claude Code hook events, exposed via 30 DSL names (including shorthand aliases).

### Session Events

| DSL Name | Claude Code Event | Matcher Support | Notes |
|---|---|---|---|
| `session-start` | `SessionStart` | Yes | Fires on start, resume, or compact |
| `session-end` | `SessionEnd` | Yes | Fires on clear, logout, or other |
| `user-prompt` | `UserPromptSubmit` | No | Always fires; can block |
| `stop` | `Stop` | No | Fires when Claude finishes responding; can block |
| `stop-failure` | `StopFailure` | No | Fires on error stop |
| `instructions-loaded` | `InstructionsLoaded` | Yes | CLAUDE.md or instructions loaded |

### Tool Events

| DSL Name | Claude Code Event | Default Matcher | Notes |
|---|---|---|---|
| `pre-tool` | `PreToolUse` | `*` | Any tool; can block |
| `post-tool` | `PostToolUse` | `*` | Any tool |
| `tool-failure` | `PostToolUseFailure` | `*` | Tool call failed |
| `pre-bash` | `PreToolUse` | `Bash` | Shorthand for pre-tool:Bash |
| `post-bash` | `PostToolUse` | `Bash` | Shorthand for post-tool:Bash |
| `pre-edit` | `PreToolUse` | `Edit\|Write` | Shorthand for pre-tool:Edit\|Write |
| `post-edit` | `PostToolUse` | `Edit\|Write` | Shorthand for post-tool:Edit\|Write |
| `permission` | `PermissionRequest` | Yes | Can block (allow/deny) |
| `permission-denied` | `PermissionDenied` | Yes | Non-blocking |

### Context Events

| DSL Name | Claude Code Event | Matcher Support | Notes |
|---|---|---|---|
| `pre-compact` | `PreCompact` | Yes | Before context compaction |
| `post-compact` | `PostCompact` | Yes | After context compaction |

### Subagent Events

| DSL Name | Claude Code Event | Matcher Support | Notes |
|---|---|---|---|
| `subagent-start` | `SubagentStart` | Yes | Subagent spawned |
| `subagent-stop` | `SubagentStop` | Yes | Subagent finished; can block |

### Notification and UI Events

| DSL Name | Claude Code Event | Matcher Support | Notes |
|---|---|---|---|
| `notification` | `Notification` | Yes | User attention needed |
| `elicitation` | `Elicitation` | Yes | Claude requests input; can block |
| `elicitation-result` | `ElicitationResult` | Yes | User responded; can block |

### Task and Workflow Events

| DSL Name | Claude Code Event | Matcher Support | Notes |
|---|---|---|---|
| `task-created` | `TaskCreated` | No | New task added; can block |
| `task-completed` | `TaskCompleted` | No | Task marked done; can block |
| `teammate-idle` | `TeammateIdle` | No | Teammate agent idle; can block |

### Configuration and Filesystem Events

| DSL Name | Claude Code Event | Matcher Support | Notes |
|---|---|---|---|
| `config-change` | `ConfigChange` | Yes | Config changed; can block |
| `cwd-changed` | `CwdChanged` | No | Working directory changed |
| `file-changed` | `FileChanged` | Yes | File changed in project |
| `worktree-create` | `WorktreeCreate` | No | Git worktree created; can block |
| `worktree-remove` | `WorktreeRemove` | No | Git worktree removed |

## Conditions

12 condition types are available in `if` clauses.

### Time Conditions

| Condition | Syntax | Description |
|---|---|---|
| `elapsed` | `elapsed > 30m` | Time since session start. Operators: `>`, `<`, `>=`, `<=`, `=`. Units: `s`, `m`, `h`. |

### Focus Conditions

| Condition | Syntax | Description |
|---|---|---|
| `away` | `away` | Terminal is not focused |
| `focused` | `focused` | Terminal is focused |
| `idle` | `idle` or `idle > 5m` | Terminal is idle; optional duration comparison |

### Pattern Matching

| Condition | Syntax | Description |
|---|---|---|
| `matches` | `matches "pattern"` | Regex match against tool input or event data |
| `file matches` | `file matches "*.go"` | Regex match against file path |
| `command matches` | `command matches "rm.*"` | Regex match against bash command; tool events only |

Deny list variant: `matches deny-list:name` — checks against a named deny list stored in hitch state.

### Resource Conditions (New in Wave 1B)

| Condition | Syntax | Description |
|---|---|---|
| `burn_rate` | `burn_rate > 0.8` | Token burn rate. Values 0–1 are fractions of limit; values >1 are absolute tokens/min. |
| `context_size` | `context_size > 50000` | Absolute context token count. Integer comparison. |
| `context_usage` | `context_usage > 80` | Percentage of context window used (0–100). Float comparison. |

### Model Condition (New in Wave 1B)

| Condition | Syntax | Description |
|---|---|---|
| `model contains` | `model contains "opus"` | Case-insensitive substring match against the active model identifier |

### Field Equality (New in Wave 1B)

| Condition | Syntax | Description |
|---|---|---|
| `error_type` | `error_type == "timeout"` | Match error type on `stop-failure` events |
| `task_status` | `task_status == "blocked"` | Match task status on `task-created` or `task-completed` events |

### Boolean Operators

Conditions compose with `and`, `or`, `not`:

```
if burn_rate > 0.8 and not focused
if elapsed > 1h or context_usage > 90
if not (model contains "haiku")
```

## Actions

9 actions are available, chained with `->`.

### Notification and Logging

| Action | Syntax | Description |
|---|---|---|
| `notify` | `notify slack "message"` | Send notification to named channel. Message is optional. |
| `log` | `log [target]` | Log the event. Optional target name. |

### Control Flow

| Action | Syntax | Description |
|---|---|---|
| `deny` | `deny "reason"` | Block the current action. Reason is optional. Only meaningful on blocking events. |
| `require` | `require tests-pass` | Run a named check; block if it fails. |

### Execution

| Action | Syntax | Description |
|---|---|---|
| `run` | `run "command"` or `run "command" async` | Execute a shell command. Add `async` to run in background without blocking. |
| `summarize` | `summarize` | Generate a session summary (stub — returns placeholder, no transcript reading yet). |

### Context Management (New in Wave 1B)

| Action | Syntax | Description |
|---|---|---|
| `inject_context` | `inject_context "text"` | Add text to the hook response's `additionalContext` field. |
| `prune` | `prune gentle` | Signal context pruning. Tiers: `gentle`, `moderate`, `aggressive`, `emergency`. Only meaningful on `pre-compact` or `post-compact`. |

### Profile Switching (New in Wave 1B)

| Action | Syntax | Description |
|---|---|---|
| `switch_profile` | `switch_profile production` | Switch the active hitch profile by name. |

## Examples

```
# Notify on stop after a long session
on stop -> notify slack if elapsed > 30m

# Block destructive commands
on pre-bash -> deny "use safer alternative" if command matches "rm -rf"

# Alert when context is nearly full
on stop -> notify desktop if context_usage > 85

# Switch to cost-saving profile when burn rate is high
on stop -> switch_profile economy if burn_rate > 0.9

# Inject reminder when using a fast model
on session-start -> inject_context "You are in fast mode. Keep responses concise." if model contains "haiku"

# Prune context aggressively before compaction when context is large
on pre-compact -> prune aggressive if context_size > 100000

# Alert on errors
on stop-failure -> notify slack if error_type == "timeout"

# Log task completions
on task-completed -> log if task_status == "done"
```
