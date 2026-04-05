# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Hitch is a hooks framework for AI coding agents. Users declare behaviors (notifications, safety guards, quality gates) in a DSL, and hitch generates Claude Code hook configurations and scripts. The CLI binary is `ht`.

**Current status:** MVP implementation complete (all 12 phases). Post-MVP work includes the API logging proxy and upcoming feature specs. See `docs/analysis/design-review.md` for the post-build review and known bugs.

## Development Guide

See [docs/agent-guide.md](docs/agent-guide.md) for all development conventions: test layout, test patterns, code style, build/test commands, and project structure.

## Build Commands

```bash
go build -o ht ./cmd/ht              # Build the binary
go test ./... -count=1                # All tests
go test ./internal/proxy/... -v       # Proxy unit tests
go test ./integration/... -v          # Integration tests
go vet ./...                          # Static analysis (no output = clean)
```

## Global Settings

Global settings modifications (`~/.claude/settings.json`) must go through the sync system (`ht sync`), which preserves non-hitch entries via marker-based ownership (`# ht:` markers). Never directly write to global settings — use the sync mechanism.

During development and testing, prefer project-scoped `.claude/settings.json` to avoid disrupting other running agents. The proxy configuration (`ANTHROPIC_BASE_URL`) is an exception — it's intentionally global.

## Technical Decisions

- **Language:** Go. Single binary, no runtime dependencies, no CGO.
- **CLI binary name:** `ht`
- **CLI framework:** Cobra
- **Database:** SQLite via `modernc.org/sqlite` (pure Go). WAL mode. Located at `~/.hitch/state.db`.
- **Encryption:** `filippo.io/age` for credentials at `~/.hitch/credentials.enc`, with env var fallback (`HT_<ADAPTER>_<FIELD>`).
- **Config directories:** `~/.hitch/` (global), `.hitch/` (project)
- **DSL file extension:** `.hitch`
- **Go module path:** `github.com/BrenanL/hitch`
- **`pkg/hookio/`** is the only public (importable) package. Everything else lives in `internal/`.
- **Platforms:** macOS (arm64/amd64), Linux (amd64/arm64), Windows WSL. Native Windows is not a v1 target.
- **WSL detection:** `uname -r` containing `microsoft`. Route desktop notifications and focus detection through PowerShell interop.

## Architecture

```
CLI (Cobra) -> DSL Parser -> Hook Generator -> Core Engine -> Channel Adapters
                                                   |
                                             SQLite State

Claude Code  --HTTP-->  Hitch Proxy (localhost:9800)  --HTTPS-->  api.anthropic.com
                              |
                        SQLite + Disk Logs
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
| `internal/state/` | SQLite database layer: channels, rules, events, sessions, kv, mute, api_requests |
| `internal/credentials/` | age encryption + env var fallback |
| `internal/platform/` | OS/WSL detection, platform-specific notifications |
| `internal/proxy/` | Transparent API logging proxy: server, forwarding, SSE, cost, detection, analysis |
| `pkg/hookio/` | **Public library** -- parse hook stdin JSON, build stdout JSON responses |
| `integration/` | End-to-end tests (separate package from SUT) |

### Key Data Flow

1. User writes DSL rule -> parsed into AST -> stored in SQLite
2. `ht sync` reads rules from SQLite -> generates settings.json hook entries with ownership markers (`# ht:rule-<id>`)
3. Claude Code fires hook -> pipes JSON stdin to `ht hook exec <rule-id>` -> hitch evaluates conditions -> executes actions -> returns JSON stdout -> logs event to SQLite
4. Exit codes: 0=allow, 2=block

### settings.json Sync Invariants

- Never delete hooks hitch didn't create (identified by `# ht:` markers)
- Round-tripping preserves all non-managed content
- Malformed settings.json -> warn and refuse to modify
- Manifest at `~/.hitch/manifest.json` tracks owned entries

## DSL

- 12+ event types mapping to Claude Code hook events
- Shorthand events like `pre-bash` and `post-edit` auto-set both event type and matcher
- Conditions support: `elapsed`, `away`/`focused`/`idle`, `matches`/`file matches`/`command matches`, `deny-list:name`, boolean operators
- Actions chain with `->`: `on stop -> summarize -> notify slack`
- Parser produces errors with line numbers and suggestions for typos

## Proxy

The proxy is a transparent HTTP server between Claude Code and the Anthropic API. It logs every request/response to SQLite and disk with full headers, bodies, token counts, model, cost, and latency. It detects context stripping (microcompact) and tool result truncation bugs.

Key commands: `ht proxy start`, `ht proxy status`, `ht proxy tail`, `ht proxy sessions`, `ht proxy session <id>`, `ht proxy analyze <id>`, `ht proxy stats`, `ht proxy inspect <id>`.

See `internal/proxy/README.md` for full proxy documentation.

## Known Bugs

1. **`resolveAdapter` config parsing** (`internal/cli/config.go`) -- creates empty map instead of parsing JSON. Notifications during hook execution fail for any adapter needing config. Critical.
2. **File locking not implemented** -- `~/.hitch/sync.lock` path defined but no actual locking code.
3. **`summarize` action is a stub** -- returns "summarized", no transcript reading.
4. **Focus detection not implemented** -- `away`/`focused`/`idle` use time-based fallback only.

## Key Documentation

- `docs/agent-guide.md` -- Development conventions, test patterns, build commands
- `docs/hooks-overview.md` -- Claude Code hooks API quick reference
- `docs/analysis/design-review.md` -- Post-MVP review: what works, what's missing, priority fixes
- `internal/proxy/README.md` -- Proxy architecture, CLI commands, bug detection, monitoring
