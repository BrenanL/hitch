# Hitch Proxy — Transparent API Logging for Claude Code

A Go HTTP proxy that sits between Claude Code and the Anthropic API. Logs every request and response — headers, bodies, tokens, cost, latency — to SQLite and disk. Detects invisible Claude Code bugs (context stripping, tool result truncation).

## Architecture

```
Claude Code  ──HTTP──>  Hitch Proxy (localhost:9800)  ──HTTPS──>  api.anthropic.com
                              │
                              v
                    SQLite (~/.hitch/state.db)       ← metadata, tokens, headers
                    Disk   (~/.hitch/proxy-logs/)    ← full request/response bodies
```

The proxy is fully transparent: requests are forwarded unchanged, SSE responses are streamed line-by-line with zero added delay.

## What Gets Logged

### SQLite (api_requests table)

| Column | Type | Source |
|---|---|---|
| `session_id` | TEXT | `X-Claude-Code-Session-Id` request header |
| `request_id` | TEXT | `message.id` from SSE `message_start` event |
| `model` | TEXT | SSE `message_start` or request body |
| `http_method` | TEXT | Request method (POST, GET, etc.) |
| `http_status` | INT | Upstream response status code |
| `input_tokens` | INT | Non-cached input tokens (SSE usage data) |
| `output_tokens` | INT | Output tokens (SSE `message_delta`) |
| `cache_read_tokens` | INT | Tokens served from Anthropic's prompt cache |
| `cache_creation_tokens` | INT | Tokens written to prompt cache |
| `latency_ms` | INT | Wall clock time, request start to stream end |
| `stop_reason` | TEXT | `end_turn`, `tool_use`, etc. |
| `endpoint` | TEXT | URL path (`/v1/messages`, `/`, etc.) |
| `streaming` | BOOL | Whether SSE streaming was used |
| `error` | TEXT | Error description if request failed |
| `request_headers` | TEXT | JSON of all request headers (API key redacted) |
| `response_headers` | TEXT | JSON of all response headers |
| `request_body_size` | INT | Request body size in bytes |
| `response_body_size` | INT | Response body size in bytes |
| `request_log_path` | TEXT | Path to full request body on disk |
| `response_log_path` | TEXT | Path to full response body on disk |
| `message_count` | INT | Number of messages in the conversation array |
| `microcompact_count` | INT | Context stripping markers detected (Bug 4) |
| `truncated_results` | INT | Truncated tool results detected (Bug 5) |
| `total_tool_result_size` | INT | Total bytes of tool result content |

### Disk (transaction logs)

Full request and response bodies are written to `~/.hitch/proxy-logs/YYYY-MM-DD/`:

- `<id>.req.json` — JSON with method, URL, all headers, full request body
- `<id>.resp.log` — First line is JSON metadata (status + headers), rest is raw response body (SSE stream or JSON)

Cost is **not** stored in the database. It is computed at display time from token counts and `~/.hitch/pricing.json`.

## Installation

### 1. Build

```bash
cd ~/dev/hitch
go build -o ht ./cmd/ht
```

### 2. Generate systemd service

```bash
./ht proxy install
```

Writes `~/.config/systemd/user/hitch-proxy.service` and prints remaining steps.

### 3. Enable always-on operation

```bash
systemctl --user daemon-reload
systemctl --user enable hitch-proxy
systemctl --user start hitch-proxy
```

### 4. Configure Claude Code

Add to `~/.claude/settings.json`:

```json
{
  "env": {
    "ANTHROPIC_BASE_URL": "http://localhost:9800",
    "CLAUDE_CODE_PROXY_RESOLVES_HOSTS": "1"
  }
}
```

Claude Code re-reads settings.json env vars dynamically — even already-running sessions will route through the proxy after this change.

### 5. Seed pricing

```bash
./ht proxy update-pricing
```

Fetches current model pricing from LiteLLM and writes `~/.hitch/pricing.json`. Falls back to built-in defaults if the fetch fails.

## CLI Commands

### `ht proxy start [--port 9800]`

Start the proxy in the foreground. Normally run via systemd.

### `ht proxy stop`

Send SIGTERM to the running proxy (reads PID from `~/.hitch/proxy.pid`).

### `ht proxy status`

```
Proxy: running (PID 1906972)
  Port:     9800
  Uptime:   3m46s
  Requests: 12
```

### `ht proxy tail [-n 20] [-v] [--session <id>]`

Show recent API requests. Default view shows compact table with both cache columns:

```
ID   TIMESTAMP           EP       SESSION  MODEL            IN    OUT  C_READ C_CREATE      COST     MS STOP      FLAGS
1    2026-04-04T22:18:05 /                 -                 0      0       0        0 $  0.0000    625           HTTP 404
2    2026-04-04T22:18:11 /v1/msg  9d3b...  opus-4-6          3      6       0    27457 $  0.2747   4417 end_turn
3    2026-04-04T22:19:44 /v1/msg  899e...  opus-4-6          3     13   27457        0 $  0.0141   3431 end_turn
```

Flags: `-v` verbose (one-line detail per request), `--session <id>` filter by session (prefix match).

### `ht proxy inspect <id>`

Deep dive into a single request — all fields, headers, file paths:

