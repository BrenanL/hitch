package daemon

import (
	"testing"
	"time"

	"github.com/BrenanL/hitch/internal/state"
)

func TestGetOrCreateNewSession(t *testing.T) {
	tracker := NewSessionTracker()
	s := tracker.GetOrCreate("session-1")

	if s.SessionID != "session-1" {
		t.Errorf("SessionID = %q, want %q", s.SessionID, "session-1")
	}
	if !s.IsActive {
		t.Error("new session should be active")
	}
}

func TestGetOrCreateExistingSession(t *testing.T) {
	tracker := NewSessionTracker()
	s1 := tracker.GetOrCreate("session-1")
	s1.Model = "opus"

	s2 := tracker.GetOrCreate("session-1")
	if s2.Model != "opus" {
		t.Error("GetOrCreate should return existing session")
	}
}

func TestGetNonExistent(t *testing.T) {
	tracker := NewSessionTracker()
	if tracker.Get("nonexistent") != nil {
		t.Error("Get should return nil for unknown session")
	}
}

func TestUpdateFromRequest(t *testing.T) {
	tracker := NewSessionTracker()
	tracker.UpdateFromRequest(state.APIRequest{
		SessionID:   "s1",
		Model:       "claude-sonnet-4-6-20250514",
		InputTokens: 1000,
		OutputTokens: 500,
		CacheReadTokens: 200,
	})

	s := tracker.Get("s1")
	if s == nil {
		t.Fatal("session should exist after UpdateFromRequest")
	}
	if s.Model != "claude-sonnet-4-6-20250514" {
		t.Errorf("Model = %q", s.Model)
	}
	if s.TotalInputTokens != 1000 {
		t.Errorf("TotalInputTokens = %d, want 1000", s.TotalInputTokens)
	}
	if s.TotalOutputTokens != 500 {
		t.Errorf("TotalOutputTokens = %d, want 500", s.TotalOutputTokens)
	}
	if s.TotalCacheRead != 200 {
		t.Errorf("TotalCacheRead = %d, want 200", s.TotalCacheRead)
	}
	if s.RequestCount != 1 {
		t.Errorf("RequestCount = %d, want 1", s.RequestCount)
	}
	if !s.IsActive {
		t.Error("session should be active after request")
	}
}

func TestUpdateFromRequestAccumulates(t *testing.T) {
	tracker := NewSessionTracker()

	tracker.UpdateFromRequest(state.APIRequest{
		SessionID:    "s1",
		InputTokens:  1000,
		OutputTokens: 500,
	})
	tracker.UpdateFromRequest(state.APIRequest{
		SessionID:    "s1",
		InputTokens:  2000,
		OutputTokens: 1000,
	})

	s := tracker.Get("s1")
	if s.TotalInputTokens != 3000 {
		t.Errorf("TotalInputTokens = %d, want 3000", s.TotalInputTokens)
	}
	if s.TotalOutputTokens != 1500 {
		t.Errorf("TotalOutputTokens = %d, want 1500", s.TotalOutputTokens)
	}
	if s.RequestCount != 2 {
		t.Errorf("RequestCount = %d, want 2", s.RequestCount)
	}
}

func TestBurnRateCalculation(t *testing.T) {
	tracker := NewSessionTracker()

	// First request
	tracker.UpdateFromRequest(state.APIRequest{
		SessionID:    "s1",
		InputTokens:  1000,
		OutputTokens: 500,
	})

	// Second request (creates burn rate from 2 samples)
	tracker.UpdateFromRequest(state.APIRequest{
		SessionID:    "s1",
		InputTokens:  1000,
		OutputTokens: 500,
	})

	s := tracker.Get("s1")
	if len(s.BurnRateSamples) != 2 {
		t.Errorf("BurnRateSamples count = %d, want 2", len(s.BurnRateSamples))
	}
	// BurnRateTPM may be 0 if samples are too close in time, but should not be negative
	if s.BurnRateTPM < 0 {
		t.Errorf("BurnRateTPM = %f, should not be negative", s.BurnRateTPM)
	}
}

