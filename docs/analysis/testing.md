# Test Suite Analysis

Honest assessment of the hitch test suite as of the MVP completion. Covers what the tests actually verify, where they fall short, and what needs to be added.

## What Tests Do Well

### Engine (conditions, executor)

`conditions_test.go` constructs real AST nodes, feeds them into `EvalCondition`, and checks actual boolean outcomes against concrete scenarios. It tests boundary behavior (elapsed 45s vs thresholds of 30s and 60s), zero-value handling (no session start), and boolean logic (AND, OR, NOT composition).

`executor_test.go` wires up a real in-memory SQLite DB, stores KV session state, executes rules end-to-end through the parser, and verifies that conditions gate actions. For example, a session start time of 2099 means "elapsed > 30s" is false, so notify shouldn't fire. These are real behavioral tests.

### DSL Parser

The parser tests parse real DSL strings, type-assert the resulting AST nodes, and check field values: channel names, patterns, durations, operator types. They cover shorthand events (`pre-bash` -> `PreToolUse` with `Bash` matcher), chained actions, compound conditions, error cases with typo suggestions, and edge cases like missing arrows and comments.

### Generator / Settings Sync

`TestMergeHooksPreservesNonHitch` proves user hooks survive a merge. `TestMergeHooksRemovesOld` proves old hitch entries get cleaned out. `TestMergeHooksIdempotent` proves re-running sync doesn't duplicate entries. These test the critical contract that hitch never destroys user config.

### Hook I/O

Input tests cover all 12 Claude Code event types and verify that field extraction (`Command()`, `FilePath()`) works on real JSON payloads. Error cases (empty input, invalid JSON, missing event name) are covered.

## Where Tests Are Weak

### State Layer: CRUD-Only

`TestRuleCRUD` and `TestChannelCRUD` insert a row, read it back, delete it. They verify SQLite works but don't test anything interesting about the application. Missing:
- Duplicate ID behavior
- Empty/null field handling
- Schema migration from v1 to v2 (when we add one)
- Concurrent access patterns

These are closer to "existence tests" than behavioral tests.

### Actions: Shallow Coverage

`TestExecuteSummarizeAction` just checks that `ActionTaken == "summarized"` but summarize is a placeholder that does nothing. `TestExecuteLogAction` checks the string without verifying an event was actually written to the DB (the DB is nil in the test context). The notify action test verifies the mock was called but doesn't deeply test message content.

The `run` and `require` actions (which shell out to real commands) have zero test coverage.

### Integration Test: API-Level, Not CLI-Level

`TestEndToEnd` exercises Go APIs directly: calling `db.RuleAdd()`, constructing an `Executor`, passing `HookInput` structs. It never runs the `ht` binary. The real user flow is: Claude Code pipes JSON to `ht hook exec <id>` over stdin and reads JSON from stdout. A bug in stdin reading, stdout writing, argument parsing, or the exit code (2 for block) would be invisible to this test.

### No CLI Tests At All

The `internal/cli/` directory has 0 test files. All the glue code — flag parsing, the `openDB()` helper, `ruleID()` hashing, the `syncScope()` flow, import/export round-trip — is untested.

### No Adapter Config Round-Trip

There's no test that verifies a real adapter can be constructed from a channel's stored config JSON. The `resolveAdapterFromDB` function in `hook.go` is untested. The desktop adapter is also untested.

### No Negative/Adversarial Input Tests

What happens if the DSL in the database is corrupted? If stdin contains 10MB of JSON? If the settings.json is valid JSON but not the expected structure (e.g., `hooks` is a string instead of an object)? The executor silently returns an error for parse failures, but nothing tests that path.

## Coverage Gap Table

| Area | Tests real behavior? | Coverage gaps |
|---|---|---|
| DSL parser | Yes | Missing: deeply nested conditions, very long inputs, unicode |
| Condition evaluator | Yes | Solid |
| Executor pipeline | Yes | Missing: parse-error path, action-error propagation |
| Settings sync | Yes | Missing: malformed settings.json (partially invalid) |
| Hook I/O | Yes (input) | Output tests are basic (check JSON keys exist) |
| State CRUD | Existence-level | No constraint/error/concurrency tests |
| Actions | Mixed | Summarize/log are stubs, run/require not tested |
| Adapters | HTTP-level | Desktop untested, no config->adapter round-trip |
| CLI commands | **Not tested** | 0 test files in internal/cli/ |
| Binary stdin/stdout | **Not tested** | The actual user-facing contract |

## Priority Improvements

1. **CLI binary tests** — pipe JSON to `ht hook exec`, check stdout JSON + exit code. This is the actual user-facing contract.
2. **State edge cases** — duplicate IDs, constraint violations, error paths.
3. **Action error propagation** — adapter failures, command failures, bad DSL in DB.
4. **Adapter config round-trip** — store config JSON in DB, reconstruct adapter from it.
5. **Adversarial inputs** — corrupted DSL, oversized stdin, malformed settings.json.
