package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/BrenanL/hitch/internal/state"
)

// RequestLog collects data during request processing.
type RequestLog struct {
	RequestID           string
	SessionID           string
	Model               string
	InputTokens         int
	OutputTokens        int
	CacheReadTokens     int
	CacheCreationTokens int
	CostUSD             float64
	LatencyMS           int64
	StopReason          string
	Streaming           bool
	Endpoint            string
	Error               string
	MicrocompactCount   int
	TruncatedResults    int
	TotalToolResultSize int
}

// Server is the transparent logging proxy between Claude Code and the Anthropic API.
type Server struct {
	port       int
	upstream   string
	db         *state.DB
	httpServer *http.Server
	startTime  time.Time
	reqCount   atomic.Int64
	pidFile    string
}

// NewServer creates a new proxy server.
func NewServer(port int, db *state.DB) *Server {
	home, _ := os.UserHomeDir()
	return &Server{
		port:     port,
		upstream: "https://api.anthropic.com",
		db:       db,
		pidFile:  filepath.Join(home, ".hitch", "proxy.pid"),
	}
}

// Start starts the proxy server and blocks until SIGINT/SIGTERM.
func (s *Server) Start() error {
	s.httpServer = &http.Server{
		Addr:         fmt.Sprintf("localhost:%d", s.port),
		Handler:      s,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 0, // no write timeout — SSE streams can last minutes
		IdleTimeout:  120 * time.Second,
	}
	s.startTime = time.Now()

	if err := s.writePID(); err != nil {
		return fmt.Errorf("writing PID file: %w", err)
	}
	defer s.removePID()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	errCh := make(chan error, 1)
	go func() {
		log.Printf("hitch proxy listening on localhost:%d -> %s", s.port, s.upstream)
		errCh <- s.httpServer.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		if err != http.ErrServerClosed {
			return err
		}
	case sig := <-sigCh:
		log.Printf("received %v, shutting down", sig)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		s.httpServer.Shutdown(ctx)
	}

	return nil
}

// ServeHTTP handles all incoming requests.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/health" {
		s.handleHealth(w)
		return
	}

	start := time.Now()

	// Capture request body
	bodyBytes, err := io.ReadAll(r.Body)
	r.Body.Close()
	if err != nil {
		http.Error(w, "failed to read request body", http.StatusBadGateway)
		return
	}
	r.Body = io.NopCloser(bytes.NewReader(bodyBytes))

	// Extract request metadata
	var reqMeta struct {
		Stream bool   `json:"stream"`
		Model  string `json:"model"`
	}
	json.Unmarshal(bodyBytes, &reqMeta)

	// Bug detection on outgoing request
	microcompactCount := detectMicrocompact(bodyBytes)
	truncatedCount, toolResultSize := detectBudgetTruncation(bodyBytes)

	if microcompactCount > 0 {
		log.Printf("[detect] microcompact: %d cleared results", microcompactCount)
	}
	if truncatedCount > 0 {
		log.Printf("[detect] budget truncation: %d truncated tool results (%d bytes total)", truncatedCount, toolResultSize)
	}

	// Forward request to upstream
	resp, err := s.forwardRequest(r)
	if err != nil {
		http.Error(w, fmt.Sprintf("upstream error: %v", err), http.StatusBadGateway)
		s.saveLog(&RequestLog{
			Endpoint:            r.URL.Path,
			Model:               reqMeta.Model,
			Streaming:           reqMeta.Stream,
			LatencyMS:           time.Since(start).Milliseconds(),
			Error:               err.Error(),
			MicrocompactCount:   microcompactCount,
			TruncatedResults:    truncatedCount,
			TotalToolResultSize: toolResultSize,
			SessionID:           extractSessionID(r),
		})
		return
	}
	defer resp.Body.Close()

	// Build log record
	rec := &RequestLog{
		Endpoint:            r.URL.Path,
		Streaming:           reqMeta.Stream,
		Model:               reqMeta.Model,
		MicrocompactCount:   microcompactCount,
		TruncatedResults:    truncatedCount,
		TotalToolResultSize: toolResultSize,
		SessionID:           extractSessionID(r),
	}

	// Choose streaming vs non-streaming path
	isStream := reqMeta.Stream && resp.StatusCode < 400 &&
		strings.Contains(resp.Header.Get("Content-Type"), "text/event-stream")

	if isStream {
		s.handleStreaming(w, resp, rec)
	} else {
		s.handleNonStreaming(w, resp, rec)
	}

	// Finalize record
	rec.LatencyMS = time.Since(start).Milliseconds()
	rec.CostUSD = estimateCost(rec.Model, rec.InputTokens, rec.OutputTokens,
		rec.CacheReadTokens, rec.CacheCreationTokens)

	s.reqCount.Add(1)
	s.saveLog(rec)
}

func (s *Server) handleHealth(w http.ResponseWriter) {
	uptime := int64(time.Since(s.startTime).Seconds())
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"status":          "ok",
		"uptime_seconds":  uptime,
		"requests_logged": s.reqCount.Load(),
		"port":            s.port,
	})
}

func (s *Server) saveLog(rec *RequestLog) {
	if s.db == nil {
		return
	}
	if err := s.db.InsertAPIRequest(state.APIRequest{
		RequestID:           rec.RequestID,
		SessionID:           rec.SessionID,
		Model:               rec.Model,
		InputTokens:         rec.InputTokens,
		OutputTokens:        rec.OutputTokens,
		CacheReadTokens:     rec.CacheReadTokens,
		CacheCreationTokens: rec.CacheCreationTokens,
		CostUSD:             rec.CostUSD,
		LatencyMS:           rec.LatencyMS,
		StopReason:          rec.StopReason,
		Streaming:           rec.Streaming,
		Endpoint:            rec.Endpoint,
		Error:               rec.Error,
		MicrocompactCount:   rec.MicrocompactCount,
		TruncatedResults:    rec.TruncatedResults,
		TotalToolResultSize: rec.TotalToolResultSize,
	}); err != nil {
		log.Printf("error saving log: %v", err)
	}
}

func (s *Server) writePID() error {
	data, _ := json.Marshal(struct {
		PID  int `json:"pid"`
		Port int `json:"port"`
	}{os.Getpid(), s.port})
	return os.WriteFile(s.pidFile, data, 0o644)
}

func (s *Server) removePID() {
	os.Remove(s.pidFile)
}

func extractSessionID(r *http.Request) string {
	for _, h := range []string{"X-Session-Id", "Anthropic-Session-Id"} {
		if id := r.Header.Get(h); id != "" {
			return id
		}
	}
	return ""
}
