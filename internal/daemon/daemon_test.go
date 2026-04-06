package daemon

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/BrenanL/hitch/internal/state"
)

func testDaemon(t *testing.T, db *state.DB) (*Daemon, *httptest.Server) {
	t.Helper()
	d := NewWithPIDPath(0, db, filepath.Join(t.TempDir(), "test.pid"))
	d.startTime = time.Now()
	mux := http.NewServeMux()
	d.registerRoutes(mux)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return d, srv
}

func TestDaemonHealthEndpoint(t *testing.T) {
	db, err := state.OpenInMemory()
	if err != nil {
		t.Fatalf("OpenInMemory: %v", err)
	}
	defer db.Close()

	d, srv := testDaemon(t, db)
	d.startTime = time.Now().Add(-1 * time.Hour)

	resp, err := http.Get(srv.URL + "/health")
	if err != nil {
		t.Fatalf("GET /health: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	var health HealthResponse
	json.NewDecoder(resp.Body).Decode(&health)

	if health.Status != "ok" {
		t.Errorf("status = %q, want %q", health.Status, "ok")
	}
	if health.UptimeSeconds < 3500 {
		t.Errorf("uptime = %d, want >= 3500", health.UptimeSeconds)
	}
}

func TestDaemonHealthContentType(t *testing.T) {
	db, err := state.OpenInMemory()
	if err != nil {
		t.Fatalf("OpenInMemory: %v", err)
	}
	defer db.Close()

	_, srv := testDaemon(t, db)

	resp, err := http.Get(srv.URL + "/health")
	if err != nil {
		t.Fatalf("GET /health: %v", err)
	}
	defer resp.Body.Close()

	ct := resp.Header.Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("Content-Type = %q, want %q", ct, "application/json")
	}
}

func TestDaemonSessionsEndpoint(t *testing.T) {
	db, err := state.OpenInMemory()
	if err != nil {
		t.Fatalf("OpenInMemory: %v", err)
	}
	defer db.Close()

	d, srv := testDaemon(t, db)

	d.Tracker.UpdateFromRequest(state.APIRequest{
		SessionID:    "s1",
		Model:        "claude-opus-4-6-20250514",
		InputTokens:  10000,
		OutputTokens: 2000,
	})
	d.Tracker.UpdateFromRequest(state.APIRequest{
		SessionID:    "s2",
		Model:        "claude-sonnet-4-6-20250514",
		InputTokens:  5000,
		OutputTokens: 1000,
	})

	resp, err := http.Get(srv.URL + "/api/sessions")
	if err != nil {
		t.Fatalf("GET /api/sessions: %v", err)
	}
	defer resp.Body.Close()

	var sessions []SessionSummary
	json.NewDecoder(resp.Body).Decode(&sessions)

	if len(sessions) != 2 {
		t.Fatalf("sessions count = %d, want 2", len(sessions))
	}

	found := false
	for _, s := range sessions {
		if s.SessionID == "s1" {
			found = true
			if s.TotalTokens != 12000 {
				t.Errorf("s1 TotalTokens = %d, want 12000", s.TotalTokens)
			}
			if s.Model != "claude-opus-4-6-20250514" {
				t.Errorf("s1 Model = %q", s.Model)
			}
		}
	}
	if !found {
		t.Error("session s1 not found in response")
	}
}

func TestDaemonSessionByIDEndpoint(t *testing.T) {
	db, err := state.OpenInMemory()
	if err != nil {
		t.Fatalf("OpenInMemory: %v", err)
	}
	defer db.Close()

	d, srv := testDaemon(t, db)

	d.Tracker.UpdateFromRequest(state.APIRequest{
		SessionID:    "s1",
		Model:        "claude-opus-4-6-20250514",
		InputTokens:  10000,
		OutputTokens: 2000,
	})

	resp, err := http.Get(srv.URL + "/api/sessions/s1")
	if err != nil {
		t.Fatalf("GET /api/sessions/s1: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	var detail SessionDetail
	json.NewDecoder(resp.Body).Decode(&detail)

	if detail.SessionID != "s1" {
		t.Errorf("SessionID = %q", detail.SessionID)
	}
	if detail.TotalInputTokens != 10000 {
		t.Errorf("TotalInputTokens = %d, want 10000", detail.TotalInputTokens)
	}
	if detail.TotalOutputTokens != 2000 {
		t.Errorf("TotalOutputTokens = %d, want 2000", detail.TotalOutputTokens)
	}
}

func TestDaemonSessionNotFound(t *testing.T) {
	db, err := state.OpenInMemory()
	if err != nil {
		t.Fatalf("OpenInMemory: %v", err)
	}
	defer db.Close()

	_, srv := testDaemon(t, db)

	resp, err := http.Get(srv.URL + "/api/sessions/nonexistent")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 404 {
		t.Errorf("status = %d, want 404", resp.StatusCode)
	}
}

func TestDaemonStatsEndpoint(t *testing.T) {
	db, err := state.OpenInMemory()
	if err != nil {
		t.Fatalf("OpenInMemory: %v", err)
	}
	defer db.Close()

	d, srv := testDaemon(t, db)
	d.Tracker.GetOrCreate("s1")

	resp, err := http.Get(srv.URL + "/api/stats")
	if err != nil {
		t.Fatalf("GET /api/stats: %v", err)
	}
	defer resp.Body.Close()

	var stats StatsResponse
	json.NewDecoder(resp.Body).Decode(&stats)

	if stats.TotalTrackedSessions != 1 {
		t.Errorf("TotalTrackedSessions = %d, want 1", stats.TotalTrackedSessions)
	}
	if stats.ActiveSessions != 1 {
		t.Errorf("ActiveSessions = %d, want 1", stats.ActiveSessions)
	}
}

func TestDaemonAlertsEndpoint(t *testing.T) {
	db, err := state.OpenInMemory()
	if err != nil {
		t.Fatalf("OpenInMemory: %v", err)
	}
	defer db.Close()

	_, srv := testDaemon(t, db)

	resp, err := http.Get(srv.URL + "/api/alerts")
	if err != nil {
		t.Fatalf("GET /api/alerts: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
}

func TestDaemonSessionsActiveFilter(t *testing.T) {
	db, err := state.OpenInMemory()
	if err != nil {
		t.Fatalf("OpenInMemory: %v", err)
	}
	defer db.Close()

	d, srv := testDaemon(t, db)

	d.Tracker.UpdateFromRequest(state.APIRequest{SessionID: "active1", InputTokens: 100})
	s := d.Tracker.GetOrCreate("inactive1")
	s.IsActive = false

	resp, err := http.Get(srv.URL + "/api/sessions?active=true")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()

	var sessions []SessionSummary
	json.NewDecoder(resp.Body).Decode(&sessions)

	if len(sessions) != 1 {
		t.Errorf("active sessions = %d, want 1", len(sessions))
	}
}

func TestDaemonPollOnce(t *testing.T) {
	db, err := state.OpenInMemory()
	if err != nil {
		t.Fatalf("OpenInMemory: %v", err)
	}
	defer db.Close()

	db.InsertAPIRequest(state.APIRequest{
		SessionID:    "poll-test",
		Model:        "claude-opus-4-6-20250514",
		InputTokens:  5000,
		OutputTokens: 1000,
	})

	d := NewWithPIDPath(0, db, filepath.Join(t.TempDir(), "test.pid"))
	d.pollOnce()

	if d.pollCount.Load() != 1 {
		t.Errorf("pollCount = %d, want 1", d.pollCount.Load())
	}

	s := d.Tracker.Get("poll-test")
	if s == nil {
		t.Fatal("session 'poll-test' should exist after poll")
	}
	if s.Model != "claude-opus-4-6-20250514" {
		t.Errorf("Model = %q", s.Model)
	}
}

func TestDaemonAlreadyRunning(t *testing.T) {
	dir := t.TempDir()
	pidPath := filepath.Join(dir, "test.pid")
	myPID := os.Getpid()

	WritePID(pidPath, PIDInfo{
		PID:       myPID,
		Port:      9801,
		StartedAt: time.Now().Format(time.RFC3339),
	})

	d := NewWithPIDPath(9801, nil, pidPath)
	err := d.Start(true)
	if err == nil {
		t.Fatal("expected error for already-running daemon")
	}
	expected := fmt.Sprintf("daemon already running (PID %d, port 9801)", myPID)
	if err.Error() != expected {
		t.Errorf("error = %q, want %q", err.Error(), expected)
	}
}

func TestStopNotRunning(t *testing.T) {
	err := Stop(filepath.Join(t.TempDir(), "nonexistent.pid"))
	if err == nil {
		t.Error("expected error stopping non-running daemon")
	}
}

func TestStopStalePID(t *testing.T) {
	dir := t.TempDir()
	pidPath := filepath.Join(dir, "test.pid")
	WritePID(pidPath, PIDInfo{PID: 99999999, Port: 9801})

	err := Stop(pidPath)
	if err == nil {
		t.Error("expected error for stale PID")
	}
}

func TestStatusNotRunning(t *testing.T) {
	status, err := Status(filepath.Join(t.TempDir(), "nonexistent.pid"), 9801)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if status.Running {
		t.Error("should not be running")
	}
}

func TestStatusStalePID(t *testing.T) {
	dir := t.TempDir()
	pidPath := filepath.Join(dir, "test.pid")
	WritePID(pidPath, PIDInfo{PID: 99999999, Port: 9801})

	status, err := Status(pidPath, 9801)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if status.Running {
		t.Error("should not be running with stale PID")
	}
	if status.StalePID != 99999999 {
		t.Errorf("StalePID = %d, want 99999999", status.StalePID)
	}
}

func TestDaemonSessionsLimit(t *testing.T) {
	db, err := state.OpenInMemory()
	if err != nil {
		t.Fatalf("OpenInMemory: %v", err)
	}
	defer db.Close()

	d, srv := testDaemon(t, db)

	for i := 0; i < 10; i++ {
		d.Tracker.UpdateFromRequest(state.APIRequest{
			SessionID:   fmt.Sprintf("s%d", i),
			InputTokens: 100,
		})
	}

	resp, err := http.Get(srv.URL + "/api/sessions?limit=3")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()

	var sessions []SessionSummary
	json.NewDecoder(resp.Body).Decode(&sessions)

	if len(sessions) != 3 {
		t.Errorf("sessions = %d, want 3", len(sessions))
	}
}

func TestDaemonSessionEventsEndpoint(t *testing.T) {
	db, err := state.OpenInMemory()
	if err != nil {
		t.Fatalf("OpenInMemory: %v", err)
	}
	defer db.Close()

	// Insert some events
	db.EventLog(state.Event{SessionID: "s1", HookEvent: "SubagentStart", ToolName: "agent-1"})
	db.EventLog(state.Event{SessionID: "s1", HookEvent: "PostCompact"})
	db.EventLog(state.Event{SessionID: "s2", HookEvent: "Stop"})

	d, srv := testDaemon(t, db)
	d.Tracker.GetOrCreate("s1")

	resp, err := http.Get(srv.URL + "/api/sessions/s1/events")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	var events []DaemonEvent
	json.NewDecoder(resp.Body).Decode(&events)

	if len(events) != 2 {
		t.Errorf("events = %d, want 2 (filtered to s1)", len(events))
	}
}

func TestDaemonSessionEventsLimit(t *testing.T) {
	db, err := state.OpenInMemory()
	if err != nil {
		t.Fatalf("OpenInMemory: %v", err)
	}
	defer db.Close()

	for i := 0; i < 10; i++ {
		db.EventLog(state.Event{SessionID: "s1", HookEvent: "PostCompact"})
	}

	_, srv := testDaemon(t, db)

	resp, err := http.Get(srv.URL + "/api/sessions/s1/events?limit=3")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()

	var events []DaemonEvent
	json.NewDecoder(resp.Body).Decode(&events)

	if len(events) != 3 {
		t.Errorf("events = %d, want 3", len(events))
	}
}

func TestDaemonSessionDetailWithSubagents(t *testing.T) {
	db, err := state.OpenInMemory()
	if err != nil {
		t.Fatalf("OpenInMemory: %v", err)
	}
	defer db.Close()

	d, srv := testDaemon(t, db)
	d.Tracker.UpdateFromRequest(state.APIRequest{
		SessionID:   "s1",
		InputTokens: 1000,
	})
	d.Tracker.UpdateFromEvent(state.Event{
		ID:          1,
		SessionID:   "s1",
		HookEvent:   "SubagentStart",
		ToolName:    "agent-abc",
		ActionTaken: "Explore",
	})

	resp, err := http.Get(srv.URL + "/api/sessions/s1")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()

	var detail SessionDetail
	json.NewDecoder(resp.Body).Decode(&detail)

	if len(detail.ActiveSubagents) != 1 {
		t.Fatalf("ActiveSubagents = %d, want 1", len(detail.ActiveSubagents))
	}
	if detail.ActiveSubagents[0].AgentID != "agent-abc" {
		t.Errorf("AgentID = %q", detail.ActiveSubagents[0].AgentID)
	}
	if detail.ActiveSubagents[0].AgentType != "Explore" {
		t.Errorf("AgentType = %q", detail.ActiveSubagents[0].AgentType)
	}
}

func TestDaemonSessionLastRequestNotFound(t *testing.T) {
	db, err := state.OpenInMemory()
	if err != nil {
		t.Fatalf("OpenInMemory: %v", err)
	}
	defer db.Close()

	_, srv := testDaemon(t, db)

	resp, err := http.Get(srv.URL + "/api/sessions/nonexistent/last-request")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 404 {
		t.Errorf("status = %d, want 404", resp.StatusCode)
	}
}

func TestDaemonPollBroadcastsSSE(t *testing.T) {
	db, err := state.OpenInMemory()
	if err != nil {
		t.Fatalf("OpenInMemory: %v", err)
	}
	defer db.Close()

	d := NewWithPIDPath(0, db, filepath.Join(t.TempDir(), "test.pid"))

	// Subscribe before polling
	ch, unsub := d.SSEHub.Subscribe("sse-test")
	defer unsub()

	db.InsertAPIRequest(state.APIRequest{
		SessionID:    "sse-test",
		Model:        "claude-sonnet-4-6-20250514",
		InputTokens:  1000,
		OutputTokens: 200,
	})

	d.pollOnce()

	select {
	case evt := <-ch:
		if evt.Source != "proxy" {
			t.Errorf("Source = %q, want %q", evt.Source, "proxy")
		}
		if evt.SessionID != "sse-test" {
			t.Errorf("SessionID = %q", evt.SessionID)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for SSE broadcast")
	}
}
