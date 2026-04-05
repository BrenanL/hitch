package integration

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/BrenanL/hitch/internal/proxy"
	"github.com/BrenanL/hitch/internal/state"
)

// --- fake upstream ---

type fakeConfig struct {
	model        string
	stopReason   string
	responseText string
	inputTokens  int
	outputTokens int
	cacheRead    int
	cacheCreate  int
	statusCode   int
	errorType    string
	errorMessage string
	slow         time.Duration
}

type fakeOption func(*fakeConfig)

func withModel(model string) fakeOption {
	return func(c *fakeConfig) { c.model = model }
}

func withTokens(input, output, cacheRead, cacheCreate int) fakeOption {
	return func(c *fakeConfig) {
		c.inputTokens = input
		c.outputTokens = output
		c.cacheRead = cacheRead
		c.cacheCreate = cacheCreate
	}
}

func withError(status int, errType, msg string) fakeOption {
	return func(c *fakeConfig) {
		c.statusCode = status
		c.errorType = errType
		c.errorMessage = msg
	}
}

func fakeAnthropic(opts ...fakeOption) http.Handler {
	cfg := &fakeConfig{
		model:        "claude-opus-4-6",
		stopReason:   "end_turn",
		responseText: "Hello!",
		inputTokens:  10,
		outputTokens: 5,
		cacheRead:    500,
		cacheCreate:  100,
	}
	for _, o := range opts {
		o(cfg)
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if cfg.slow > 0 {
			time.Sleep(cfg.slow)
		}

		if r.URL.Path == "/" {
			http.Error(w, "not found", 404)
			return
		}

		// Error mode
		if cfg.statusCode >= 400 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(cfg.statusCode)
			json.NewEncoder(w).Encode(map[string]any{
				"error": map[string]any{
					"type":    cfg.errorType,
					"message": cfg.errorMessage,
				},
			})
			return
		}

		// Parse request to check stream flag
		body, _ := io.ReadAll(r.Body)
		var req struct {
			Stream bool `json:"stream"`
		}
		json.Unmarshal(body, &req)

		if req.Stream {
			writeSSEResponse(w, cfg)
		} else {
			writeJSONResponse(w, cfg)
		}
	})
}

func writeSSEResponse(w http.ResponseWriter, cfg *fakeConfig) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(200)

	flusher, _ := w.(http.Flusher)

	// message_start
	fmt.Fprintf(w, "event: message_start\ndata: %s\n\n",
		mustJSON(map[string]any{
			"type": "message_start",
			"message": map[string]any{
				"id":    "msg_test_stream",
				"model": cfg.model,
				"usage": map[string]any{
					"input_tokens":                  cfg.inputTokens,
					"cache_read_input_tokens":       cfg.cacheRead,
					"cache_creation_input_tokens":   cfg.cacheCreate,
				},
			},
		}))
	if flusher != nil {
		flusher.Flush()
	}

	// content_block_start
	fmt.Fprintf(w, "event: content_block_start\ndata: %s\n\n",
		mustJSON(map[string]any{
			"type":          "content_block_start",
			"index":         0,
			"content_block": map[string]any{"type": "text", "text": ""},
		}))
	if flusher != nil {
		flusher.Flush()
	}

	// content_block_delta
	fmt.Fprintf(w, "event: content_block_delta\ndata: %s\n\n",
		mustJSON(map[string]any{
			"type":  "content_block_delta",
			"index": 0,
			"delta": map[string]any{"type": "text_delta", "text": cfg.responseText},
		}))
	if flusher != nil {
		flusher.Flush()
	}

	// content_block_stop
	fmt.Fprintf(w, "event: content_block_stop\ndata: %s\n\n",
		mustJSON(map[string]any{"type": "content_block_stop", "index": 0}))
	if flusher != nil {
		flusher.Flush()
	}

	// message_delta
	fmt.Fprintf(w, "event: message_delta\ndata: %s\n\n",
		mustJSON(map[string]any{
			"type":  "message_delta",
			"delta": map[string]any{"stop_reason": cfg.stopReason},
			"usage": map[string]any{"output_tokens": cfg.outputTokens},
		}))
	if flusher != nil {
		flusher.Flush()
	}

	// message_stop
	fmt.Fprintf(w, "event: message_stop\ndata: %s\n\n",
		mustJSON(map[string]any{"type": "message_stop"}))
	if flusher != nil {
		flusher.Flush()
	}
}

