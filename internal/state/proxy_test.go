package state

import (
	"testing"
	"time"
)

func TestQueryRequestsNear(t *testing.T) {
	db, err := OpenInMemory()
	if err != nil {
		t.Fatalf("OpenInMemory: %v", err)
	}
	defer db.Close()

	now := time.Now().UTC()
	// Insert rows with explicit timestamps via raw SQL since InsertAPIRequest uses DB default.
	insertWithTime := func(sessionID string, ts time.Time) {
		t.Helper()
		_, err := db.db.Exec(
			`INSERT INTO api_requests (session_id, timestamp, http_status) VALUES (?, ?, 200)`,
			sessionID, ts.Format("2006-01-02T15:04:05"),
		)
		if err != nil {
			t.Fatalf("insert: %v", err)
		}
	}

	anchor := now
	insertWithTime("sess1", anchor.Add(-60*time.Second)) // outside ±30s
	insertWithTime("sess1", anchor.Add(-20*time.Second)) // inside ±30s
	insertWithTime("sess1", anchor.Add(+20*time.Second)) // inside ±30s
	insertWithTime("sess1", anchor.Add(+60*time.Second)) // outside ±30s
	insertWithTime("sess2", anchor.Add(-15*time.Second)) // inside ±30s

	anchorStr := anchor.Format("2006-01-02T15:04:05")

	// Query sess1 within ±30s of anchor: should match the -20s and +20s entries.
	got, err := db.QueryRequestsNear(anchorStr, 30, "sess1")
	if err != nil {
		t.Fatalf("QueryRequestsNear: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("QueryRequestsNear (sess1 ±30s): got %d, want 2", len(got))
	}

	// Query without session filter: should include sess1(-20s), sess1(+20s), and sess2(-15s).
	gotAll, err := db.QueryRequestsNear(anchorStr, 30, "")
	if err != nil {
		t.Fatalf("QueryRequestsNear (no session): %v", err)
	}
	if len(gotAll) != 3 {
		t.Errorf("QueryRequestsNear (all sessions ±30s): got %d, want 3", len(gotAll))
	}

	// Query sess1 within ±5s of anchor: no entries in that narrow window.
	gotNone, err := db.QueryRequestsNear(anchorStr, 5, "sess1")
	if err != nil {
		t.Fatalf("QueryRequestsNear (±5s): %v", err)
	}
	if len(gotNone) != 0 {
		t.Errorf("QueryRequestsNear (±5s): got %d, want 0", len(gotNone))
	}
}

func TestQueryRequestsNearFieldValues(t *testing.T) {
	db, err := OpenInMemory()
	if err != nil {
		t.Fatalf("OpenInMemory: %v", err)
	}
	defer db.Close()

	now := time.Now().UTC()
	ts := now.Format("2006-01-02T15:04:05")

	// Insert a row and verify returned fields match what was inserted.
	_, err = db.db.Exec(
		`INSERT INTO api_requests (session_id, timestamp, http_status, model, input_tokens, output_tokens) VALUES (?, ?, 200, ?, ?, ?)`,
		"sess-field-check", ts, "claude-opus-4-6", 1234, 567,
	)
	if err != nil {
		t.Fatalf("insert: %v", err)
	}

	got, err := db.QueryRequestsNear(ts, 5, "sess-field-check")
	if err != nil {
		t.Fatalf("QueryRequestsNear: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("QueryRequestsNear: got %d rows, want 1", len(got))
	}

	r := got[0]
	if r.SessionID != "sess-field-check" {
		t.Errorf("SessionID = %q, want sess-field-check", r.SessionID)
	}
	if r.HTTPStatus != 200 {
		t.Errorf("HTTPStatus = %d, want 200", r.HTTPStatus)
	}
	if r.Model != "claude-opus-4-6" {
		t.Errorf("Model = %q, want claude-opus-4-6", r.Model)
	}
	if r.InputTokens != 1234 {
		t.Errorf("InputTokens = %d, want 1234", r.InputTokens)
	}
	if r.OutputTokens != 567 {
		t.Errorf("OutputTokens = %d, want 567", r.OutputTokens)
	}
}

// TestProxyGapsDetection verifies the core detection primitives used by
// runProxyGaps: a StopFailure event with no matching proxy request within ±30s
// should be identifiable via QueryStopFailureEvents + QueryRequestsNear.
func TestProxyGapsDetection(t *testing.T) {
	db, err := OpenInMemory()
	if err != nil {
		t.Fatalf("OpenInMemory: %v", err)
	}
	defer db.Close()

	now := time.Now().UTC()

	// One StopFailure event at T=0 for sessA — no proxy request nearby.
	unmatchedEvTime := now.Add(-10 * time.Minute)
	if err := db.EventLog(Event{
		SessionID: "sessA",
		HookEvent: "StopFailure",
		Timestamp: unmatchedEvTime.Format(time.RFC3339),
	}); err != nil {
		t.Fatalf("EventLog (unmatched): %v", err)
	}

	// One StopFailure event for sessB — has a matching proxy request within ±30s.
	matchedEvTime := now.Add(-5 * time.Minute)
	if err := db.EventLog(Event{
		SessionID: "sessB",
		HookEvent: "StopFailure",
		Timestamp: matchedEvTime.Format(time.RFC3339),
	}); err != nil {
		t.Fatalf("EventLog (matched): %v", err)
	}

	// Insert proxy request for sessB at the same time.
	matchedTs := matchedEvTime.Format("2006-01-02T15:04:05")
	_, err = db.db.Exec(
		`INSERT INTO api_requests (session_id, timestamp, http_status) VALUES (?, ?, 200)`,
		"sessB", matchedTs,
	)
	if err != nil {
		t.Fatalf("insert proxy request: %v", err)
	}

	since := now.Add(-30 * time.Minute).Format(time.RFC3339)
	events, err := db.QueryStopFailureEvents(since, "")
	if err != nil {
		t.Fatalf("QueryStopFailureEvents: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("got %d StopFailure events, want 2", len(events))
	}

	// Simulate the gaps detection: for each event, look for nearby proxy requests.
	var matched, unmatched int
	for _, ev := range events {
		nearby, err := db.QueryRequestsNear(ev.Timestamp, 30, ev.SessionID)
		if err != nil {
			t.Fatalf("QueryRequestsNear for session %s: %v", ev.SessionID, err)
		}
		if len(nearby) > 0 {
			matched++
		} else {
			unmatched++
		}
	}

	if matched != 1 {
		t.Errorf("matched = %d, want 1 (sessB has a proxy entry)", matched)
	}
	if unmatched != 1 {
		t.Errorf("unmatched = %d, want 1 (sessA has no proxy entry)", unmatched)
	}
}
