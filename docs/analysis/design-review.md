# Design vs. Implementation Review

Post-MVP review comparing the actual implementation against the original design documents (`philosophy.md`, `architecture.md`, `decisions.md`).

---

## Philosophy Adherence

The philosophy document establishes five principles:

### 1. Invisible infrastructure — Mostly achieved

`ht init` creates directories and installs system hooks in one command. No daemons. The sync mechanism transparently manages settings.json. One weakness: the credential store requires a passphrase prompt that isn't cached across invocations (the store caches in-memory per process, but each `ht hook exec` is a new process). Users need env vars for a truly invisible experience.

### 2. Progressive disclosure — Achieved

The 30-second quickstart (`init` → `channel add` → `rule add`) works. The DSL supports chaining, conditions, and boolean logic for advanced use. Packages provide pre-built bundles for common patterns.

### 3. Agent-friendly by design — Partially achieved

The CLI has clear subcommands an agent can call. But **custom hooks** (`run hook:my-custom-check`) are not implemented — `hook list` is a stub. The architecture describes `~/.hitch/hooks/manifest.json` and a `.hitch/hooks/` directory for agent-written scripts. This entire system is missing. Agents can add rules and channels, but can't drop custom scripts.

### 4. Composable, not monolithic — Achieved

`pkg/hookio` is importable independently. Adapters can send messages without the DSL. The state layer works without notifications. The DSL parser is standalone from the executor.

### 5. Not locked to Claude Code — Mostly achieved

The core concepts are agent-agnostic. The event names in the parser are hardcoded to Claude Code's event taxonomy, but adding events for another agent system would just mean extending the event map.

---

## Architecture Alignment

### Package layout — Exact match

Every planned package exists: `cmd/ht/`, `internal/cli/`, `internal/dsl/`, `internal/engine/`, `internal/generator/`, `internal/adapters/`, `internal/state/`, `internal/credentials/`, `internal/platform/`, `pkg/hookio/`. One addition not in the original architecture: `internal/packages/` for Phase 10 (built-in packages).

### SQLite schema — Match with minor difference

All 6 tables present with specified columns and indexes. One minor difference: `kv_state` uses `PRIMARY KEY (key, session_id)` with `session_id TEXT NOT NULL DEFAULT ''` instead of `PRIMARY KEY (key, COALESCE(session_id, ''))` with nullable `session_id`. Functionally equivalent.

### Adapter interface — Exact match

`Message`, `Level`, `SendResult`, and `Adapter` interface are identical to the architecture document. All four v1 adapters implemented (ntfy, Discord, Slack, desktop).

### HookInput/HookOutput types — Matches and exceeds spec

All fields from the architecture are present, plus additional event-specific fields and tool input types. Output builders cover all Claude Code hook response types.

### Settings.json sync — Match

The merge algorithm follows the documented steps: read manifest markers → remove old hitch entries → add current entries → prune empty groups → write back. Ownership markers follow `# ht:rule-<id>` and `# ht:system:<name>` conventions. Round-trip preservation works.

---

## Decision Document Compliance

| Decision | Status | Notes |
|---|---|---|
| Binary name `ht` | Compliant | |
| Config dirs `~/.hitch/` and `.hitch/` | Compliant | |
| DSL file extension `.hitch` | Compliant | Import/export uses it |
| Go module `github.com/BrenanL/hitch` | Compliant | |
| `pkg/hookio/` only public package | Compliant | Everything else in `internal/` |
| Go, single binary, no CGO | Compliant | `modernc.org/sqlite` is pure Go |
| Cobra CLI framework | Compliant | |
| SQLite WAL mode | Compliant | |
| `filippo.io/age` for credentials | Compliant | |
| Env var fallback `HT_<ADAPTER>_<FIELD>` | Compliant | |
| Exit codes: 0=allow, 2=block | Compliant | |
| Manifest at `~/.hitch/manifest.json` | Compliant | |
| File locking `~/.hitch/sync.lock` | **Not implemented** | Path defined but no locking code |
| MIT License | Compliant | |
| Viper for config | **Not used** | Reasonable simplification — SQLite handles all state |

---

## DSL Grammar Compliance

