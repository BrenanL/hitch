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
