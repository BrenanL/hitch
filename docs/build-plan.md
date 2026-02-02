# Build Plan

Phased implementation plan for hitch. Each phase builds on the previous one. A coding agent should complete phases in order.

## Phase 1: Skeleton

**Goal:** Go project compiles and runs `ht --version`.

**Steps:**
1. Initialize Go module (`go mod init github.com/<org>/hitch`)
2. Set up Cobra CLI skeleton in `cmd/ht/main.go`
3. Add root command, version flag, help text
4. Add stub subcommands: `init`, `channel`, `rule`, `hook`, `sync`, `status`, `log`, `mute`, `package`, `notify`, `export`, `import`
5. Verify `go build ./cmd/ht` produces a working binary
6. Add MIT LICENSE file

**Files created:**
```
cmd/ht/main.go
internal/cli/root.go
internal/cli/init.go        (stub)
internal/cli/channel.go     (stub)
internal/cli/rule.go        (stub)
internal/cli/hook.go        (stub)
internal/cli/sync.go        (stub)
internal/cli/status.go      (stub)
internal/cli/log.go         (stub)
internal/cli/mute.go        (stub)
internal/cli/package.go     (stub)
internal/cli/notify.go      (stub)
internal/cli/export.go      (stub)
internal/cli/import.go      (stub)
go.mod
go.sum
LICENSE
```

**Acceptance:** `ht --help` shows all subcommands. `ht --version` prints version.

---

## Phase 2: State layer (SQLite)

**Goal:** SQLite database creates, migrates, and supports basic CRUD.

**Steps:**
1. Add `modernc.org/sqlite` dependency
2. Implement database initialization (create file, run schema, set WAL mode)
3. Implement schema migration system (version table + Go migration functions)
4. Implement all table operations:
   - channels: add, list, get, remove, update last_used
   - rules: add, list, get, remove, enable, disable
   - events: log, query by session/event/time
   - sessions: create, update, get
   - kv_state: get, set, delete, cleanup expired
   - mute: get, set, clear
5. Write tests for each operation

**Files created:**
```
internal/state/db.go
internal/state/db_test.go
internal/state/channels.go
internal/state/rules.go
internal/state/events.go
internal/state/sessions.go
internal/state/kv.go
internal/state/mute.go
internal/state/migrations.go
```

**Acceptance:** Tests pass. Database creates cleanly. Schema is correct. WAL mode enabled. Concurrent access works.

---

## Phase 3: Hook I/O library

**Goal:** Parse Claude Code hook input, build valid output. This is the public `pkg/hookio` package.

**Steps:**
1. Define types for all 12 hook events' input schemas
2. Implement stdin JSON reader that detects event type and returns typed struct
3. Implement output builders for each decision type:
   - `Allow()`, `Deny(reason)`, `Ask()` for PreToolUse
   - `Block(reason)` for Stop/PostToolUse/UserPromptSubmit
   - `InjectContext(text)` for SessionStart/UserPromptSubmit
   - `Continue(false, stopReason)` for any event
4. Implement exit code handling
5. Write tests with sample JSON from Claude Code docs

**Files created:**
```
pkg/hookio/types.go
pkg/hookio/input.go
pkg/hookio/input_test.go
pkg/hookio/output.go
pkg/hookio/output_test.go
```

**Acceptance:** Can round-trip every hook event type's JSON. Output matches Claude Code's expected format exactly.

---

## Phase 4: Channel adapters

**Goal:** Send notifications through ntfy, Discord, Slack, and desktop.

**Steps:**
1. Define the `Adapter` interface and `Message` type
2. Implement ntfy adapter (HTTP POST to ntfy.sh)
3. Implement Discord webhook adapter (HTTP POST with embed format)
4. Implement Slack webhook adapter (HTTP POST with Block Kit format)
5. Implement desktop adapter (OS detection → notify-send / osascript / powershell)
6. Implement adapter registry (lookup by name)
7. Write tests (use httptest for HTTP adapters)

**Files created:**
```
internal/adapters/adapter.go
internal/adapters/registry.go
internal/adapters/ntfy.go
internal/adapters/ntfy_test.go
internal/adapters/discord.go
internal/adapters/discord_test.go
internal/adapters/slack.go
internal/adapters/slack_test.go
internal/adapters/desktop.go
internal/platform/detect.go
internal/platform/notify.go
```

**Acceptance:** `ht notify ntfy "test"` sends a real notification. `ht channel test` works for each adapter. Desktop notifications work on macOS, Linux, and WSL.

---

## Phase 5: Credential storage

