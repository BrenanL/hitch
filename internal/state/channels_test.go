package state

import (
	"testing"
)

func TestChannelCRUD(t *testing.T) {
	db, err := OpenInMemory()
	if err != nil {
		t.Fatalf("OpenInMemory: %v", err)
	}
	defer db.Close()

	// Add
	ch := Channel{
		ID:      "ntfy",
		Adapter: "ntfy",
		Name:    "my-alerts",
		Config:  `{"topic":"my-alerts"}`,
		Enabled: true,
	}
	if err := db.ChannelAdd(ch); err != nil {
		t.Fatalf("ChannelAdd: %v", err)
	}

	// List
	channels, err := db.ChannelList()
	if err != nil {
		t.Fatalf("ChannelList: %v", err)
	}
	if len(channels) != 1 {
		t.Fatalf("ChannelList: got %d, want 1", len(channels))
	}
	if channels[0].ID != "ntfy" {
		t.Errorf("ID = %q, want %q", channels[0].ID, "ntfy")
	}
	if channels[0].Adapter != "ntfy" {
		t.Errorf("Adapter = %q, want %q", channels[0].Adapter, "ntfy")
	}

	// Get
	got, err := db.ChannelGet("ntfy")
	if err != nil {
		t.Fatalf("ChannelGet: %v", err)
	}
	if got == nil {
		t.Fatal("ChannelGet: got nil")
	}
	if got.Name != "my-alerts" {
		t.Errorf("Name = %q, want %q", got.Name, "my-alerts")
	}

	// Get not found
	got, err = db.ChannelGet("nonexistent")
	if err != nil {
		t.Fatalf("ChannelGet not found: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil for nonexistent channel")
	}

	// Update last used
	if err := db.ChannelUpdateLastUsed("ntfy"); err != nil {
		t.Fatalf("ChannelUpdateLastUsed: %v", err)
	}
	got, _ = db.ChannelGet("ntfy")
	if got.LastUsedAt == "" {
		t.Error("LastUsedAt should be set after update")
	}

	// Remove
	if err := db.ChannelRemove("ntfy"); err != nil {
		t.Fatalf("ChannelRemove: %v", err)
	}
	channels, _ = db.ChannelList()
	if len(channels) != 0 {
		t.Errorf("after remove: got %d channels, want 0", len(channels))
	}

	// Remove not found
	if err := db.ChannelRemove("ntfy"); err == nil {
		t.Error("ChannelRemove of nonexistent should return error")
	}
}

func TestChannelDuplicateID(t *testing.T) {
	db, err := OpenInMemory()
	if err != nil {
		t.Fatalf("OpenInMemory: %v", err)
	}
	defer db.Close()

	ch := Channel{ID: "dup", Adapter: "ntfy", Name: "test", Config: "{}", Enabled: true}
	if err := db.ChannelAdd(ch); err != nil {
		t.Fatalf("first add: %v", err)
	}

	ch2 := Channel{ID: "dup", Adapter: "discord", Name: "test2", Config: "{}", Enabled: true}
	if err := db.ChannelAdd(ch2); err == nil {
		t.Error("expected error when adding duplicate channel ID")
	}
}

func TestChannelListEmpty(t *testing.T) {
	db, err := OpenInMemory()
	if err != nil {
		t.Fatalf("OpenInMemory: %v", err)
	}
	defer db.Close()

	channels, err := db.ChannelList()
	if err != nil {
		t.Fatalf("ChannelList: %v", err)
	}
	if channels != nil && len(channels) != 0 {
		t.Errorf("empty DB should return empty list, got %d", len(channels))
	}
}

func TestChannelConfigPreserved(t *testing.T) {
	db, err := OpenInMemory()
	if err != nil {
		t.Fatalf("OpenInMemory: %v", err)
	}
	defer db.Close()

	config := `{"topic":"my-topic","server":"https://ntfy.example.com"}`
	ch := Channel{ID: "cfg", Adapter: "ntfy", Name: "cfg-test", Config: config, Enabled: true}
	if err := db.ChannelAdd(ch); err != nil {
		t.Fatalf("ChannelAdd: %v", err)
	}

	got, _ := db.ChannelGet("cfg")
	if got.Config != config {
		t.Errorf("Config not round-tripped:\ngot:  %s\nwant: %s", got.Config, config)
	}
}
