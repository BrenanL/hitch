package proxy

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"
)

// TransactionLogger writes full request/response bodies to disk.
type TransactionLogger struct {
	baseDir string
	counter atomic.Int64
}

// NewTransactionLoggerWithDir creates a logger that writes to the given directory.
func NewTransactionLoggerWithDir(dir string) *TransactionLogger {
	return &TransactionLogger{baseDir: dir}
}

// NewTransactionLogger creates a logger that writes to ~/.hitch/proxy-logs/.
func NewTransactionLogger() *TransactionLogger {
	home, _ := os.UserHomeDir()
	return NewTransactionLoggerWithDir(filepath.Join(home, ".hitch", "proxy-logs"))
}

func (t *TransactionLogger) logDir(ts time.Time) string {
	dir := filepath.Join(t.baseDir, ts.Format("2006-01-02"))
	os.MkdirAll(dir, 0o755)
	return dir
}

func (t *TransactionLogger) nextID(ts time.Time) string {
	n := t.counter.Add(1)
	return fmt.Sprintf("%s_%06d", ts.Format("150405"), n)
}

// WriteRequestLog writes the full request (headers + body) to a JSON file.
// Returns the file path.
func (t *TransactionLogger) WriteRequestLog(ts time.Time, method, url string, headers http.Header, body []byte) string {
	id := t.nextID(ts)
	dir := t.logDir(ts)
	path := filepath.Join(dir, id+".req.json")

	entry := struct {
		Method  string              `json:"method"`
		URL     string              `json:"url"`
		Headers map[string][]string `json:"headers"`
		Body    json.RawMessage     `json:"body,omitempty"`
	}{
		Method:  method,
		URL:     url,
		Headers: SanitizeHeaders(headers),
	}

	if json.Valid(body) {
		entry.Body = body
	}

	data, err := json.Marshal(entry)
	if err != nil {
		return ""
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return ""
	}
	return path
}

// ResponseLog captures the response body incrementally.
type ResponseLog struct {
	file *os.File
	path string
	size int64
}

// CreateResponseLog creates a response log file and writes the metadata header line.
func (t *TransactionLogger) CreateResponseLog(ts time.Time, id string, statusCode int, headers http.Header) *ResponseLog {
	dir := t.logDir(ts)
	path := filepath.Join(dir, id+".resp.log")

	f, err := os.Create(path)
	if err != nil {
		return &ResponseLog{} // noop logger
	}

	// First line: JSON metadata
	meta := struct {
		Status  int                 `json:"status"`
		Headers map[string][]string `json:"headers"`
	}{
		Status:  statusCode,
		Headers: SanitizeHeaders(headers),
	}
	data, _ := json.Marshal(meta)
	f.Write(data)
	f.WriteString("\n")

	return &ResponseLog{file: f, path: path, size: int64(len(data)) + 1}
}

// WriteLine writes a single line to the response log.
func (r *ResponseLog) WriteLine(line string) {
	if r.file == nil {
		return
	}
	n, _ := r.file.WriteString(line)
	r.file.WriteString("\n")
	r.size += int64(n) + 1
}

// WriteBody writes a complete response body (for non-streaming).
func (r *ResponseLog) WriteBody(body []byte) {
	if r.file == nil {
		return
	}
	n, _ := r.file.Write(body)
	r.size += int64(n)
}

// Close closes the response log file.
func (r *ResponseLog) Close() {
	if r.file != nil {
		r.file.Close()
	}
}

// Path returns the file path.
func (r *ResponseLog) Path() string {
	return r.path
}

// Size returns bytes written.
func (r *ResponseLog) Size() int64 {
	return r.size
}

// SanitizeHeaders returns a copy of headers with sensitive values redacted.
func SanitizeHeaders(h http.Header) map[string][]string {
	sanitized := make(map[string][]string, len(h))
	for key, vals := range h {
		lower := strings.ToLower(key)
		if lower == "x-api-key" || lower == "authorization" {
			sanitized[key] = []string{"[REDACTED]"}
			continue
		}
		sanitized[key] = vals
	}
	return sanitized
}
