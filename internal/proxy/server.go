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
	HTTPMethod          string
	HTTPStatus          int
	InputTokens         int
	OutputTokens        int
	CacheReadTokens     int
	CacheCreationTokens int
	LatencyMS           int64
	StopReason          string
	Streaming           bool
	Endpoint            string
	Error               string
	MicrocompactCount   int
	TruncatedResults    int
	TotalToolResultSize int
	RequestHeaders      string
	ResponseHeaders     string
	RequestBodySize     int64
	ResponseBodySize    int64
	RequestLogPath      string
	ResponseLogPath     string
	MessageCount        int
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
	txlog      *TransactionLogger
}

// NewServer creates a new proxy server.
func NewServer(port int, db *state.DB) *Server {
	home, _ := os.UserHomeDir()
	return &Server{
		port:     port,
		upstream: "https://api.anthropic.com",
		db:       db,
		pidFile:  filepath.Join(home, ".hitch", "proxy.pid"),
		txlog:    NewTransactionLogger(),
	}
}

// Start starts the proxy server and blocks until SIGINT/SIGTERM.
func (s *Server) Start() error {
	// Seed pricing file on first run
	SeedPricingFile()

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
		Stream   bool              `json:"stream"`
		Model    string            `json:"model"`
		Messages []json.RawMessage `json:"messages"`
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

	// Capture request headers
	reqHeadersJSON, _ := json.Marshal(SanitizeHeaders(r.Header))
	sessionID := r.Header.Get("X-Claude-Code-Session-Id")

	// Write request body to transaction log
	txID := s.txlog.nextID(start)
	reqLogPath := s.txlog.WriteRequestLog(start, r.Method, r.URL.Path, r.Header, bodyBytes)

	// Forward request to upstream
	resp, err := s.forwardRequest(r)
	if err != nil {
		http.Error(w, fmt.Sprintf("upstream error: %v", err), http.StatusBadGateway)
		s.saveLog(&RequestLog{
			Endpoint:            r.URL.Path,
			HTTPMethod:          r.Method,
			Model:               reqMeta.Model,
			Streaming:           reqMeta.Stream,
			LatencyMS:           time.Since(start).Milliseconds(),
			Error:               err.Error(),
			SessionID:           sessionID,
			RequestHeaders:      string(reqHeadersJSON),
			RequestBodySize:     int64(len(bodyBytes)),
			RequestLogPath:      reqLogPath,
			MessageCount:        len(reqMeta.Messages),
			MicrocompactCount:   microcompactCount,
			TruncatedResults:    truncatedCount,
			TotalToolResultSize: toolResultSize,
		})
		return
	}
	defer resp.Body.Close()

	// Capture response headers
	respHeadersJSON, _ := json.Marshal(SanitizeHeaders(resp.Header))

	// Create response log
	respLog := s.txlog.CreateResponseLog(start, txID, resp.StatusCode, resp.Header)
	defer respLog.Close()

	// Build log record
	rec := &RequestLog{
		Endpoint:            r.URL.Path,
		HTTPMethod:          r.Method,
		HTTPStatus:          resp.StatusCode,
		Streaming:           reqMeta.Stream,
		Model:               reqMeta.Model,
		SessionID:           sessionID,
		MessageCount:        len(reqMeta.Messages),
		RequestHeaders:      string(reqHeadersJSON),
		ResponseHeaders:     string(respHeadersJSON),
		RequestBodySize:     int64(len(bodyBytes)),
		RequestLogPath:      reqLogPath,
		MicrocompactCount:   microcompactCount,
		TruncatedResults:    truncatedCount,
		TotalToolResultSize: toolResultSize,
	}

	// Choose streaming vs non-streaming path
	isStream := reqMeta.Stream && resp.StatusCode < 400 &&
		strings.Contains(resp.Header.Get("Content-Type"), "text/event-stream")

	if isStream {
		s.handleStreaming(w, resp, rec, respLog)
	} else {
		s.handleNonStreaming(w, resp, rec, respLog)
	}

	// Finalize record
	rec.LatencyMS = time.Since(start).Milliseconds()
	rec.ResponseLogPath = respLog.Path()
	rec.ResponseBodySize = respLog.Size()

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
		HTTPMethod:          rec.HTTPMethod,
		HTTPStatus:          rec.HTTPStatus,
		InputTokens:         rec.InputTokens,
		OutputTokens:        rec.OutputTokens,
		CacheReadTokens:     rec.CacheReadTokens,
		CacheCreationTokens: rec.CacheCreationTokens,
		LatencyMS:           rec.LatencyMS,
		StopReason:          rec.StopReason,
		Streaming:           rec.Streaming,
		Endpoint:            rec.Endpoint,
		Error:               rec.Error,
		MicrocompactCount:   rec.MicrocompactCount,
		TruncatedResults:    rec.TruncatedResults,
		TotalToolResultSize: rec.TotalToolResultSize,
		RequestHeaders:      rec.RequestHeaders,
		ResponseHeaders:     rec.ResponseHeaders,
		RequestBodySize:     rec.RequestBodySize,
		ResponseBodySize:    rec.ResponseBodySize,
		RequestLogPath:      rec.RequestLogPath,
		ResponseLogPath:     rec.ResponseLogPath,
		MessageCount:        rec.MessageCount,
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