func writeJSONResponse(w http.ResponseWriter, cfg *fakeConfig) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	json.NewEncoder(w).Encode(map[string]any{
		"id":          "msg_test_json",
		"type":        "message",
		"role":        "assistant",
		"model":       cfg.model,
		"stop_reason": cfg.stopReason,
		"content":     []map[string]any{{"type": "text", "text": cfg.responseText}},
		"usage": map[string]any{
			"input_tokens":                cfg.inputTokens,
			"output_tokens":               cfg.outputTokens,
			"cache_read_input_tokens":     cfg.cacheRead,
			"cache_creation_input_tokens": cfg.cacheCreate,
		},
	})
}

func mustJSON(v any) string {
	data, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return string(data)
}

// --- test setup ---

func setupTestProxy(t *testing.T, opts ...fakeOption) (proxyURL string, db *state.DB, logDir string) {
	t.Helper()

	db, err := state.OpenInMemory()
	if err != nil {
		t.Fatalf("opening in-memory DB: %v", err)
	}

	upstream := httptest.NewServer(fakeAnthropic(opts...))

	logDir = t.TempDir()
	srv := proxy.NewServerWithUpstream(0, db, upstream.URL, logDir)

	proxyServer := httptest.NewServer(srv)

	t.Cleanup(func() {
		proxyServer.Close()
		upstream.Close()
		db.Close()
	})

	return proxyServer.URL, db, logDir
}

func sendRequest(t *testing.T, proxyURL string, stream bool, headers map[string]string, body map[string]any) *http.Response {
	t.Helper()
	if body == nil {
		body = map[string]any{}
	}
	body["stream"] = stream
	if _, ok := body["model"]; !ok {
		body["model"] = "claude-opus-4-6"
	}

	data, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshaling request body: %v", err)
	}

	req, err := http.NewRequest("POST", proxyURL+"/v1/messages", strings.NewReader(string(data)))
	if err != nil {
		t.Fatalf("creating request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Api-Key", "sk-test-key-12345")
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("sending request: %v", err)
	}
	return resp
}

func getDBRow(t *testing.T, db *state.DB) state.APIRequest {
	t.Helper()
	// saveLog is called synchronously at the end of ServeHTTP, so the row
	// should be available immediately after the HTTP response is fully read.
	rows, err := db.QueryRecentRequests(1, "")
	if err != nil {
		t.Fatalf("querying DB: %v", err)
	}
	if len(rows) == 0 {
		t.Fatal("no DB rows found after request")
	}
	return rows[0]
}

// --- integration tests ---

func TestStreamingPassthrough(t *testing.T) {
	proxyURL, db, _ := setupTestProxy(t,
		withModel("claude-opus-4-6"),
		withTokens(10, 42, 500, 100),
	)

	resp := sendRequest(t, proxyURL, true, nil, nil)
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	// Read full SSE stream
	var events []string
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "event: ") {
			events = append(events, strings.TrimPrefix(line, "event: "))
		}
	}

	// Verify SSE events were forwarded
	if len(events) < 4 {
		t.Errorf("expected at least 4 SSE events, got %d: %v", len(events), events)
	}

	row := getDBRow(t, db)
	if row.Model != "claude-opus-4-6" {
		t.Errorf("DB Model = %q, want %q", row.Model, "claude-opus-4-6")
	}
	if row.InputTokens != 10 {
		t.Errorf("DB InputTokens = %d, want 10", row.InputTokens)
	}
	if row.OutputTokens != 42 {
		t.Errorf("DB OutputTokens = %d, want 42", row.OutputTokens)
	}
	if row.CacheReadTokens != 500 {
		t.Errorf("DB CacheReadTokens = %d, want 500", row.CacheReadTokens)
	}
	if row.CacheCreationTokens != 100 {
		t.Errorf("DB CacheCreationTokens = %d, want 100", row.CacheCreationTokens)
	}
	if row.StopReason != "end_turn" {
		t.Errorf("DB StopReason = %q, want %q", row.StopReason, "end_turn")
	}
	if !row.Streaming {
		t.Error("DB Streaming should be true")
	}
	if row.HTTPStatus != 200 {
		t.Errorf("DB HTTPStatus = %d, want 200", row.HTTPStatus)
	}
}

