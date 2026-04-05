# Proxy Test Harness Requirements

This document outlines what's needed to test the proxy package without hitting the real Anthropic API or requiring a running Claude Code instance.

## Current state

The proxy package has zero test files. All verification so far has been manual integration testing (start proxy, run `claude -p`, check tail output).

## Test architecture

The proxy is an HTTP server that forwards to an upstream API. To test it, we need a **fake upstream** — a local HTTP server that speaks the Anthropic API protocol (SSE streaming, non-streaming JSON, error responses).

```
Test  ──HTTP──>  Proxy (localhost:random)  ──HTTP──>  Fake Upstream (localhost:random)
                       │                                     │
                       v                                     │
                 SQLite (in-memory)                     Returns canned
                 Disk (temp dir)                        SSE/JSON responses
```

Both the proxy and fake upstream bind to `localhost:0` (OS-assigned ports) so tests can run in parallel without port conflicts.

## What the fake upstream needs to produce

### Streaming response (SSE)

A valid Anthropic Messages API SSE sequence:

```
event: message_start
data: {"type":"message_start","message":{"id":"msg_test123","type":"message","role":"assistant","model":"claude-opus-4-6","usage":{"input_tokens":10,"cache_read_input_tokens":500,"cache_creation_input_tokens":100},"content":[]}}

event: content_block_start
data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}

event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hello!"}}

event: content_block_stop
data: {"type":"content_block_stop","index":0}

event: message_delta
data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":5}}

event: message_stop
data: {"type":"message_stop"}
```

### Non-streaming response (JSON)

```json
{
  "id": "msg_test456",
  "type": "message",
  "role": "assistant",
  "model": "claude-opus-4-6",
  "content": [{"type": "text", "text": "Hello!"}],
  "stop_reason": "end_turn",
  "usage": {
    "input_tokens": 10,
    "output_tokens": 5,
    "cache_read_input_tokens": 500,
    "cache_creation_input_tokens": 100
  }
}
```

### Error responses

- `429 Too Many Requests` with `{"error":{"type":"rate_limit_error","message":"..."}}`
- `500 Internal Server Error`
- `401 Unauthorized`
- Connection refused (upstream down)
- Slow response (test timeout handling)
- Truncated SSE stream (upstream disconnects mid-stream)

## Test categories

### 1. Unit tests (no HTTP, no DB)

These test pure functions in isolation.

| File | Functions to test | What to verify |
|---|---|---|
| `cost.go` | `EstimateCost`, `LoadPricingFromFile`, `FetchLiteLLMPricing` | Correct math, file parsing, prefix matching, zero for unknown models |
| `detect.go` | `detectMicrocompact`, `detectBudgetTruncation` | Marker counting, truncation detection across content structures |
| `sse.go` | `parseSSEData`, `parseNonStreamingResponse` | Token extraction from message_start, message_delta, non-streaming JSON |
| `txlog.go` | `SanitizeHeaders` | API key redaction, other headers preserved |

These are straightforward table-driven tests. No dependencies.

### 2. Integration tests (proxy + fake upstream + in-memory DB)

These test the full request lifecycle.

**Setup per test:**
```go
func setupTestProxy(t *testing.T) (proxyURL string, fakeUpstream *httptest.Server, db *state.DB) {
    // 1. Open in-memory SQLite
    db, _ = state.OpenInMemory()
    
    // 2. Start fake upstream
    fakeUpstream = httptest.NewServer(http.HandlerFunc(fakeHandler))
    
    // 3. Create proxy server pointing at fake upstream
    srv := proxy.NewServerWithUpstream(port, db, fakeUpstream.URL)
    // Need to expose upstream URL as configurable (currently hardcoded)
    
    // 4. Start proxy on random port
    proxyServer = httptest.NewServer(srv)
    
    t.Cleanup(func() {
        proxyServer.Close()
        fakeUpstream.Close()
        db.Close()
    })
    
    return proxyServer.URL, fakeUpstream, db
}
```

**Required code change:** The `Server` struct currently hardcodes `upstream: "https://api.anthropic.com"`. Tests need to point it at the fake. Options:
- Add `NewServerWithUpstream(port int, db *state.DB, upstream string)` constructor
- Or make `upstream` a parameter on `NewServer`

**Test cases:**

