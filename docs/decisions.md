# Finalized Decisions

This document records the technical decisions for hitch. These are final — not deliberations, not options. A coding agent should treat these as constraints.

## Name: hitch

- **Project:** hitch
- **CLI binary:** `ht`
- **Config directories:** `~/.hitch/` (global), `.hitch/` (project)
- **DSL file extension:** `.hitch`
- **Go module:** `github.com/<org>/hitch` (org TBD)

The name evokes "hitching onto events" — connecting, attaching, riding along. Zero CLI abbreviation conflicts.

## Language: Go

Single Go binary. No runtime dependencies.

**Why:** Fast startup (hooks block Claude's execution, latency matters), single binary distribution (no "install Python 3.11" problems), cross-platform (macOS, Linux, WSL all run the same binary), pure-Go SQLite (no CGO/native deps), mature CLI ecosystem (Cobra, Viper, bubbletea).

**Go libraries to evaluate:**
- CLI framework: Cobra
- Config: Viper
- SQLite: `modernc.org/sqlite` (pure Go, no CGO)
- Encryption: `filippo.io/age` (pure Go)
- JSON: standard library `encoding/json`
- DSL parser: hand-written recursive descent or `participle`

## License: MIT

Maximum adoption, minimal friction. Same as beads.

## Hook execution model: Hybrid

Two types of hooks:

**Built-in hooks** execute via the `ht` binary itself. Claude Code's hook command calls `ht hook exec <rule-id>`, which reads stdin JSON, evaluates conditions, performs actions (notify, deny, etc.), and writes the response to stdout. One binary, no scripts to manage.

**Custom hooks** are user/agent-written scripts in `.hitch/hooks/` (project) or `~/.hitch/hooks/` (global). They're registered in a manifest and invoked by name: `ht hook exec hook:my-custom-check`. An agent can create custom hooks by writing a script file and registering it — no hitch internals knowledge needed.

The distinction: built-in hooks use hitch's compiled logic (fast, maintained). Custom hooks are arbitrary scripts (flexible, user-owned). Both are invoked the same way from Claude Code's perspective.

## State: SQLite

Single SQLite file at `~/.hitch/state.db`. WAL mode for concurrent access from parallel hook processes.

**What it stores:**
- Configured channels and their metadata
- DSL rules and their enabled/disabled state
- Hook event log (every hook invocation, timestamped)
- Session tracking (start/end, files modified, summaries)
- Key-value state for cross-hook communication (e.g., elapsed time tracking)
- Mute state

**Why SQLite over flat files:** Queryable (`ht log`, `ht status`), handles concurrent writes from parallel hooks, structured schema, foundation for the eventual dashboard/orchestrator.

## Credentials: age encryption + environment variable fallback

**Primary:** Credentials stored in `~/.hitch/credentials.enc`, encrypted with [age](https://github.com/FiloSottile/age). Master passphrase prompted on first use, cached for the session. Pure Go implementation, no external dependency.

**Fallback:** Any credential overridable via environment variable (`HT_DISCORD_WEBHOOK`, `HT_TWILIO_TOKEN`, etc.). Useful for CI, Docker, and environments where file-based secrets are awkward.

**Why not system keychain:** Per-OS APIs (macOS Keychain, libsecret, Windows Credential Manager), WSL makes keychain access unreliable, three implementations to maintain. Could be added later as an optional backend.

## Config format: Two-layer model

**Layer 1 (user-facing):** `.hitch` DSL files. Users write and share these. One file per scope:
- `~/.hitch/rules.hitch` — global rules
- `.hitch/rules.hitch` — project rules

**Layer 2 (generated):** Claude Code's `settings.json`. Hitch reads existing settings, merges its hook entries, writes back. Users never edit this for managed hooks.

The sync pipeline: DSL file -> parser -> hook generator -> settings.json + manifest. `ht sync` is idempotent.

## settings.json ownership tracking

Each hook hitch writes into settings.json includes a marker in the command string: `# ht:<rule-id>`. A manifest at `~/.hitch/manifest.json` (or `.hitch/manifest.json` for project scope) maps rule IDs to settings.json entries. During sync, hitch removes its old entries (identified by manifest), adds current entries, and preserves everything else.

**Invariants:**
- Never delete hooks we didn't create
- Round-tripping preserves all non-managed content
- Malformed settings.json = warn and refuse to modify
- Concurrent access protected by file locking

## First notification channels (v1)

| Channel | Priority | Auth model |
|---|---|---|
| ntfy.sh | 1 | Topic name only (no account needed) |
| Discord webhook | 2 | Webhook URL |
| Slack webhook | 3 | Webhook URL |
| Desktop notification | 4 | OS-native |

These four cover the most common needs with minimal setup friction. More channels in v2.

## DSL ships in v1

The DSL is the product differentiator. Without it, hitch is just another hooks helper. The parser ships in v1. The DSL spec is in [architecture.md](architecture.md).

## The deny-list gap

Claude Code has NO built-in deny-list for `--dangerously-skip-permissions`. It's all-or-nothing. `PreToolUse` hooks run before the permission system, so hitch can implement an effective deny-list as a hook — blocking destructive patterns while allowing everything else.

This is one of hitch's strongest selling points: "Run with `--dangerously-skip-permissions` safely."

## Plugin strategy

**Standalone CLI is primary.** Not locked to Claude Code, full feature set, full CLI experience.

**Claude Code plugin is secondary.** A thin wrapper providing pre-configured hook packages and skills for managing hooks from within Claude. Built on the same binary. Deferred to after v1 stabilizes.

## WSL strategy

Design for Linux-native. Detect WSL via `uname -r` containing `microsoft`. Route platform-specific operations (sound, native Windows notifications, focus detection) through PowerShell interop. Everything else (network requests, SQLite, file operations) works natively in WSL.

## Target platforms

- macOS (arm64, amd64)
- Linux (amd64, arm64)
- Windows WSL (runs Linux binary)

Native Windows (non-WSL) is not a v1 target.
