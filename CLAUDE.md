# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Hitch is a hooks framework for AI coding agents. Users declare behaviors (notifications, safety guards, quality gates) in a DSL, and hitch generates Claude Code hook configurations and scripts. The CLI binary is `ht`.

**Current status:** MVP implementation complete (all 12 phases). See `docs/analysis/design-review.md` for post-build review.

## CRITICAL: Do NOT touch global Claude settings

**NEVER use `--global` flags or modify `~/.claude/settings.json`.** Other agents on this machine depend on global Claude settings. All testing and development must use project-scoped mode (the default). Only project-level `.claude/settings.json` files should be written to.

## Technical Decisions (Constraints)

These are finalized in `docs/decisions.md` — treat them as requirements, not suggestions:

- **Language:** Go. Single binary, no runtime dependencies, no CGO.
- **CLI binary name:** `ht`
- **CLI framework:** Cobra + Viper
- **Database:** SQLite via `modernc.org/sqlite` (pure Go). WAL mode. Located at `~/.hitch/state.db`.
- **Encryption:** `filippo.io/age` for credentials at `~/.hitch/credentials.enc`, with env var fallback (`HT_<ADAPTER>_<FIELD>`).
- **Config directories:** `~/.hitch/` (global), `.hitch/` (project)
- **DSL file extension:** `.hitch`
- **Go module path:** `github.com/<org>/hitch`
- **`pkg/hookio/`** is the only public (importable) package. Everything else lives in `internal/`.
- **Platforms:** macOS (arm64/amd64), Linux (amd64/arm64), Windows WSL. Native Windows is not a v1 target.
- **WSL detection:** `uname -r` containing `microsoft`. Route desktop notifications and focus detection through PowerShell interop.

## Build Commands

```bash
go build ./cmd/ht          # Build the binary
go test ./...              # Run all tests
go vet ./...               # Static analysis
golangci-lint run          # Linting (Phase 12)
```

## Architecture

```
CLI (Cobra) → DSL Parser → Hook Generator → Core Engine → Channel Adapters
                                                ↓
                                          SQLite State
```

### Package Layout

| Package | Purpose |
|---|---|
| `cmd/ht/` | CLI entrypoint |
| `internal/cli/` | Cobra command definitions |
| `internal/dsl/` | Lexer, recursive descent parser, AST types, semantic validator |
| `internal/engine/` | Hook execution: condition evaluator, action executor, deny lists |
| `internal/generator/` | settings.json read/merge/write, manifest tracking, system hooks |
| `internal/adapters/` | Notification channel interface + implementations (ntfy, Discord, Slack, desktop) |
| `internal/state/` | SQLite database layer: channels, rules, events, sessions, kv, mute |
| `internal/credentials/` | age encryption + env var fallback |
| `internal/platform/` | OS/WSL detection, platform-specific notifications |
| `pkg/hookio/` | **Public library** — parse hook stdin JSON, build stdout JSON responses |

### Key Data Flow

1. User writes DSL rule → parsed into AST → stored in SQLite
2. `ht sync` reads rules from SQLite → generates settings.json hook entries with ownership markers (`# ht:rule-<id>`)
3. Claude Code fires hook → pipes JSON stdin to `ht hook exec <rule-id>` → hitch evaluates conditions → executes actions → returns JSON stdout → logs event to SQLite
4. Exit codes: 0=allow, 2=block

### settings.json Sync Invariants

- Never delete hooks hitch didn't create (identified by `# ht:` markers)
- Round-tripping preserves all non-managed content
- Malformed settings.json → warn and refuse to modify
- Concurrent access protected by file locking (`~/.hitch/sync.lock`)
- Manifest at `~/.hitch/manifest.json` tracks owned entries

## DSL

Full EBNF grammar is in `docs/architecture.md`. Key points:

- 12+ event types mapping to Claude Code hook events (see event mapping table in architecture.md)
- Shorthand events like `pre-bash` and `post-edit` auto-set both event type and matcher
- Conditions support: `elapsed`, `away`/`focused`/`idle`, `matches`/`file matches`/`command matches`, `deny-list:name`, boolean operators
- Actions chain with `->`: `on stop -> summarize -> notify slack`
- Parser should produce errors with line numbers and suggestions for typos

## Implementation Phases

Phases 1-4 and 6 can be built in parallel. Phase 5 depends on 2. Phase 7 depends on 2+6. Phase 8 integrates everything. Phases 9-12 are sequential. See `docs/build-plan.md` for detailed steps and acceptance criteria per phase.

## Key Documentation

- `docs/architecture.md` — Complete technical spec: DSL grammar, SQLite schema, adapter interface, sync algorithm, CLI reference
- `docs/build-plan.md` — 12-phase implementation plan with dependencies and acceptance criteria
- `docs/decisions.md` — Finalized technical constraints (treat as requirements)
- `docs/philosophy.md` — Design principles and vision
- `docs/hooks-overview.md` — Claude Code hooks API quick reference
- `docs/ideas.md` — 46 hook ideas organized by package