```
ID:            2
Timestamp:     2026-04-04T22:18:11.753
Session:       9d3b2d12-...
Model:         claude-opus-4-6
HTTP:          POST 200
Tokens:
  Input:         3
  Output:        6
  Cache Read:    0
  Cache Create:  27457
  Cost:          $0.2747
Request Headers:
  Anthropic-Version: 2023-06-01
  X-Claude-Code-Session-Id: 9d3b2d12-...
  ...
Body Size:     45231 bytes req / 1234 bytes resp
Request Log:   ~/.hitch/proxy-logs/2026-04-04/221811_000002.req.json
Response Log:  ~/.hitch/proxy-logs/2026-04-04/221811_000002.resp.log
```

### `ht proxy sessions [-n 20]`

List sessions with aggregate stats:

```
SESSION                              REQS  FIRST           LAST            TOKENS       COST
5995cc7d-f1a9-411d-9aca-5d1c36e54164   15  22:26:22        22:36:01        129877 $   3.2100
9d3b2d12-c9fb-4d28-8374-1fc7a9e10c45    1  22:18:11        22:18:11             9 $   0.2747
```

### `ht proxy stats [--since 1h | --today | --session <id>]`

Aggregate statistics. Cost computed at runtime from token counts and current pricing file.

```
Requests:        10
Input tokens:    15
Output tokens:   44
Cache read:      109828
Cache creation:  27457
Cache hit rate:  100.0%
Est. cost:       $0.3307
Avg latency:     2108ms
```

### `ht proxy update-pricing`

Fetch current pricing from LiteLLM's GitHub-hosted model database and write to `~/.hitch/pricing.json`. Falls back to built-in defaults if fetch fails.

### `ht proxy install`

Generate systemd user service and print installation steps.

## Pricing

Pricing is loaded from `~/.hitch/pricing.json` at display time, not stored in the database. Format matches `cc_token_pricing.json`:

```json
{
  "_comment": "Prices in USD per million tokens. cache_write uses 1h tier.",
  "claude-opus-4-6": {
    "input": 5.00,
    "output": 25.00,
    "cache_write": 6.25,
    "cache_read": 0.50
  }
}
```

Update with `ht proxy update-pricing` or edit the file directly. The proxy seeds this file with defaults on first start if it doesn't exist.

## Understanding the Output

### The `/` endpoint entries

Claude Code makes a probe request to the root URL before each API call (startup connectivity check). Returns HTTP 404 from Anthropic's API. Normal behavior, not a failure.

### Cache economics

| Scenario | cache_creation | cache_read | Opus 4.6 cost |
|---|---|---|---|
| Cold start (new cache) | 27,457 | 0 | $0.17 |
| Warm (cache hit) | 0 | 27,457 | $0.01 |
| Growing session | 1,500 | 105,000 | $0.06 |

The ~27K tokens is the system prompt (CLAUDE.md, tools, project context). `input_tokens` only counts new non-cached tokens (e.g., 3 tokens for "say hello"). Healthy steady-state cache read ratio: 95-99%.

### Session tracking

Sessions are tracked via the `X-Claude-Code-Session-Id` header that Claude Code sends on every API request. Use `ht proxy sessions` to list them, `ht proxy tail --session <id>` to filter.

## Bug Detection

### Microcompact (Bug 4: Silent Context Stripping)

Claude Code replaces tool results with `[Old tool result content cleared]` mid-session. The proxy counts these markers in every outgoing request. When `mc:N` appears in tail, N results have been silently erased.

**Healthy:** 0-5 per session. **Unhealthy:** 50+.

### Budget Truncation (Bug 5: 200K Cap)

Claude Code enforces a ~200K character aggregate limit on tool results. When exceeded, older results get truncated to 1-41 characters. The proxy detects these.

**Healthy:** 0 truncated results. **Unhealthy:** 10+ (start a fresh session).

## Always-On Design

- **systemd user service** — starts on login, restarts on crash
- **Permanent env var** — `ANTHROPIC_BASE_URL` stays in settings.json
- **Fail loud** — proxy down = connection refused (not silently unlogged)
- **Health endpoint** — `GET /health` returns JSON status
- **Dynamic pickup** — even running sessions re-read settings.json env

## File Layout

```
internal/proxy/
  server.go       HTTP server, main handler, health endpoint, PID file
  forward.go      Request cloning, upstream forwarding, gzip decompression
  sse.go          SSE streaming passthrough, event parsing, non-streaming handler
  cost.go         Pricing file loader, LiteLLM fetcher, cost estimation
  detect.go       Microcompact and budget truncation detection
  txlog.go        Transaction logger — writes full bodies to disk

internal/cli/
  proxy.go        CLI: start, stop, status, tail, inspect, sessions, stats,
                  install, update-pricing

internal/state/
  proxy.go        DB methods: Insert, Query, GetRequest, ListSessions, Stats
  migrations.go   Schema v3: api_requests table with full columns
```

## Monitoring

| Command | What it shows |
|---|---|
| `ht proxy status` | Running? Uptime? Request count? |
| `ht proxy tail -n 10` | Last 10 requests: tokens, cache, cost, latency |
| `ht proxy tail -v -n 5` | Verbose one-line detail per request |
| `ht proxy inspect <id>` | Full deep dive: headers, paths, all fields |
| `ht proxy sessions` | Sessions with request counts and costs |
| `ht proxy stats --today` | Today's aggregate: cost, cache hit rate |
| `ht proxy stats --session <id>` | Per-session aggregate |
| `curl localhost:9800/health` | Quick health check for scripts |
| `journalctl --user -u hitch-proxy -f` | Service logs (startup + bug warnings) |
