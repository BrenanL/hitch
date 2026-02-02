package state

import (
	"testing"
	"time"
)

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
