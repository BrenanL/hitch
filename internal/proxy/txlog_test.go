package proxy

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSanitizeHeadersRedactsAPIKey(t *testing.T) {
	h := http.Header{}
	h.Set("X-Api-Key", "sk-secret-123")
	h.Set("Content-Type", "application/json")

	sanitized := SanitizeHeaders(h)
	if sanitized["X-Api-Key"][0] != "[REDACTED]" {
		t.Errorf("X-Api-Key should be redacted, got %q", sanitized["X-Api-Key"][0])
	}
	if sanitized["Content-Type"][0] != "application/json" {
		t.Errorf("Content-Type should be preserved, got %q", sanitized["Content-Type"][0])
	}
}

func TestSanitizeHeadersRedactsAuthorization(t *testing.T) {
	h := http.Header{}
	h.Set("Authorization", "Bearer sk-ant-12345")

	sanitized := SanitizeHeaders(h)
	if sanitized["Authorization"][0] != "[REDACTED]" {
		t.Errorf("Authorization should be redacted, got %q", sanitized["Authorization"][0])
	}
}

func TestSanitizeHeadersPreservesOthers(t *testing.T) {
	h := http.Header{}
	h.Set("Anthropic-Version", "2023-06-01")
	h.Set("User-Agent", "claude-code/1.0")

	sanitized := SanitizeHeaders(h)
	if sanitized["Anthropic-Version"][0] != "2023-06-01" {
		t.Errorf("Anthropic-Version should be preserved")
	}
	if sanitized["User-Agent"][0] != "claude-code/1.0" {
		t.Errorf("User-Agent should be preserved")
	}
}

func TestWriteRequestLog(t *testing.T) {
	logger := NewTransactionLoggerWithDir(t.TempDir())
	ts := time.Now()
	headers := http.Header{}
	headers.Set("Content-Type", "application/json")
	headers.Set("X-Api-Key", "sk-secret")
	body := []byte(`{"model":"claude-opus-4-6","messages":[]}`)

	path := logger.WriteRequestLog(ts, "POST", "https://api.anthropic.com/v1/messages", headers, body)
	if path == "" {
		t.Fatal("WriteRequestLog returned empty path")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading log file: %v", err)
	}

	var entry struct {
		Method  string              `json:"method"`
		URL     string              `json:"url"`
		Headers map[string][]string `json:"headers"`
		Body    json.RawMessage     `json:"body"`
	}
	if err := json.Unmarshal(data, &entry); err != nil {
		t.Fatalf("parsing log file JSON: %v", err)
	}

	if entry.Method != "POST" {
		t.Errorf("Method = %q, want POST", entry.Method)
	}
	if entry.URL != "https://api.anthropic.com/v1/messages" {
		t.Errorf("URL = %q", entry.URL)
	}
	if entry.Headers["X-Api-Key"][0] != "[REDACTED]" {
		t.Error("API key should be redacted in log file")
	}
	if entry.Headers["Content-Type"][0] != "application/json" {
		t.Error("Content-Type should be preserved in log file")
	}
	if !json.Valid(entry.Body) {
		t.Error("Body should be valid JSON")
	}
}

func TestCreateResponseLog(t *testing.T) {
	logger := NewTransactionLoggerWithDir(t.TempDir())
	ts := time.Now()
	headers := http.Header{}
	headers.Set("Content-Type", "text/event-stream")

	respLog := logger.CreateResponseLog(ts, "test_resp", 200, headers)
	defer respLog.Close()

	respLog.WriteLine("event: message_start")
	respLog.WriteLine("data: {}")
	respLog.Close()

	data, err := os.ReadFile(respLog.Path())
	if err != nil {
		t.Fatalf("reading response log: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) < 3 {
		t.Fatalf("expected at least 3 lines, got %d", len(lines))
	}

	// First line should be JSON metadata
	var meta struct {
		Status  int                 `json:"status"`
		Headers map[string][]string `json:"headers"`
	}
	if err := json.Unmarshal([]byte(lines[0]), &meta); err != nil {
		t.Fatalf("first line should be valid JSON metadata: %v", err)
	}
	if meta.Status != 200 {
		t.Errorf("metadata status = %d, want 200", meta.Status)
	}

	if lines[1] != "event: message_start" {
		t.Errorf("line 2 = %q, want %q", lines[1], "event: message_start")
	}
	if lines[2] != "data: {}" {
		t.Errorf("line 3 = %q, want %q", lines[2], "data: {}")
	}
}

func TestResponseLogSize(t *testing.T) {
	logger := NewTransactionLoggerWithDir(t.TempDir())
	ts := time.Now()

	respLog := logger.CreateResponseLog(ts, "size_test", 200, http.Header{})
	initialSize := respLog.Size() // metadata line size

	respLog.WriteLine("hello") // 5 + 1 newline = 6 bytes
	afterFirst := respLog.Size()
	if afterFirst != initialSize+6 {
		t.Errorf("size after first write = %d, want %d", afterFirst, initialSize+6)
	}

	respLog.WriteBody([]byte("world")) // 5 bytes
	afterSecond := respLog.Size()
	if afterSecond != afterFirst+5 {
		t.Errorf("size after second write = %d, want %d", afterSecond, afterFirst+5)
	}
	respLog.Close()
}

func TestTransactionLoggerDirCreation(t *testing.T) {
	base := t.TempDir()
	logger := NewTransactionLoggerWithDir(base)
	ts := time.Now()

	path := logger.WriteRequestLog(ts, "GET", "/", http.Header{}, nil)
	if path == "" {
		t.Fatal("WriteRequestLog returned empty path")
	}

	dateDir := filepath.Join(base, ts.Format("2006-01-02"))
	info, err := os.Stat(dateDir)
	if err != nil {
		t.Fatalf("date directory should exist: %v", err)
	}
	if !info.IsDir() {
		t.Error("date path should be a directory")
	}
}
