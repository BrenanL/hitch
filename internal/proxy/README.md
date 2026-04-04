# Hitch Proxy — Transparent API Logging for Claude Code

A Go HTTP proxy that sits between Claude Code and the Anthropic API, logging every request to SQLite with zero added latency. Part of the [Hitch](../../README.md) hooks framework.

## Architecture

```
Claude Code  ──HTTP──>  Hitch Proxy (localhost:9800)  ──HTTPS──>  api.anthropic.com
                              │
                              v
                        SQLite (~/.hitch/state.db)
```

The proxy is fully transparent: it forwards requests unchanged, streams SSE responses line-by-line in real time, and extracts metadata (tokens, model, cost, latency) without modifying the payload.

## What Gets Logged

Each API request produces a row in the `api_requests` table:

| Field | Source |
|---|---|
| `model` | SSE `message_start` event or request body |
| `input_tokens`, `output_tokens` | SSE `message_start` + `message_delta` events |
| `cache_read_tokens`, `cache_creation_tokens` | SSE usage data |
| `cost_usd` | Estimated from token counts × model pricing |
| `latency_ms` | Wall clock time from request start to stream end |
| `stop_reason` | `end_turn`, `tool_use`, etc. from `message_delta` |
| `endpoint` | URL path (e.g. `/v1/messages`, `/`) |
| `error` | HTTP status for non-2xx upstream responses |
| `microcompact_count` | Count of `[Old tool result content cleared]` markers in request (Bug 4) |
| `truncated_results` | Count of tool results truncated to 1-41 chars (Bug 5) |

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

This writes `~/.config/systemd/user/hitch-proxy.service` and prints all remaining steps.

### 3. Enable always-on operation

```bash
systemctl --user daemon-reload
systemctl --user enable hitch-proxy
systemctl --user start hitch-proxy
```

The service auto-starts on login and restarts on crash.

### 4. Configure Claude Code

Add the env block to `~/.claude/settings.json` (merge with existing content):

```json
{
  "env": {
    "ANTHROPIC_BASE_URL": "http://localhost:9800",
    "CLAUDE_CODE_PROXY_RESOLVES_HOSTS": "1"
  }
}
```

**Only new Claude Code sessions pick up this change.** Existing sessions continue using their original env.

### 5. (Optional) SessionStart health check

Add to the `hooks` section of `~/.claude/settings.json`:

```json
"SessionStart": [{
  "matcher": "",
  "hooks": [{
    "type": "command",
    "command": "curl -sf http://localhost:9800/health >/dev/null 2>&1 || echo '{\"warning\": \"Hitch proxy not running\"}'"
  }]
}]
```

This warns at session start if the proxy is unreachable.

## CLI Commands

### `ht proxy start [--port 9800]`

Start the proxy in the foreground. Normally run via systemd, not directly.

### `ht proxy stop`

Send SIGTERM to the running proxy process (reads PID from `~/.hitch/proxy.pid`).

### `ht proxy status`

Show whether the proxy is running, its port, uptime, and request count.

```
Proxy: running (PID 1906972)
  Port:     9800
  Uptime:   3m46s
  Requests: 2
```

### `ht proxy tail [-n 20]`

Show the most recent N API requests in chronological order.

```
TIMESTAMP            MODEL                       IN     OUT   CACHE      COST     MS STOP
2026-04-04T22:18:05                               0       0       0 $  0.0000    625  ERR
2026-04-04T22:18:11  claude-opus-4-6              3       6       0 $  0.5153   4417 end_turn
2026-04-04T22:19:44  claude-opus-4-6              3      13   27457 $  0.0422   3431 end_turn
```

Flags in the rightmost column:
- `mc:N` — N microcompact (context stripping) events detected
- `tr:N` — N truncated tool results detected (budget cap)
- `ERR` — upstream returned an error or request hit a non-API endpoint

### `ht proxy stats [--since 1h | --today | --session <id>]`

Aggregate statistics over a time window.

