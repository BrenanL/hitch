package state

import (
	"testing"
	"time"
)

func TestMute(t *testing.T) {
	db, err := OpenInMemory()
	if err != nil {
		t.Fatalf("OpenInMemory: %v", err)
	}
	defer db.Close()

	// Initially not muted
	muted, err := db.IsMuted()
	if err != nil {
		t.Fatalf("IsMuted: %v", err)
	}
	if muted {
		t.Error("should not be muted initially")
	}

	// Mute until future
	future := time.Now().UTC().Add(1 * time.Hour).Format(time.RFC3339)
	if err := db.MuteSet(future); err != nil {
		t.Fatalf("MuteSet: %v", err)
	}

	muted, err = db.IsMuted()
	if err != nil {
		t.Fatalf("IsMuted after set: %v", err)
	}
	if !muted {
		t.Error("should be muted after MuteSet with future time")
	}

	// Get returns the timestamp
	until, err := db.MuteGet()
	if err != nil {
		t.Fatalf("MuteGet: %v", err)
	}
	if until != future {
		t.Errorf("MuteGet = %q, want %q", until, future)
	}

	// Clear
	if err := db.MuteClear(); err != nil {
		t.Fatalf("MuteClear: %v", err)
	}
	muted, _ = db.IsMuted()
	if muted {
		t.Error("should not be muted after clear")
	}

	// Mute with past time = not muted
	past := time.Now().UTC().Add(-1 * time.Hour).Format(time.RFC3339)
	db.MuteSet(past)
	muted, _ = db.IsMuted()
	if muted {
		t.Error("should not be muted with past time")
	}
}
