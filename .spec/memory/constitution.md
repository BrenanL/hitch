<!-- Sync Impact Report
Version change: N/A → 1.0.0 (initial)
Added sections: All (initial creation)
-->

# Hitch Constitution

## Core Principles

### I. Architectural Independence

Every subsystem is independently usable AND controllable through the daemon. Each has three access paths:

- **Library:** `internal/<package>/` — core logic, importable by anything
- **CLI:** `ht <command>` — works directly, no daemon required
- **API:** Daemon HTTP endpoints wrap the library — used by dashboards, orchestrators, other tools

The daemon is an amplifier, not a dependency. Pruning, session analysis, cost calculation, and profiles all work without it. When running, the daemon becomes the unified API entry point. Orchestrators and external tools build on the daemon API, not the CLI.

MUST: No library package may depend on the daemon. No CLI command may hard-require the daemon unless it is fundamentally an interactive daemon operation (attach, launch). Commands that read/query data always call the library directly. Commands that mutate session state route through the daemon when available, fall back to direct library calls when not.

### II. Single Source of Truth

Every tunable value, every state record, and every type definition has exactly one canonical location. Duplication creates drift.

- **Config:** `internal/config/` owns all tunable defaults, env var names, config file keys, and the resolution chain (CLI flag > env var > config file > default). No hardcoded paths, ports, or defaults outside this package.
- **State:** SQLite (`hitch.db`) is the single authoritative state store for all operational state. Exceptions: `config.json` (user-editable), PID files (Unix convention), `pricing.json` (cached external data).
- **Types:** Shared types live in shared packages. If two packages need the same struct, it MUST be extracted to a common location. Never duplicate a type definition across packages.

MUST: `grep -r '"\.hitch"' internal/` returns no results outside `internal/config/`. No scattered string literals for paths, ports, or defaults.

### III. Three-Scope Consistency

All Hitch operations that read or write Claude Code settings or hooks support three scopes:

- `project` — `.claude/settings.json` (shared, committed to version control)
- `local` — `.claude/settings.local.json` (per-developer, gitignored, safe default)
- `global` — `~/.claude/settings.json` (user-wide, applies to all projects)

Default scope for hook/rule operations: `local`. System hooks are the exception — they always install globally because that's how Hitch must function.

MUST: Every command that reads or writes settings/hooks accepts `--scope project|local|global`. Every display that shows configuration includes a scope badge (`[P]`, `[L]`, `[U]`).

### IV. No Import Cycles — Dependency Flows Downward

Go forbids circular imports. The package dependency graph MUST be a DAG (directed acyclic graph). The dependency direction is:

```
CLI commands (internal/cli/)
    ↓ imports
Libraries (internal/pruning/, internal/config/, etc.)
    ↓ imports
Public packages (pkg/usage/, pkg/sessions/, pkg/settings/, pkg/hookio/, pkg/envvars/)
    ↓ imports
Shared types (pkg/tokens/types, or similar)
    ↓ imports
Standard library only
```

When two packages need to share a type, the type moves DOWN to a package both can import. When a library needs functionality from a sibling, the caller (CLI layer) composes them — the libraries don't import each other.

MUST: `go build ./...` succeeds. Any new import between packages must be checked against this DAG. If it creates a cycle, restructure by extracting shared types downward or by having the CLI layer compose.

### V. Strategies Are Pure Functions, Safety Is Non-Negotiable

Pruning strategies analyze the message list and return proposed actions — they never mutate state directly. The pipeline applies actions with mandatory safety mechanisms:

- `isProtected()` — messages that must never be touched (system prompts, compact boundaries, behavioral digests, etc.)
- T1.5 guard — never remove a `tool_use` whose `tool_result` is still present
- T1.4 relinking — repair `parentUuid` chains when messages are removed

These safety mechanisms are not optional. They run on every strategy's proposed actions. No shortcut, no bypass, no "just this once." Corrupted sessions are unrecoverable.

MUST: Every strategy checks `isProtected` before proposing removal. The pipeline applies T1.5 and T1.4 to every action set. Tests verify each safety mechanism independently.

### VI. Decisions Govern Specs, Specs Govern Implementation

The authority chain is: Vision → Decisions (D-numbered) → Specs → Task Plan → Implementation.

- **Decisions** are the design authority. When a spec contradicts a decision, the decision wins unless the decision is formally amended with a new D-number.
- **Specs** are the implementation authority. They interpret decisions into exact function signatures, schemas, and acceptance criteria. Specs may add implementation detail not in decisions, but may not contradict them.
- **Task plan** decomposes specs into implementable units. Each task references its spec sections.
- When a spec needs to deviate from a decision, it MUST propose an amendment (new D-number in the addendum) rather than silently diverging.

MUST: Every spec header lists all decision IDs it implements. Every task references its spec sections. Agents check decisions before implementing spec details.

### VII. Universal Output Contract

Every CLI command supports four output modes:

- **Default** — rich terminal output with colors and alignment
- `--plain` / `-p` — non-interactive formatted text (no TUI, but still pretty)
- `--text` — truly plain text, no colors, safe for piping and agent consumption
- `--json` — machine-readable JSON with stable schema

