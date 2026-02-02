package state

import (
	"testing"
	"time"
)

func TestKVGetSet(t *testing.T) {
	db, err := OpenInMemory()
	if err != nil {
		t.Fatalf("OpenInMemory: %v", err)
	}
	defer db.Close()

	// Set global key
	if err := db.KVSet("greeting", "hello", "", ""); err != nil {
		t.Fatalf("KVSet: %v", err)
	}

	// Get global key
	val, err := db.KVGet("greeting", "")
	if err != nil {
		t.Fatalf("KVGet: %v", err)
	}
	if val != "hello" {
		t.Errorf("KVGet = %q, want %q", val, "hello")
	}

	// Set session-scoped key
	if err := db.KVSet("counter", "42", "sess1", ""); err != nil {
		t.Fatalf("KVSet session: %v", err)
	}

	// Get session-scoped key
	val, err = db.KVGet("counter", "sess1")
	if err != nil {
		t.Fatalf("KVGet session: %v", err)
	}
	if val != "42" {
		t.Errorf("KVGet session = %q, want %q", val, "42")
	}

	// Different session returns empty
	val, err = db.KVGet("counter", "sess2")
	if err != nil {
		t.Fatalf("KVGet other session: %v", err)
	}
	if val != "" {
		t.Errorf("KVGet other session = %q, want empty", val)
	}

	// Get nonexistent key
	val, err = db.KVGet("nonexistent", "")
	if err != nil {
		t.Fatalf("KVGet nonexistent: %v", err)
	}
	if val != "" {
		t.Errorf("KVGet nonexistent = %q, want empty", val)
	}

	// Delete
	if err := db.KVDelete("greeting", ""); err != nil {
		t.Fatalf("KVDelete: %v", err)
	}
	val, _ = db.KVGet("greeting", "")
	if val != "" {
		t.Errorf("after delete: got %q, want empty", val)
	}
}

func TestKVExpiry(t *testing.T) {
	db, err := OpenInMemory()
	if err != nil {
		t.Fatalf("OpenInMemory: %v", err)
	}
	defer db.Close()

	// Set a key that's already expired
	past := time.Now().UTC().Add(-1 * time.Hour).Format(time.RFC3339)
	if err := db.KVSet("expired", "value", "", past); err != nil {
		t.Fatalf("KVSet expired: %v", err)
	}

	// Should not be returned
	val, err := db.KVGet("expired", "")
	if err != nil {
		t.Fatalf("KVGet expired: %v", err)
	}
	if val != "" {
		t.Errorf("expired key should return empty, got %q", val)
	}

	// Set a key that expires in the future
	future := time.Now().UTC().Add(1 * time.Hour).Format(time.RFC3339)
	if err := db.KVSet("valid", "value", "", future); err != nil {
		t.Fatalf("KVSet future: %v", err)
	}
	val, _ = db.KVGet("valid", "")
	if val != "value" {
		t.Errorf("future key = %q, want %q", val, "value")
	}

	// Cleanup expired
	n, err := db.KVCleanupExpired()
	if err != nil {
		t.Fatalf("KVCleanupExpired: %v", err)
	}
	if n != 1 {
		t.Errorf("KVCleanupExpired removed %d, want 1", n)
	}
}