| Production | Status |
|---|---|
| 15 event types | Exact match |
| Matcher syntax `:value` | Match |
| Action chaining `->` | Match |
| `notify channel [message]` | Match |
| `run "cmd" [async]` | Match |
| `deny ["reason"]` | Match |
| `require check-name` | Match |
| `summarize` | Implemented as stub |
| `log [target]` | Match |
| `elapsed` condition | Match |
| `away`/`focused`/`idle` | Implemented with fallback (no real focus detection) |
| `matches`/`file matches`/`command matches` | Match |
| `deny-list:name` | Match |
| `not`/`and`/`or` | Match |
| Error messages with line numbers | Match, with Levenshtein suggestions |

---

## Significant Gaps

### 1. Custom hooks system — Missing entirely

The architecture describes agents writing scripts to `.hitch/hooks/`, registering them in a manifest, and invoking via `ht hook exec hook:<name>`. No `scripts.go` exists (planned as `internal/generator/scripts.go`), no hooks manifest, and `hook list` is a stub.

### 2. File locking for sync — Not implemented

`decisions.md` specifies file locking via `~/.hitch/sync.lock`. The path is defined in `config.go` but never used. No `flock` call anywhere. Sync only runs on `rule add`/`rule remove`/`init`, not on every hook execution, so risk is low in practice.

### 3. Credential store not wired into channel flow

`channel add` stores config as plain JSON in SQLite. The credential store exists and works, but no CLI command stores secrets through it. Webhook URLs sit in plaintext in the `channels` table.

### 4. `resolveAdapter` in config.go is broken

The function used by `ht hook exec` to create adapters from stored channels never parses the JSON config:

```go
var config map[string]string
if ch.Config != "" && ch.Config != "{}" {
    config = make(map[string]string) // creates empty map, never parses JSON
}
```

Compare to `channel test` which correctly does `json.Unmarshal([]byte(ch.Config), &config)`. Notifications during hook execution would use adapters with no config, causing failures for any adapter that needs configuration.

### 5. `summarize` action is a stub

Returns `"summarized"` and does nothing. The architecture describes reading the transcript and generating a summary. Fine for v1 but worth tracking.

### 6. Terminal focus detection not implemented

`away` falls back to `elapsed > 60s`, `focused` to `elapsed <= 60s`. No OS-specific detection (AppleScript, xdotool, PowerShell) exists. The `platform/` package has OS detection and desktop notifications but no focus detection.

### 7. Viper not used

Listed in `decisions.md` but not in `go.mod`. All config handled through SQLite and `resolvePaths()`. Arguably a good simplification.

---

## Minor Divergences

- `importcmd.go` and `packagecmd.go` instead of `import.go` and `package.go` — Go keyword conflict avoidance. The build plan anticipated this.
- No `platform/sound.go` — Architecture mentioned sound playback. Not implemented.
- Package enable hardcoded to global scope — No flag to override.
- Mute check happens at adapter resolution, not executor level — Functionally correct but architecturally odd.

---

## What Went Well

1. **Core architecture is solid.** The layered design translated cleanly into code. Parser doesn't know about SQLite, engine doesn't know about Cobra.

2. **DSL parser is complete and robust.** All 15 events, 6 actions, 5 condition types, boolean operators, error recovery with suggestions.

3. **Settings.json sync is correct.** Marker-based ownership, round-trip preservation, empty group pruning.

4. **Exit code contract honored.** 0 for allow, 2 for block, consistently.

5. **Fail-open behavior.** Missing rules, disabled rules, corrupted DSL, nil DB — all default to allowing. Correct safety default.

6. **Deny list system works end-to-end.** Embedded patterns, custom files, substring matching, full integration.

7. **`pkg/hookio` is a clean public API.** Builders cover all Claude Code hook response types.

---

## Priority Fixes

1. **Fix `resolveAdapter` config parsing** — Blocks all notification delivery during hook execution. Critical bug.
2. **Implement file locking for sync** — Correctness issue for concurrent environments.
3. **Wire credential store into channel flow** — Webhook URLs shouldn't be plaintext in SQLite.
4. **Custom hooks system** — Key philosophy goal, missing entirely.
5. **Focus detection** — Would make `away`/`focused`/`idle` conditions real instead of time-based approximations.
6. **Summarize action** — Would complete the `on stop -> summarize -> notify` workflow.
