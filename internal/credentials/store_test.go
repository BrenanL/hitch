package credentials

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStoreSetGet(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "creds.enc")
	store := NewStore(path)
	store.SetPassphrase("test-pass-123")

	// Set
	if err := store.Set("ntfy.topic", "my-alerts"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if err := store.Set("discord.webhook_url", "https://discord.com/api/webhooks/123/abc"); err != nil {
		t.Fatalf("Set: %v", err)
	}

	// Verify file was created
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("credentials file not created: %v", err)
	}

	// Get
	val, err := store.Get("ntfy.topic")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if val != "my-alerts" {
		t.Errorf("Get = %q, want %q", val, "my-alerts")
	}

	// Get from fresh store (forces reload from disk)
	store2 := NewStore(path)
	store2.SetPassphrase("test-pass-123")
	val, err = store2.Get("ntfy.topic")
	if err != nil {
		t.Fatalf("Get from fresh store: %v", err)
	}
	if val != "my-alerts" {
		t.Errorf("Get from fresh store = %q, want %q", val, "my-alerts")
	}
}

func TestStoreDelete(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(filepath.Join(dir, "creds.enc"))
	store.SetPassphrase("test-pass")

	store.Set("key1", "val1")
	store.Set("key2", "val2")

	if err := store.Delete("key1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	val, _ := store.Get("key1")
	if val != "" {
		t.Errorf("deleted key should return empty, got %q", val)
	}

	val, _ = store.Get("key2")
	if val != "val2" {
		t.Errorf("other key = %q, want %q", val, "val2")
	}
}

func TestStoreList(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(filepath.Join(dir, "creds.enc"))
	store.SetPassphrase("test-pass")

	store.Set("a", "1")
	store.Set("b", "2")

	keys, err := store.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(keys) != 2 {
		t.Errorf("List: got %d keys, want 2", len(keys))
	}
}

func TestStoreWrongPassphrase(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "creds.enc")
	store := NewStore(path)
	store.SetPassphrase("correct-pass")
	store.Set("key", "val")

	// Try with wrong passphrase
	store2 := NewStore(path)
	store2.SetPassphrase("wrong-pass")
	_, err := store2.Get("key")
	if err == nil {
		t.Error("expected error with wrong passphrase")
	}
}

func TestStoreNoPassphrase(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(filepath.Join(dir, "creds.enc"))

	if err := store.Set("key", "val"); err == nil {
		t.Error("expected error without passphrase")
	}
}

func TestEnvFallback(t *testing.T) {
	t.Setenv("HT_NTFY_TOPIC", "env-topic")

	dir := t.TempDir()
	store := NewStore(filepath.Join(dir, "creds.enc"))

	// Should get from env even without passphrase
	val, err := store.Get("ntfy.topic")
	if err != nil {
		t.Fatalf("Get with env fallback: %v", err)
	}
	if val != "env-topic" {
		t.Errorf("Get = %q, want %q", val, "env-topic")
	}
}

func TestKeyToEnv(t *testing.T) {
	tests := []struct {
		key  string
		want string
	}{
		{"ntfy.topic", "HT_NTFY_TOPIC"},
		{"discord.webhook_url", "HT_DISCORD_WEBHOOK_URL"},
		{"slack.webhook_url", "HT_SLACK_WEBHOOK_URL"},
	}
	for _, tt := range tests {
		got := keyToEnv(tt.key)
		if got != tt.want {
			t.Errorf("keyToEnv(%q) = %q, want %q", tt.key, got, tt.want)
		}
	}
}

func TestStoreEmptyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "creds.enc")
	store := NewStore(path)
	store.SetPassphrase("pass")

	// Get from nonexistent file should return empty
	val, err := store.Get("nonexistent")
	if err != nil {
		t.Fatalf("Get from nonexistent: %v", err)
	}
	if val != "" {
		t.Errorf("expected empty, got %q", val)
	}
}
