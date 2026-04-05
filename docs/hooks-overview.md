# Claude Code Hooks — Quick Reference

Claude Code hooks are user-defined shell commands, LLM prompts, or agents that execute automatically at specific points in Claude Code's lifecycle. They give you deterministic control over Claude's behavior — ensuring certain actions always happen rather than relying on the LLM to choose.

**Official docs:**
- [Hooks Guide](https://code.claude.com/docs/en/hooks-guide) — practical walkthrough, examples, troubleshooting
- [Hooks Reference](https://code.claude.com/docs/en/hooks) — full event schemas, JSON I/O, advanced features

## Hook Events

There are 26 hook events covering the full session lifecycle:

### Core Session Events

| Event | When It Fires | Can Block? | Primary Use Cases |
|---|---|---|---|
| `SessionStart` | Session begins or resumes | No | Load context, set env vars, inject reminders |
| `SessionEnd` | Session terminates | No | Cleanup, logging, session summaries |
| `UserPromptSubmit` | User submits a prompt, before Claude sees it | Yes | Validate prompts, add context, filter input |
| `Stop` | Claude finishes responding | Yes (block = continue) | Quality gates, notifications, task chaining |
| `StopFailure` | Claude stops due to an error | No | Error-specific alerting and recovery |
| `InstructionsLoaded` | CLAUDE.md or instructions file is loaded | No | Audit instructions, inject overrides |

### Tool Events

| Event | When It Fires | Can Block? | Primary Use Cases |
|---|---|---|---|
| `PreToolUse` | Before a tool call executes | Yes (allow/deny/ask) | Block commands, modify input, auto-approve |
| `PostToolUse` | After a tool call succeeds | No (feedback only) | Format code, lint, log, validate output |
| `PostToolUseFailure` | After a tool call fails | No (feedback only) | Alert on errors, provide corrective context |
| `PermissionRequest` | Permission dialog is about to show | Yes (allow/deny) | Auto-approve safe tools, deny dangerous ones |
| `PermissionDenied` | A permission request was denied | No | Log denials, notify, adjust behavior |

### Context Events

| Event | When It Fires | Can Block? | Primary Use Cases |
|---|---|---|---|
| `PreCompact` | Before context compaction | No | Save critical context before it's summarized |
| `PostCompact` | After context compaction completes | No | Restore context, log compaction details |

### Subagent Events

| Event | When It Fires | Can Block? | Primary Use Cases |
|---|---|---|---|
| `SubagentStart` | A subagent is spawned | No | Inject context, log agent activity |
| `SubagentStop` | A subagent finishes | Yes | Chain tasks, validate subagent output |

### Notification and UI Events

| Event | When It Fires | Can Block? | Primary Use Cases |
|---|---|---|---|
| `Notification` | Claude needs user attention | No | Route alerts to phone, desktop, Slack, etc. |
| `Elicitation` | Claude requests input from the user | Yes | Pre-fill responses, filter elicitation requests |
| `ElicitationResult` | User responds to an elicitation | Yes | Validate user input before Claude sees it |

### Task and Workflow Events

| Event | When It Fires | Can Block? | Primary Use Cases |
|---|---|---|---|
| `TaskCreated` | A new task is added to the task list | Yes | Validate tasks, enforce naming conventions |
| `TaskCompleted` | A task is marked done | Yes | Quality gates before marking complete |
| `TeammateIdle` | A teammate agent has been idle | Yes | Reassign work, send alerts |

### Configuration and Filesystem Events

| Event | When It Fires | Can Block? | Primary Use Cases |
|---|---|---|---|
| `ConfigChange` | Claude Code configuration changes | Yes | Audit config changes, enforce policies |
| `CwdChanged` | Working directory changes | No | Load project-specific context |
| `FileChanged` | A file in the project changes | No | Trigger linting, tests, or sync |
| `WorktreeCreate` | A git worktree is created | Yes | Set up worktree environment, enforce policies |
| `WorktreeRemove` | A git worktree is removed | No | Cleanup, logging |

### Blocking behavior

Events marked "Can Block" let your hook prevent the action from proceeding:
- **Exit code 2** from a command hook = block
- **JSON output** with `decision: "block"` or `permissionDecision: "deny"` = block with structured control
- **Prompt/Agent hooks** returning `{ "ok": false, "reason": "..." }` = block

Events that can't block still let you inject context, log activity, or trigger side effects.

## Hook Types

| Type | Description | When to Use |
|---|---|---|
| `command` | Runs a shell command | Deterministic rules, scripts, API calls |
| `prompt` | Single-turn LLM evaluation | Judgment calls that need reasoning |
| `agent` | Multi-turn subagent with tool access | Verification that requires reading files/running commands |

Command hooks are the most common. Prompt and agent hooks are for decisions that need LLM reasoning (e.g., "are all tasks actually complete?").

## Configuration

Hooks are defined in JSON settings files. Three levels of nesting:

1. **Event** — which lifecycle point (`PreToolUse`, `Stop`, etc.)
2. **Matcher** — regex filter for when it fires (`Bash`, `Edit|Write`, `*`)
3. **Handler** — the command/prompt/agent to run

### Example: Block destructive Bash commands

```json
{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "Bash",
        "hooks": [
          {
            "type": "command",
            "command": "/path/to/validate-command.sh"
          }
        ]
      }
    ]
  }
}
```

### Where hooks live

| Location | Scope | Shareable? |
|---|---|---|
| `~/.claude/settings.json` | All your projects | No (local to machine) |
| `.claude/settings.json` | Single project | Yes (commit to repo) |
| `.claude/settings.local.json` | Single project | No (gitignored) |
| Plugin `hooks/hooks.json` | When plugin is enabled | Yes (bundled with plugin) |
| Skill/Agent frontmatter | While component is active | Yes (in component file) |
| Managed policy settings | Organization-wide | Yes (admin-controlled) |

## Hook I/O

### Input (JSON on stdin)

Every hook receives common fields plus event-specific data:

```json
{
  "session_id": "abc123",
  "transcript_path": "/path/to/transcript.jsonl",
  "cwd": "/home/user/my-project",
  "permission_mode": "default",
  "hook_event_name": "PreToolUse",
  "tool_name": "Bash",
  "tool_input": { "command": "npm test" }
}
```

### Output

- **Exit 0**: Action proceeds. Stdout parsed for JSON.
- **Exit 2**: Action blocked. Stderr fed back to Claude.
- **Other exit**: Non-blocking error. Stderr logged.

JSON output fields: `continue`, `stopReason`, `decision`, `reason`, `additionalContext`, `hookSpecificOutput`, and more depending on the event.

## Matchers

| Event | What Matcher Filters | Examples |
|---|---|---|
| `PreToolUse`, `PostToolUse`, etc. | Tool name | `Bash`, `Edit\|Write`, `mcp__.*` |
| `SessionStart` | How session started | `startup`, `resume`, `compact` |
| `SessionEnd` | Why session ended | `clear`, `logout`, `other` |
| `Notification` | Notification type | `permission_prompt`, `idle_prompt` |
| `SubagentStart/Stop` | Agent type | `Bash`, `Explore`, `Plan` |
| `PreCompact` | Trigger type | `manual`, `auto` |
| `UserPromptSubmit`, `Stop` | No matcher support | Always fires |

## Async Hooks

Add `"async": true` to a command hook to run it in the background without blocking Claude. Useful for long-running tasks like test suites or deployments. Output is delivered on the next conversation turn.

## The `/hooks` Menu

Type `/hooks` in Claude Code to interactively view, add, and delete hooks without editing JSON files directly. Changes through this menu take effect immediately.

## Key Gotchas

- Hooks run with your full user permissions — review scripts before deploying
- Hook timeout defaults to 10 minutes (configurable per hook)
- `PostToolUse` cannot undo actions (the tool already ran)
- `Stop` fires whenever Claude finishes responding, not only at task completion
- `Stop` does NOT fire on user interrupts
- Stop hooks need to check `stop_hook_active` to avoid infinite loops
- Shell profile `echo` statements can break JSON parsing — wrap them in `[[ $- == *i* ]]` checks
- Hooks from file edits don't take effect until reviewed in `/hooks` or session restart