| Test | Request | Fake upstream response | Verify |
|---|---|---|---|
| Basic streaming passthrough | POST /v1/messages, stream:true | SSE sequence with tokens | Response matches upstream byte-for-byte, DB row has correct tokens/model/stop_reason |
| Non-streaming passthrough | POST /v1/messages, stream:false | JSON response | Response body matches, DB row correct |
| Large SSE events | POST /v1/messages | SSE with 5MB data line | Scanner doesn't truncate, response intact |
| Error forwarding | POST /v1/messages | 429 error response | Client gets 429, DB row has error field |
| Upstream down | POST /v1/messages | (server stopped) | Client gets 502, DB row has connection error |
| Health endpoint | GET /health | (not forwarded) | Returns 200 with JSON status |
| Root probe | GET / | 404 | Client gets 404, DB row logged with HTTP 404 error |
| Session ID capture | Request with X-Claude-Code-Session-Id header | SSE response | DB row has session_id populated |
| Header capture | Request with anthropic-version, user-agent | Any response | DB request_headers JSON contains them, API key redacted |
| Response headers | Any request | Response with x-ratelimit headers | DB response_headers JSON contains them |
| Gzip response | Any request | Gzip-encoded SSE | Client gets decompressed, tokens parsed correctly |
| Microcompact detection | Request body with `[Old tool result content cleared]` | Any response | DB microcompact_count > 0 |
| Budget truncation detection | Request with short tool_result content blocks | Any response | DB truncated_results > 0 |
| Transaction log files | Any request | Any response | .req.json and .resp.log files exist at logged paths, content is valid |
| Cost not in DB | Any request | Any response | No cost_usd column in DB (confirm schema) |
| Message count | Request with 5 messages in array | Any response | DB message_count = 5 |
| Mid-stream disconnect | POST /v1/messages, stream:true | SSE that closes after 3 events | Partial data logged, error field set |

### 3. Pricing tests

| Test | What to verify |
|---|---|
| Load from valid JSON file | Parses correctly, all models present |
| Load from missing file | Falls back to defaults |
| Load with `_comment` field | Comment ignored, models parsed |
| Prefix matching | `claude-opus-4-6-20260205` matches `claude-opus-4-6` pricing |
| Unknown model | Returns 0 cost |
| Correct math | For known inputs, cost matches hand-calculated expected value |
| LiteLLM format conversion | Per-token costs correctly multiplied to per-million |

### 4. Transaction log tests

| Test | What to verify |
|---|---|
| Request log format | Valid JSON, contains method, url, headers, body |
| Response log format | First line is valid JSON metadata, rest is raw body |
| API key not in log files | Authorization and x-api-key headers redacted |
| Directory creation | Logs create date-based subdirectories |
| Path stored in DB | request_log_path and response_log_path match actual files |

## Code changes needed for testability

1. **Configurable upstream URL**: `NewServer` needs an upstream parameter so tests can point at `httptest.Server`. Currently hardcoded to `https://api.anthropic.com`.

2. **Configurable log directory**: `TransactionLogger` needs a configurable base dir so tests write to a temp directory, not `~/.hitch/proxy-logs/`.

3. **Expose `ServeHTTP`**: Already implements `http.Handler` so `httptest.NewServer(srv)` works. No change needed.

4. **In-memory DB**: Already exists via `state.OpenInMemory()`. No change needed.

## Fake upstream implementation sketch

```go
// fakeAnthropic returns a configurable fake Anthropic API handler.
func fakeAnthropic(opts ...fakeOption) http.Handler {
    cfg := &fakeConfig{
        model:       "claude-opus-4-6",
        inputTokens: 10,
        outputTokens: 5,
        cacheRead:   500,
        cacheCreate: 100,
        stopReason:  "end_turn",
        responseText: "Hello!",
    }
    for _, o := range opts {
        o(cfg)
    }
    
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if r.URL.Path == "/" {
            http.Error(w, "not found", 404)
            return
        }
        
        // Parse request to check stream flag
        body, _ := io.ReadAll(r.Body)
        var req struct { Stream bool `json:"stream"` }
        json.Unmarshal(body, &req)
        
        if req.Stream {
            writeSSEResponse(w, cfg)
        } else {
            writeJSONResponse(w, cfg)
        }
    })
}
```

## Test helper: assertDBRow

```go
func assertDBRow(t *testing.T, db *state.DB, id int64, checks map[string]any) {
    t.Helper()
    r, err := db.GetRequest(id)
    if err != nil {
        t.Fatalf("GetRequest(%d): %v", id, err)
    }
    // Check each expected field...
}
```

## Running tests

```bash
go test ./internal/proxy/...          # proxy package only
go test ./internal/proxy/... -v       # verbose
go test ./internal/proxy/... -run TestStreaming  # single test
go test ./... -count=1                # all tests, no cache
```

## Priority order

1. Unit tests for `cost.go` and `detect.go` — pure functions, easy wins
2. Unit tests for `parseSSEData` and `parseNonStreamingResponse` — critical parsing logic
3. Add `NewServerWithUpstream` constructor — unlocks integration tests
4. Basic streaming integration test — the core happy path
5. Error handling integration tests
6. Transaction log tests
7. Header and session capture tests