func TestActiveSessions(t *testing.T) {
	tracker := NewSessionTracker()
	tracker.GetOrCreate("s1")
	tracker.GetOrCreate("s2")
	s3 := tracker.GetOrCreate("s3")
	s3.IsActive = false

	all := tracker.ActiveSessions(false, 0)
	if len(all) != 3 {
		t.Errorf("total sessions = %d, want 3", len(all))
	}

	active := tracker.ActiveSessions(true, 0)
	if len(active) != 2 {
		t.Errorf("active sessions = %d, want 2", len(active))
	}
}

func TestActiveSessionsLimit(t *testing.T) {
	tracker := NewSessionTracker()
	for i := 0; i < 10; i++ {
		tracker.GetOrCreate("s" + string(rune('0'+i)))
	}

	limited := tracker.ActiveSessions(false, 3)
	if len(limited) != 3 {
		t.Errorf("limited sessions = %d, want 3", len(limited))
	}
}

func TestCount(t *testing.T) {
	tracker := NewSessionTracker()
	tracker.GetOrCreate("s1")
	tracker.GetOrCreate("s2")
	s3 := tracker.GetOrCreate("s3")
	s3.IsActive = false

	total, active := tracker.Count()
	if total != 3 {
		t.Errorf("total = %d, want 3", total)
	}
	if active != 2 {
		t.Errorf("active = %d, want 2", active)
	}
}

func TestMarkInactive(t *testing.T) {
	tracker := NewSessionTracker()
	s := tracker.GetOrCreate("s1")
	// Set last activity to 10 minutes ago
	s.LastActivity = time.Now().Add(-10 * time.Minute)

	tracker.MarkInactive(5 * time.Minute)
	if s.IsActive {
		t.Error("session should be marked inactive after timeout")
	}
}

func TestMarkInactiveRecentSession(t *testing.T) {
	tracker := NewSessionTracker()
	tracker.GetOrCreate("s1") // just created, should be recent

	tracker.MarkInactive(5 * time.Minute)
	s := tracker.Get("s1")
	if !s.IsActive {
		t.Error("recent session should remain active")
	}
}

func TestPrune(t *testing.T) {
	tracker := NewSessionTracker()
	s := tracker.GetOrCreate("old")
	s.IsActive = false
	s.LastActivity = time.Now().Add(-25 * time.Hour)

	tracker.GetOrCreate("recent") // active, should not be pruned

	tracker.Prune(24 * time.Hour)

	if tracker.Get("old") != nil {
		t.Error("old inactive session should be pruned")
	}
	if tracker.Get("recent") == nil {
		t.Error("recent session should not be pruned")
	}
}

func TestUpdateFromEventSubagentStart(t *testing.T) {
	tracker := NewSessionTracker()
	tracker.UpdateFromEvent(state.Event{
		ID:          1,
		SessionID:   "s1",
		HookEvent:   "SubagentStart",
		ToolName:    "agent-abc",
		ActionTaken: "Explore",
	})

	s := tracker.Get("s1")
	if s == nil {
		t.Fatal("session should exist after SubagentStart event")
	}
	if len(s.ActiveSubagents) != 1 {
		t.Fatalf("ActiveSubagents = %d, want 1", len(s.ActiveSubagents))
	}
	if s.ActiveSubagents[0].AgentID != "agent-abc" {
		t.Errorf("AgentID = %q", s.ActiveSubagents[0].AgentID)
	}
	if s.ActiveSubagents[0].AgentType != "Explore" {
		t.Errorf("AgentType = %q", s.ActiveSubagents[0].AgentType)
	}
	if s.ActiveSubagents[0].StoppedAt != nil {
		t.Error("subagent should not be stopped")
	}
}

