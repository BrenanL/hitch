package state

import (
	"testing"
	"time"
)

func TestQueryStopFailureEvents(t *testing.T) {
	db, err := OpenInMemory()
	if err != nil {
		t.Fatalf("OpenInMemory: %v", err)
	}
	defer db.Close()

	now := time.Now().UTC()

	events := []Event{
		{SessionID: "sess1", HookEvent: "StopFailure", Timestamp: now.Add(-10 * time.Minute).Format(time.RFC3339)},
		{SessionID: "sess2", HookEvent: "StopFailure", Timestamp: now.Add(-5 * time.Minute).Format(time.RFC3339)},
		{SessionID: "sess3", HookEvent: "Stop", Timestamp: now.Add(-3 * time.Minute).Format(time.RFC3339)},
		{SessionID: "sess1", HookEvent: "StopFailure", Timestamp: now.Add(-1 * time.Minute).Format(time.RFC3339)},
	}
	for _, e := range events {
		if err := db.EventLog(e); err != nil {
			t.Fatalf("EventLog: %v", err)
		}
	}

	// Query all StopFailure events — should get 3, not the Stop event.
	since := now.Add(-30 * time.Minute).Format(time.RFC3339)
	got, err := db.QueryStopFailureEvents(since, "")
	if err != nil {
		t.Fatalf("QueryStopFailureEvents: %v", err)
	}
	if len(got) != 3 {
		t.Errorf("QueryStopFailureEvents: got %d events, want 3", len(got))
	}
	for _, e := range got {
		if e.HookEvent != "StopFailure" {
			t.Errorf("got non-StopFailure event: %q", e.HookEvent)
		}
	}

	// Query with a narrow since window — only the most recent StopFailure.
	narrowSince := now.Add(-3 * time.Minute).Format(time.RFC3339)
	narrow, err := db.QueryStopFailureEvents(narrowSince, "")
	if err != nil {
		t.Fatalf("QueryStopFailureEvents (narrow): %v", err)
	}
	if len(narrow) != 1 {
		t.Errorf("QueryStopFailureEvents (narrow): got %d events, want 1", len(narrow))
	}
}

func TestQueryStopFailureEventsWithUntil(t *testing.T) {
	db, err := OpenInMemory()
	if err != nil {
		t.Fatalf("OpenInMemory: %v", err)
	}
	defer db.Close()

	now := time.Now().UTC()

	events := []Event{
		{SessionID: "s1", HookEvent: "StopFailure", Timestamp: now.Add(-20 * time.Minute).Format(time.RFC3339)},
		{SessionID: "s2", HookEvent: "StopFailure", Timestamp: now.Add(-10 * time.Minute).Format(time.RFC3339)},
		{SessionID: "s3", HookEvent: "StopFailure", Timestamp: now.Add(-2 * time.Minute).Format(time.RFC3339)},
	}
	for _, e := range events {
		if err := db.EventLog(e); err != nil {
			t.Fatalf("EventLog: %v", err)
		}
	}

	since := now.Add(-30 * time.Minute).Format(time.RFC3339)
	// until set to -5min: should exclude the -2min entry
	until := now.Add(-5 * time.Minute).Format(time.RFC3339)

	got, err := db.QueryStopFailureEvents(since, until)
	if err != nil {
		t.Fatalf("QueryStopFailureEvents with until: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("got %d events, want 2 (until bound should exclude -2min entry)", len(got))
	}
	for _, e := range got {
		if e.HookEvent != "StopFailure" {
			t.Errorf("got non-StopFailure event: %q", e.HookEvent)
		}
	}
}

func TestEventLogAndQuery(t *testing.T) {
	db, err := OpenInMemory()
	if err != nil {
		t.Fatalf("OpenInMemory: %v", err)
	}
	defer db.Close()

	now := time.Now().UTC()

	// Log events
	events := []Event{
		{SessionID: "sess1", HookEvent: "Stop", RuleID: "r1", ActionTaken: "notified:discord", DurationMs: 50, Timestamp: now.Add(-2 * time.Minute).Format(time.RFC3339)},
		{SessionID: "sess1", HookEvent: "PreToolUse", RuleID: "r2", ToolName: "Bash", ActionTaken: "denied", DurationMs: 5, Timestamp: now.Add(-1 * time.Minute).Format(time.RFC3339)},
		{SessionID: "sess2", HookEvent: "Stop", RuleID: "r1", ActionTaken: "notified:discord", DurationMs: 45, Timestamp: now.Format(time.RFC3339)},
	}
	for _, e := range events {
		if err := db.EventLog(e); err != nil {
			t.Fatalf("EventLog: %v", err)
		}
	}

	// Query all
	all, err := db.EventQuery(EventFilter{})
	if err != nil {
		t.Fatalf("EventQuery all: %v", err)
	}
	if len(all) != 3 {
		t.Errorf("EventQuery all: got %d, want 3", len(all))
	}

	// Query by session
	bySession, err := db.EventQuery(EventFilter{SessionID: "sess1"})
	if err != nil {
		t.Fatalf("EventQuery by session: %v", err)
	}
	if len(bySession) != 2 {
		t.Errorf("EventQuery by session: got %d, want 2", len(bySession))
	}

	// Query by event type
	byEvent, err := db.EventQuery(EventFilter{HookEvent: "Stop"})
	if err != nil {
		t.Fatalf("EventQuery by event: %v", err)
	}
	if len(byEvent) != 2 {
		t.Errorf("EventQuery by event: got %d, want 2", len(byEvent))
	}

	// Query by time
	since := now.Add(-90 * time.Second).Format(time.RFC3339)
	bySince, err := db.EventQuery(EventFilter{Since: since})
	if err != nil {
		t.Fatalf("EventQuery by since: %v", err)
	}
	if len(bySince) != 2 {
		t.Errorf("EventQuery by since: got %d, want 2", len(bySince))
	}

	// Query with limit
	limited, err := db.EventQuery(EventFilter{Limit: 1})
	if err != nil {
		t.Fatalf("EventQuery with limit: %v", err)
	}
	if len(limited) != 1 {
		t.Errorf("EventQuery with limit: got %d, want 1", len(limited))
	}
}