Additionally, `--full` shows complete UUIDs (default is 8-char short form per D-026). All commands register these flags via `applyOutputFlags()` — never inline. `--json` output uses full UUIDs regardless of `--full`.

MUST: New commands use `applyOutputFlags()`. No command defines its own output mode flags inline. All ID displays use 8-char short form by default.

### VIII. Loud Failures, Doctor Recovery

Commands that need initialized state check and fail with clear error messages pointing to `ht init`. Recovery path: `ht doctor` diagnoses, `ht doctor --fix` repairs. Silent failures are bugs.

- `ht sync` with missing prerequisites MUST print what's wrong, not silently succeed
- `ht rule add` without init MUST say "hitch not initialized — run: ht init"
- Every fixable failure in `ht doctor` has an auto-fix path via `--fix`

MUST: No command silently skips work due to missing state. Error messages include the recovery command.

### IX. The Daemon Is Infrastructure

The daemon API is the foundation layer for other tools. Web UIs, orchestrators, messaging bridges, and custom agent management systems call the daemon API rather than reimplementing subprocess management. The daemon is the canonical agent process manager — everything else is a client.

Session brokering: the daemon maintains a registry of active sessions and routes requests to the right session via ID lookup. Any client can locate, observe, and (for managed sessions) interact with any session through the daemon API.

MUST: The daemon HTTP API is stable and well-documented. All session interaction (send prompt, interrupt, prune, profile switch) goes through the same API surface whether called from CLI, dashboard, or external tool.

## Architectural Constraints

### Package Ownership Rules

| Domain | Owner Package | Public? | Consumers |
|--------|--------------|---------|-----------|
| Config resolution + defaults | `internal/config/` | No | All CLI commands, all libraries |
| Cost estimation, pricing, billing blocks, context % | `pkg/usage/` | Yes | Sessions, CLI costs, daemon, external tools (cc-streamdeck-monitor, etc.) |
| Token estimation (chars/4 heuristic) | `internal/tokens/` | No | Usage, pruning, proxy inspection |
| Burn rate + velocity metrics | `internal/metrics/` | No | Daemon, dashboard, hook conditions |
| Pruning strategies + safety | `internal/pruning/` | No | CLI prune, daemon auto-prune, DSL action |
| Session transcript parsing + analysis | `pkg/sessions/` | Yes | CLI sessions/costs/reports, daemon |
| Settings read/write/merge | `pkg/settings/` | Yes | Profiles, generator, sync, TUI |
| Hook I/O protocol | `pkg/hookio/` | Yes | Hook executor (engine) |
| Environment variable registry | `pkg/envvars/` | Yes | TUI, documentation |

**`pkg/` = importable by external tools.** `internal/` = private to Hitch. A package goes in `pkg/` when it provides value to programs that don't use the `ht` binary (e.g., a StreamDeck monitor importing `pkg/usage/` to calculate subscription usage %).

Never duplicate logic that exists in an owned package. If you find yourself writing cost calculation, token estimation, or burn rate math inline — stop and use the shared package.

### SQLite Tables

All operational state lives in `hitch.db`. Known tables:

- `rules` — DSL rules
- `events` — hook execution events
- `sync_entries` — tracks synced hooks (markers, scope, settings path)
- `active_profiles` — profile apply/reset state (project_dir PK, profile_name, scope, tracked_keys, originals, applied_at, optional session_id for daemon linkage)
- `session_records` — daemon session tracking (session_id PK, origin, state, profile, pid, etc.)
- `session_index` — cached session summaries for performance
- `prune_events` — pruning operation logs
- `api_requests` — proxy API request/response logs

New tables require a migration in `internal/state/migrations.go`.

### Protected Paths

Claude Code treats writes to `.claude/settings.json`, `.claude/settings.local.json`, `CLAUDE.md`, and `.claude/memory/` as protected. These always prompt for human approval, even in autonomous mode. Agents MUST NOT write to these directly — use Hitch commands (`ht sync`, `ht profile switch`) that write externally.

## Development Workflow

### Test-First Discipline

No design is complete without a test harness. No task is complete without tests.

- Unit tests co-located with source (`*_test.go` in same package)
- Integration tests in `integration/` (separate package, tests exported APIs)
- Use `state.OpenInMemory()` for DB tests, `t.TempDir()` for file tests
- `httptest.NewServer` for HTTP integration tests
- Simple assertions with `t.Fatalf` (setup) and `t.Errorf` (assertions)
- No external test libraries (no testify)

### Quality Gates

Every wave ends with:
1. `go test ./... -count=1` passing
2. `go vet ./...` clean
3. Documentation updated
4. Human tryout (manual testing of new commands)

### One Commit Per Task

Each task gets its own commit. No batching. Commit message includes the beads task ID.

## Governance

This constitution governs all rev-3 specification and implementation work. It supersedes informal conventions and prior implicit practices.

**Amendment process:** New principles or changes to existing ones require approval followed by propagation to affected specs and this constitution.

**Compliance:** Every spec and task plan must be auditable against these principles.

**Version**: 1.0.0 | **Ratified**: 2026-04-06 | **Last Amended**: 2026-04-06