func TestUpdateFromEventSubagentStop(t *testing.T) {
	tracker := NewSessionTracker()
	tracker.UpdateFromEvent(state.Event{
		ID:        1,
		SessionID: "s1",
		HookEvent: "SubagentStart",
		ToolName:  "agent-abc",
	})
	tracker.UpdateFromEvent(state.Event{
		ID:        2,
		SessionID: "s1",
		HookEvent: "SubagentStop",
		ToolName:  "agent-abc",
	})

	s := tracker.Get("s1")
	if s.ActiveSubagents[0].StoppedAt == nil {
		t.Error("subagent should be marked as stopped")
	}
}

func TestUpdateFromEventPostCompact(t *testing.T) {
	tracker := NewSessionTracker()
	tracker.UpdateFromEvent(state.Event{
		ID:        1,
		SessionID: "s1",
		HookEvent: "PostCompact",
	})
	tracker.UpdateFromEvent(state.Event{
		ID:        2,
		SessionID: "s1",
		HookEvent: "PostCompact",
	})

	s := tracker.Get("s1")
	if s.CompactionCount != 2 {
		t.Errorf("CompactionCount = %d, want 2", s.CompactionCount)
	}
}

func TestUpdateFromEventSessionStart(t *testing.T) {
	tracker := NewSessionTracker()
	tracker.UpdateFromEvent(state.Event{
		ID:          1,
		SessionID:   "s1",
		HookEvent:   "SessionStart",
		ActionTaken: "~/dev/hitch",
	})

	s := tracker.Get("s1")
	if !s.IsActive {
		t.Error("session should be active after SessionStart")
	}
	if s.ProjectDir != "~/dev/hitch" {
		t.Errorf("ProjectDir = %q", s.ProjectDir)
	}
}

func TestUpdateFromEventStop(t *testing.T) {
	tracker := NewSessionTracker()
	tracker.GetOrCreate("s1")
	tracker.UpdateFromEvent(state.Event{
		ID:        1,
		SessionID: "s1",
		HookEvent: "Stop",
	})

	s := tracker.Get("s1")
	if s.IsActive {
		t.Error("session should be inactive after Stop event")
	}
}

func TestUpdateFromEventEmptySession(t *testing.T) {
	tracker := NewSessionTracker()
	// Event with no session ID should be ignored
	tracker.UpdateFromEvent(state.Event{
		ID:        1,
		HookEvent: "PostCompact",
	})

	total, _ := tracker.Count()
	if total != 0 {
		t.Errorf("total sessions = %d, want 0", total)
	}
}

func TestLastRequestIDTracking(t *testing.T) {
	tracker := NewSessionTracker()
	tracker.UpdateFromRequest(state.APIRequest{
		ID:          42,
		SessionID:   "s1",
		InputTokens: 100,
	})

	if tracker.LastRequestID() != 42 {
		t.Errorf("LastRequestID = %d, want 42", tracker.LastRequestID())
	}
}

func TestLastEventIDTracking(t *testing.T) {
	tracker := NewSessionTracker()
	tracker.UpdateFromEvent(state.Event{
		ID:        17,
		SessionID: "s1",
		HookEvent: "PostCompact",
	})

	if tracker.LastEventID() != 17 {
		t.Errorf("LastEventID = %d, want 17", tracker.LastEventID())
	}
}

func TestComputeBurnRateUsesMetrics(t *testing.T) {
	// Two samples 1 minute apart with 6000 token difference = 6000 TPM
	samples := []BurnSample{
		{Timestamp: time.Now().Add(-1 * time.Minute), TokensTotal: 0},
		{Timestamp: time.Now(), TokensTotal: 6000},
	}
	rate := computeBurnRate(samples)
	// Allow some tolerance for timing
	if rate < 5500 || rate > 6500 {
		t.Errorf("burn rate = %.0f, expected ~6000", rate)
	}
}
