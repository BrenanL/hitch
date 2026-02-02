package state

import (
	"testing"
)

func TestOpenInMemory(t *testing.T) {
	db, err := OpenInMemory()
	if err != nil {
		t.Fatalf("OpenInMemory: %v", err)
	}
	defer db.Close()

	if v := db.SchemaVersion(); v != 1 {
		t.Errorf("schema version = %d, want 1", v)
	}
}

func TestMigrateIdempotent(t *testing.T) {
	db, err := OpenInMemory()
	if err != nil {
		t.Fatalf("OpenInMemory: %v", err)
	}
	defer db.Close()

	// Running migrate again should be a no-op.
	if err := db.migrate(); err != nil {
		t.Fatalf("second migrate: %v", err)
	}

	if v := db.SchemaVersion(); v != 1 {
		t.Errorf("schema version = %d, want 1", v)
	}
}

func TestTablesExist(t *testing.T) {
	db, err := OpenInMemory()
	if err != nil {
		t.Fatalf("OpenInMemory: %v", err)
	}
	defer db.Close()

	tables := []string{"channels", "rules", "events", "sessions", "kv_state", "mute", "schema_version"}
	for _, table := range tables {
		var name string
		err := db.db.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name=?`, table).Scan(&name)
		if err != nil {
			t.Errorf("table %q not found: %v", table, err)
		}
	}
}