func TestNonStreamingPassthrough(t *testing.T) {
	proxyURL, db, _ := setupTestProxy(t,
		withTokens(15, 8, 1000, 200),
	)

	resp := sendRequest(t, proxyURL, false, nil, nil)
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	var result map[string]any
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("response not valid JSON: %v", err)
	}

	row := getDBRow(t, db)
	if row.RequestID != "msg_test_json" {
		t.Errorf("DB RequestID = %q, want %q", row.RequestID, "msg_test_json")
	}
	if row.InputTokens != 15 {
		t.Errorf("DB InputTokens = %d, want 15", row.InputTokens)
	}
	if row.OutputTokens != 8 {
		t.Errorf("DB OutputTokens = %d, want 8", row.OutputTokens)
	}
}

func TestErrorForwarding(t *testing.T) {
	proxyURL, db, _ := setupTestProxy(t,
		withError(429, "rate_limit_error", "Too many requests"),
	)

	resp := sendRequest(t, proxyURL, true, nil, nil)
	defer resp.Body.Close()

	if resp.StatusCode != 429 {
		t.Errorf("status = %d, want 429", resp.StatusCode)
	}

	row := getDBRow(t, db)
	if row.HTTPStatus != 429 {
		t.Errorf("DB HTTPStatus = %d, want 429", row.HTTPStatus)
	}
	if row.Error == "" {
		t.Error("DB Error should be set for 429 response")
	}
}

func TestUpstreamDown(t *testing.T) {
	db, err := state.OpenInMemory()
	if err != nil {
		t.Fatalf("opening DB: %v", err)
	}
	defer db.Close()

	// Create a server pointing at a closed upstream
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	upstreamURL := upstream.URL
	upstream.Close() // close it immediately

	srv := proxy.NewServerWithUpstream(0, db, upstreamURL, t.TempDir())
	proxyServer := httptest.NewServer(srv)
	defer proxyServer.Close()

	resp := sendRequest(t, proxyServer.URL, true, nil, nil)
	defer resp.Body.Close()

	if resp.StatusCode != 502 {
		t.Errorf("status = %d, want 502", resp.StatusCode)
	}

	row := getDBRow(t, db)
	if row.Error == "" {
		t.Error("DB Error should be set for upstream down")
	}
}

func TestHealthEndpoint(t *testing.T) {
	proxyURL, _, _ := setupTestProxy(t)

	resp, err := http.Get(proxyURL + "/health")
	if err != nil {
		t.Fatalf("GET /health: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("health status = %d, want 200", resp.StatusCode)
	}

	var health map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&health); err != nil {
		t.Fatalf("decoding health response: %v", err)
	}
	if health["status"] != "ok" {
		t.Errorf("health status = %v, want ok", health["status"])
	}
}

func TestSessionIDCapture(t *testing.T) {
	proxyURL, db, _ := setupTestProxy(t)

	resp := sendRequest(t, proxyURL, false, map[string]string{
		"X-Claude-Code-Session-Id": "test-sess-abc",
	}, nil)
	defer resp.Body.Close()
	io.ReadAll(resp.Body)

	row := getDBRow(t, db)
	if row.SessionID != "test-sess-abc" {
		t.Errorf("DB SessionID = %q, want %q", row.SessionID, "test-sess-abc")
	}
}

func TestHeaderCapture(t *testing.T) {
	proxyURL, db, _ := setupTestProxy(t)

	resp := sendRequest(t, proxyURL, false, map[string]string{
		"Anthropic-Version": "2024-01-01",
	}, nil)
	defer resp.Body.Close()
	io.ReadAll(resp.Body)

	row := getDBRow(t, db)

	// Check request headers JSON
	var reqHeaders map[string][]string
	if err := json.Unmarshal([]byte(row.RequestHeaders), &reqHeaders); err != nil {
		t.Fatalf("parsing request headers: %v", err)
	}

	if vals, ok := reqHeaders["Anthropic-Version"]; !ok || vals[0] != "2024-01-01" {
		t.Errorf("Anthropic-Version not captured: %v", reqHeaders)
	}

	if vals, ok := reqHeaders["X-Api-Key"]; ok && vals[0] != "[REDACTED]" {
		t.Errorf("X-Api-Key should be redacted, got %q", vals[0])
	}
}