**Goal:** Securely store and retrieve channel credentials.

**Steps:**
1. Add `filippo.io/age` dependency
2. Implement credential store: encrypt/decrypt credentials file
3. Implement master passphrase flow (prompt on first use, cache in memory)
4. Implement environment variable fallback (`HT_<ADAPTER>_<FIELD>`)
5. Wire credential store into channel adapter configuration
6. Wire into `ht channel add` / `ht channel remove`

**Files created:**
```
internal/credentials/store.go
internal/credentials/store_test.go
internal/credentials/env.go
```

**Acceptance:** `ht channel add discord <url>` encrypts the URL. `ht channel test discord` decrypts and uses it. `HT_DISCORD_WEBHOOK_URL` overrides the stored value.

---

## Phase 6: DSL parser

**Goal:** Parse `.hitch` DSL files into an AST.

**Steps:**
1. Define AST types (Rule, Event, Matcher, Action, Condition, etc.)
2. Implement lexer/tokenizer
3. Implement recursive descent parser
4. Implement semantic validator:
   - Unknown events → error with suggestion
   - Unknown channels → warning (may be configured later)
   - Invalid conditions → error with explanation
   - Missing time units → error
5. Implement error reporting with line numbers
6. Write comprehensive parser tests covering every grammar production

**Files created:**
```
internal/dsl/tokens.go
internal/dsl/lexer.go
internal/dsl/lexer_test.go
internal/dsl/ast.go
internal/dsl/parser.go
internal/dsl/parser_test.go
internal/dsl/validator.go
internal/dsl/validator_test.go
internal/dsl/errors.go
```

**Acceptance:** All example rules from the DSL spec parse correctly. Error messages are clear and include line numbers. Invalid input produces helpful errors, not panics.

---

## Phase 7: Hook generator and settings.json sync

**Goal:** Convert parsed rules into Claude Code hook entries and manage settings.json.

**Steps:**
1. Implement rule-to-hook-command generation (each rule → `ht hook exec <rule-id>`)
2. Implement rule-to-settings-JSON generation (build the hooks object structure)
3. Implement settings.json reader (parse, handle missing file, handle malformed JSON)
4. Implement manifest reader/writer
5. Implement sync algorithm (read → identify owned → remove old → add new → write)
6. Implement file locking
7. Implement ownership marker parsing (`# ht:rule-<id>`)
8. Implement system hooks installation (SessionStart for elapsed time, UserPromptSubmit for idle time)
9. Wire into `ht sync` and auto-sync on `ht rule add/remove`

**Files created:**
```
internal/generator/hooks.go
internal/generator/hooks_test.go
internal/generator/settings.go
internal/generator/settings_test.go
internal/generator/manifest.go
internal/generator/manifest_test.go
internal/generator/system_hooks.go
```

**Acceptance:** `ht rule add 'on stop -> notify ntfy'` creates a rule in SQLite, generates a settings.json entry, and the entry is correctly structured. `ht sync --dry-run` shows the diff. Running sync twice is idempotent. Non-hitch hooks in settings.json are preserved.

---

## Phase 8: Condition evaluator and hook executor

**Goal:** When Claude Code calls `ht hook exec <rule-id>`, evaluate conditions and execute actions.

**Steps:**
1. Implement condition evaluator:
   - `elapsed`: read session start from kv_state, compare to now
   - `matches`/`command matches`/`file matches`: regex against input fields
   - `deny-list:name`: load deny list file, match each pattern
   - `away`/`focused`: platform-specific focus detection (best-effort)
   - `idle`: read last interaction time from kv_state
   - Boolean operators: `and`, `or`, `not`
2. Implement action executor:
   - `notify`: look up channel, build message from context, send
   - `deny`: build deny response JSON
   - `run`: execute shell command, capture output
   - `require`: run check, block stop if failed
   - `summarize`: read transcript, generate summary (placeholder for v1)
   - `log`: write to event log
3. Implement action chaining (`->` sequences)
4. Implement the `ht hook exec` command:
   - Read stdin
   - Look up rule
   - Evaluate conditions
   - Execute actions
   - Write output
   - Log event
5. Implement custom hook dispatch (`ht hook exec hook:<name>`)

**Files created:**
```
internal/engine/executor.go
internal/engine/executor_test.go
internal/engine/conditions.go
internal/engine/conditions_test.go
internal/engine/actions.go
internal/engine/actions_test.go
```

**Acceptance:** Full end-to-end: add a rule via DSL, sync to settings.json, simulate a hook call with piped JSON stdin, verify correct output. Conditions filter correctly. Notifications actually send. Deny rules produce correct JSON output.