```
Requests:        10
Input tokens:    15
Output tokens:   44
Cache read:      109828
Cache creation:  27457
Cache hit rate:  100.0%
Total cost:      $0.6831
Avg latency:     2108ms

  MODEL                          REQS     TOKENS       COST
  claude-opus-4-6                   5     109887 $   0.6831
```

### `ht proxy install`

Generate the systemd user service file and print step-by-step installation instructions.

## Understanding the Output

### The `/` endpoint entries

Claude Code makes a probe request to the root URL (`/`) before each API call to check connectivity. The Anthropic API returns 404 for this path, so these show up as `ERR` entries with `HTTP 404`. This is normal behavior — these are not failures.

### Cache economics

The first request in a new cache window has high `cache_creation_tokens` (the system prompt being cached). Subsequent requests show `cache_read_tokens` instead, at ~10x lower cost:

| Request | cache_creation | cache_read | Cost |
|---|---|---|---|
| First (cold) | 27,457 | 0 | $0.5153 |
| Subsequent | 0 | 27,457 | $0.0417 |

A healthy cache read ratio in steady state is 95-99%.

### Cost estimation

Pricing is hardcoded for current models (update `internal/proxy/cost.go` as needed):

| Model | Input | Output | Cache Read | Cache Write |
|---|---|---|---|---|
| claude-opus-4-6 | $15/M | $75/M | $1.50/M | $18.75/M |
| claude-sonnet-4-6 | $3/M | $15/M | $0.30/M | $3.75/M |
| claude-haiku-4-5 | $0.80/M | $4/M | $0.08/M | $1/M |

## Bug Detection

### Microcompact (Bug 4: Silent Context Stripping)

Claude Code silently replaces tool results with `[Old tool result content cleared]` mid-session. The proxy counts these markers in every outgoing request and logs the count. When `mc:N` appears in tail output, N tool results have been silently erased from your context.

**Healthy:** 0-5 per session. **Unhealthy:** 50+.

### Budget Truncation (Bug 5: 200K Cap)

Claude Code enforces a ~200K character aggregate limit on tool results. When exceeded, older results get truncated to 1-41 characters. The proxy detects these truncated results and logs the count.

**Healthy:** 0 truncated results. **Unhealthy:** 10+ (start a fresh session).

## Always-On Design

The proxy is designed to never be forgotten:

- **systemd user service** — starts on login, restarts on crash
- **Permanent env var** — `ANTHROPIC_BASE_URL` stays in settings.json at all times
- **Fail loud** — if proxy is down, Claude Code gets connection refused (not silently unlogged)
- **Health endpoint** — `GET /health` returns `{"status":"ok","uptime_seconds":N,"requests_logged":N,"port":9800}`

## Bypassing the Proxy

To run Claude Code without the proxy for a single session:

```bash
ANTHROPIC_BASE_URL=https://api.anthropic.com claude -p "..."
```

**Note:** This override may not work in all Claude Code versions. If the settings.json env block takes precedence over shell env vars, you may need to temporarily edit settings.json to bypass the proxy.

## File Layout

```
internal/proxy/
  server.go       HTTP server, main handler, health endpoint, PID management
  forward.go      Request cloning, upstream forwarding, gzip decompression
  sse.go          SSE streaming passthrough, event parsing, non-streaming handler
  cost.go         Model pricing and cost estimation
  detect.go       Microcompact and budget truncation detection

internal/cli/
  proxy.go        CLI commands: start, stop, status, tail, stats, install

internal/state/
  proxy.go        DB methods: InsertAPIRequest, QueryRecentRequests, GetProxyStats
  migrations.go   Schema v2: api_requests table
```

## Monitoring

| Command | What it shows |
|---|---|
| `ht proxy status` | Is it running? How long? How many requests? |
| `ht proxy tail -n 5` | Last 5 requests with tokens, cost, latency |
| `ht proxy stats --today` | Today's aggregate: total cost, cache hit rate |
| `ht proxy stats --since 1h` | Last hour's stats |
| `curl localhost:9800/health` | Quick health check (for scripts/hooks) |
| `journalctl --user -u hitch-proxy -f` | Systemd service logs (startup + bug detection warnings only) |
