# Agent & Development Guide

Conventions for agents and developers working in this codebase.

## Language and Dependencies

- Go 1.24+, pure Go only (no CGO)
- CLI framework: Cobra (`github.com/spf13/cobra`)
- Database: SQLite via `modernc.org/sqlite` (pure Go driver)
- No external test libraries (no testify) — use standard `testing` package

## Test Layout

Following Go best practices:

- **Unit tests** (white-box): `*_test.go` files co-located with source in the same package. These have access to unexported functions.
- **Integration tests** (end-to-end): `integration/` directory at the project root. Separate package, tests only exported APIs.
- **Test fixtures**: `testdata/` directories within packages (Go ignores these during build).

```
internal/proxy/
  cost.go
  cost_test.go          # package proxy — unit tests
  detect.go
  detect_test.go        # package proxy — unit tests
  testdata/             # fixture files

integration/
  proxy_integration_test.go   # package integration — e2e tests
```

## Test Patterns

- Simple assertions with `t.Fatalf` (setup failures) and `t.Errorf` (assertion failures)
- No table-driven tests required — straightforward sequential tests are fine
- Use `state.OpenInMemory()` for database tests (in-memory SQLite)
- Use `t.TempDir()` for tests that write files
- Use `httptest.NewServer` for HTTP integration tests

## Build and Test Commands

```bash
go build -o ht ./cmd/ht          # Build the binary
go test ./... -count=1            # All tests (no cache)
go test ./internal/proxy/... -v   # Proxy unit tests (verbose)
go test ./integration/... -v      # Integration tests (verbose)
go vet ./...                      # Static analysis (no output = clean)
```

## Proxy Development

The proxy has testability constructors for pointing at fake upstreams and temp directories:

```go
proxy.NewServerWithUpstream(port, db, upstreamURL, logDir)  // for tests
proxy.NewServer(port, db)                                     // production defaults
```

The proxy runs as a systemd user service. After building:

```bash
systemctl --user restart hitch-proxy    # Pick up new binary
./ht proxy status                        # Verify running
```

## Code Style

- No unnecessary abstractions — three similar lines beats a premature helper
- No docstrings/comments on code you didn't change
- No error handling for scenarios that can't happen
- Prefer editing existing files over creating new ones
- Keep imports organized: stdlib, then external, then internal

## Project Structure

- `cmd/ht/` — CLI entrypoint only
- `internal/` — All private packages (cli, dsl, engine, generator, adapters, state, credentials, platform, packages, proxy)
- `pkg/hookio/` — Only public importable package
- `integration/` — End-to-end tests
- `docs/` — Public documentation
- `private/` — Developer-only docs (gitignored sub-repo)

## Global Settings

Global settings modifications must go through the sync system (`ht sync`), which preserves non-hitch entries via marker-based ownership (`# ht:rule-<id>`). During development and testing, prefer project-scoped `.claude/settings.json` to avoid disrupting other running agents. Direct writes to `~/.claude/settings.json` are only acceptable when explicitly managing the proxy or other system-level configuration.