---

## Phase 9: CLI commands (full implementation)

**Goal:** All CLI commands work end-to-end.

**Steps:**
1. Implement `ht init` (create directories, install system hooks, create DB)
2. Implement `ht channel add/list/test/remove` (wired to state + credentials + adapters)
3. Implement `ht rule add/list/remove/enable/disable` (wired to state + DSL parser + sync)
4. Implement `ht status` (query state DB, format output)
5. Implement `ht log` (query events table with filters)
6. Implement `ht mute/unmute` (update mute table, check mute in executor)
7. Implement `ht sync` (with `--dry-run`)
8. Implement `ht export/import`
9. Implement `ht notify` (direct send utility)

**Files updated:** All files in `internal/cli/`

**Acceptance:** Every command documented in the CLI reference section of architecture.md works. Help text is accurate. Error messages are helpful.

---

## Phase 10: Built-in packages

**Goal:** Pre-built hook packages that users can enable with one command.

**Steps:**
1. Define package format (named collection of DSL rules)
2. Implement `notifier` package:
   - `on stop -> notify <default-channel> if elapsed > 30s`
   - `on notification:permission -> notify <default-channel> "Claude needs permission"`
   - `on notification:idle -> notify <default-channel> "Claude is waiting for input"`
3. Implement `safety` package:
   - `on pre-bash -> deny if matches deny-list:destructive`
   - `on pre-edit -> deny if file matches ".env|.git/*|*.pem|*.key"`
   - `on pre-bash -> deny if command matches "git push --force.*(main|master)"`
4. Implement `quality` package:
   - `on post-edit -> run "<test-command>" async` (auto-detect test runner)
   - `on stop -> require tests-pass`
5. Implement `observer` package:
   - `on stop -> log`
   - `on session-end -> log`
   - `on post-bash -> log`
6. Implement `ht package enable/disable/list/show`

**Files created:**
```
internal/packages/packages.go
internal/packages/notifier.go
internal/packages/safety.go
internal/packages/quality.go
internal/packages/observer.go
```

**Acceptance:** `ht package enable notifier` adds the notifier rules. `ht package disable notifier` removes them. Rules are synced to settings.json. Packages use the user's configured default channel.

---

## Phase 11: Deny lists

**Goal:** Ship curated deny lists and support custom ones.

**Steps:**
1. Embed the `destructive` deny list in the binary (Go embed)
2. Implement deny list loading (built-in + `~/.hitch/deny-lists/*.txt`)
3. Implement deny list matching in condition evaluator
4. Add `ht deny-list list`, `ht deny-list show <name>`, `ht deny-list add <name>`

**Files created:**
```
internal/engine/denylists.go
internal/engine/denylists_test.go
internal/engine/embedded/destructive.txt
```

**Acceptance:** `on pre-bash -> deny if matches deny-list:destructive` correctly blocks `rm -rf /` and allows `npm test`. Custom deny lists in `~/.hitch/deny-lists/` are loaded and usable.

---

## Phase 12: Polish and testing

**Goal:** Integration tests, error handling, edge cases, documentation.

**Steps:**
1. End-to-end integration tests (init → add channel → add rule → sync → simulate hook → verify)
2. Error handling audit (every external call wrapped, helpful messages)
3. Concurrent access tests (multiple hook processes hitting SQLite simultaneously)
4. WSL-specific testing (focus detection fallback, desktop notification via PowerShell)
5. Update README with final installation instructions
6. Verify all CLI help text matches actual behavior

**Acceptance:** All tests pass. `go vet` and `golangci-lint` clean. The tool works end-to-end on macOS, Linux, and WSL.

---

## Dependency summary

| Phase | Depends on |
|---|---|
| 1. Skeleton | — |
| 2. State (SQLite) | — |
| 3. Hook I/O | — |
| 4. Adapters | — |
| 5. Credentials | 2 (state for channel configs) |
| 6. DSL Parser | — |
| 7. Generator/Sync | 2 (state), 6 (parser) |
| 8. Executor | 2 (state), 3 (hookio), 4 (adapters), 5 (creds), 6 (parser) |
| 9. CLI commands | All above |
| 10. Packages | 6 (parser), 7 (generator), 9 (CLI) |
| 11. Deny lists | 8 (executor) |
| 12. Polish | All above |

Phases 1-4 and 6 can be built in parallel. Phase 5 depends on 2. Phase 7 depends on 2 and 6. Phase 8 is the integration point that pulls everything together. Phases 9-12 are sequential.