func TestMicrocompactDetection(t *testing.T) {
	proxyURL, db, _ := setupTestProxy(t)

	body := map[string]any{
		"messages": []map[string]any{
			{
				"role": "user",
				"content": []map[string]any{
					{"type": "tool_result", "content": "[Old tool result content cleared]"},
					{"type": "tool_result", "content": "[Old tool result content cleared]"},
				},
			},
		},
	}

	resp := sendRequest(t, proxyURL, false, nil, body)
	defer resp.Body.Close()
	io.ReadAll(resp.Body)

	row := getDBRow(t, db)
	if row.MicrocompactCount != 2 {
		t.Errorf("DB MicrocompactCount = %d, want 2", row.MicrocompactCount)
	}
}

func TestBudgetTruncationDetection(t *testing.T) {
	proxyURL, db, _ := setupTestProxy(t)

	body := map[string]any{
		"messages": []map[string]any{
			{
				"role": "user",
				"content": []map[string]any{
					{"type": "tool_result", "content": "x"},     // 1 char — truncated
					{"type": "tool_result", "content": "short"}, // 5 chars — truncated
				},
			},
		},
	}

	resp := sendRequest(t, proxyURL, false, nil, body)
	defer resp.Body.Close()
	io.ReadAll(resp.Body)

	row := getDBRow(t, db)
	if row.TruncatedResults < 2 {
		t.Errorf("DB TruncatedResults = %d, want >= 2", row.TruncatedResults)
	}
}

func TestTransactionLogFiles(t *testing.T) {
	proxyURL, db, logDir := setupTestProxy(t)

	resp := sendRequest(t, proxyURL, false, nil, nil)
	defer resp.Body.Close()
	io.ReadAll(resp.Body)

	row := getDBRow(t, db)

	if row.RequestLogPath == "" {
		t.Fatal("DB RequestLogPath is empty")
	}
	if row.ResponseLogPath == "" {
		t.Fatal("DB ResponseLogPath is empty")
	}

	// Verify files exist
	if _, err := os.Stat(row.RequestLogPath); err != nil {
		t.Errorf("request log file does not exist: %v", err)
	}
	if _, err := os.Stat(row.ResponseLogPath); err != nil {
		t.Errorf("response log file does not exist: %v", err)
	}

	// Verify request log is valid JSON
	reqData, err := os.ReadFile(row.RequestLogPath)
	if err != nil {
		t.Fatalf("reading request log: %v", err)
	}
	var reqLog map[string]any
	if err := json.Unmarshal(reqData, &reqLog); err != nil {
		t.Fatalf("request log not valid JSON: %v", err)
	}
	if reqLog["method"] != "POST" {
		t.Errorf("request log method = %v, want POST", reqLog["method"])
	}

	// Verify files are within logDir
	if !strings.HasPrefix(row.RequestLogPath, logDir) {
		t.Errorf("request log path %q not in logDir %q", row.RequestLogPath, logDir)
	}
}

func TestMessageCount(t *testing.T) {
	proxyURL, db, _ := setupTestProxy(t)

	body := map[string]any{
		"messages": []map[string]any{
			{"role": "user", "content": "msg 1"},
			{"role": "assistant", "content": "msg 2"},
			{"role": "user", "content": "msg 3"},
			{"role": "assistant", "content": "msg 4"},
			{"role": "user", "content": "msg 5"},
		},
	}

	resp := sendRequest(t, proxyURL, false, nil, body)
	defer resp.Body.Close()
	io.ReadAll(resp.Body)

	row := getDBRow(t, db)
	if row.MessageCount != 5 {
		t.Errorf("DB MessageCount = %d, want 5", row.MessageCount)
	}
}
