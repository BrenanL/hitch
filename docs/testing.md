# Testing Guide

## Running Tests

```bash
# All tests
go test ./...

# Verbose
go test -v ./...

# Specific package
go test -v ./internal/engine/...
go test -v ./internal/dsl/...

# Specific test
go test -v ./internal/engine/ -run TestExecutorDenyRule

# With coverage
go test -cover ./...

# Coverage report
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out -o coverage.html
```

## Test Architecture

### Unit Tests (per-package)

Each internal package has its own `_test.go` files that test the package API in isolation:

| Package | What's tested |
|---|---|
| `internal/dsl/` | Lexer tokenization, parser AST output, validator warnings, error messages with line numbers |
| `internal/engine/` | Condition evaluation, action execution, executor pipeline, deny list matching |
| `internal/state/` | SQLite CRUD operations, schema migration, edge cases |
| `internal/adapters/` | HTTP request formatting (ntfy, Discord, Slack via httptest) |
| `internal/credentials/` | age encrypt/decrypt round-trip, env var fallback |
| `internal/generator/` | Hook entry generation, settings.json merge algorithm, manifest tracking |
| `pkg/hookio/` | JSON input parsing for all 12 event types, output builder formatting |

### Integration Tests (`internal/integration_test.go`)

Tests the assembled pipeline at the Go API level:
- **TestEndToEnd**: init DB -> add channel -> add rules -> parse DSL -> generate hook entries -> sync settings.json -> simulate system hooks -> execute rules -> verify event logging -> test enable/disable -> test mute
- **TestDenyListEndToEnd**: embedded list loading + command classification
- **TestDSLRoundTrip**: multiple rule formats parse to correct AST
- **TestSettingsRoundTrip**: settings.json survives marshal/unmarshal

### CLI Binary Tests (`test/cli_test.go`)

Tests the actual `ht` binary as a black box:
- Builds the binary
- Runs commands with a temp `$HOME` (isolated from real config)
- Pipes JSON to `ht hook exec` on stdin, checks stdout JSON + exit code
- Verifies `ht init`, `ht rule add`, `ht sync` produce correct file output
- Tests the exact contract Claude Code depends on

## Testing Locally with Claude Code

After the automated tests pass, manual testing with Claude Code:

```bash
# Build and init
go build -o ht ./cmd/ht
./ht init --global

# Add a test rule
./ht rule add 'on pre-bash -> deny "test block" if command matches "rm -rf"'
./ht sync

# Start a Claude Code session and ask it to run "rm -rf /tmp/test"
# It should be blocked by the deny rule

# Check the event log
./ht log
```

## Writing New Tests

### For new DSL features

Add cases to `internal/dsl/parser_test.go`:

```go
func TestParseNewFeature(t *testing.T) {
    rules, err := Parse(`on stop -> new-action "arg"`)
    if err != nil {
        t.Fatalf("Parse: %v", err)
    }
    action, ok := rules[0].Actions[0].(NewAction)
    if !ok {
        t.Fatalf("action type = %T", rules[0].Actions[0])
    }
    if action.Arg != "arg" {
        t.Errorf("arg = %q", action.Arg)
    }
}
```

### For new conditions

Add cases to `internal/engine/conditions_test.go`:

```go
func TestEvalNewCondition(t *testing.T) {
    ctx := &EvalContext{
        // set up context for the condition
    }
    cond := dsl.NewCondition{/* fields */}
    if !EvalCondition(cond, ctx) {
        t.Error("expected true")
    }
}
```

### For new CLI commands

Add a subtest to `test/cli_test.go`:

```go
func TestCLINewCommand(t *testing.T) {
    env := setupTestEnv(t)
    out, err := env.run("new-command", "arg1")
    if err != nil {
        t.Fatalf("new-command failed: %v\n%s", err, out)
    }
    if !strings.Contains(out, "expected output") {
        t.Errorf("output = %q", out)
    }
}
```

### For new adapters

Use `httptest.NewServer` to mock the external service:

```go
func TestNewAdapter(t *testing.T) {
    var gotRequest *http.Request
    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        gotRequest = r
        w.WriteHeader(http.StatusOK)
    }))
    defer srv.Close()

    adapter, _ := NewAdapter("new-service", map[string]string{
        "url": srv.URL,
    })

    result := adapter.Send(context.Background(), adapters.Message{
        Title: "Test",
        Body:  "Hello",
    })
    if !result.Success {
        t.Errorf("send failed: %v", result.Error)
    }
    // assert on gotRequest
}
```
