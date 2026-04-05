package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestFormatTokens verifies SI-suffix formatting.
func TestFormatTokens(t *testing.T) {
	cases := []struct {
		n    int
		want string
	}{
		{0, "0"},
		{999, "999"},
		{1000, "1K"},
		{1500, "2K"},
		{1_000_000, "1.0M"},
		{1_250_000, "1.2M"},
	}
	for _, c := range cases {
		got := formatTokens(c.n)
		if got != c.want {
			t.Errorf("formatTokens(%d) = %q, want %q", c.n, got, c.want)
		}
	}
}

// TestFormatSessionDuration verifies minute/hour formatting.
func TestFormatSessionDuration(t *testing.T) {
	cases := []struct {
		d    time.Duration
		want string
	}{
		{0, "0m"},
		{5 * time.Minute, "5m"},
		{90 * time.Minute, "1h 30m"},
		{-time.Minute, "0m"},
	}
	for _, c := range cases {
		got := formatSessionDuration(c.d)
		if got != c.want {
			t.Errorf("formatSessionDuration(%v) = %q, want %q", c.d, got, c.want)
		}
	}
}

// TestShortModelName verifies model name shortening.
func TestShortModelName(t *testing.T) {
	cases := []struct {
		model string
		want  string
	}{
		{"claude-opus-4-6", "Opus"},
		{"claude-sonnet-4-6", "Sonnet"},
		{"claude-haiku-3-5", "Haiku"},
		{"claude-SONNET-4", "Sonnet"},
		{"", "unknown"},
		{"custom-model", "custom-model"},
	}
	for _, c := range cases {
		got := shortModelName(c.model)
		if got != c.want {
			t.Errorf("shortModelName(%q) = %q, want %q", c.model, got, c.want)
		}
	}
}

// TestShortenHome verifies home directory replacement.
func TestShortenHome(t *testing.T) {
	home, _ := os.UserHomeDir()
	path := filepath.Join(home, "dev", "myproject")
	got := shortenHome(path)
	if !strings.HasPrefix(got, "~/") {
		t.Errorf("shortenHome(%q) = %q, expected ~/... prefix", path, got)
	}
	// A path without the home prefix is returned unchanged.
	other := "/tmp/something"
	if shortenHome(other) != other {
		t.Errorf("shortenHome(%q) should be unchanged", other)
	}
}

// TestTruncate verifies string truncation.
func TestTruncate(t *testing.T) {
	cases := []struct {
		s    string
		n    int
		want string
	}{
		{"hello", 10, "hello"},
		{"hello world", 5, "hello"},
		{"", 5, ""},
	}
	for _, c := range cases {
		got := truncate(c.s, c.n)
		if got != c.want {
			t.Errorf("truncate(%q, %d) = %q, want %q", c.s, c.n, got, c.want)
		}
	}
}

// TestFindSessionByPrefixNotFound verifies the error path when no session matches.
func TestFindSessionByPrefixNotFound(t *testing.T) {
	tmpHome := t.TempDir()
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", oldHome)

	// Create the projects directory so DiscoverProjects succeeds but finds nothing.
	if err := os.MkdirAll(filepath.Join(tmpHome, ".claude", "projects"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	_, err := findSessionByPrefix("aabbccdd", "")
	if err == nil {
		t.Fatal("expected error for no-match prefix, got nil")
	}
	if !strings.Contains(err.Error(), "no session found") {
		t.Errorf("error = %q; want 'no session found'", err.Error())
	}
}

// TestFindSessionByPrefixAmbiguous verifies the error when multiple sessions match.
func TestFindSessionByPrefixAmbiguous(t *testing.T) {
	tmpHome := t.TempDir()
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", oldHome)

	// Create a fake project directory with two JSONL files sharing a prefix.
	projDir := filepath.Join(tmpHome, ".claude", "projects", "-home-user-dev-test")
	if err := os.MkdirAll(projDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	// Write minimal JSONL files with UUIDs that share the prefix "aabbccdd".
	uuid1 := "aabbccdd-0000-0000-0000-000000000001"
	uuid2 := "aabbccdd-0000-0000-0000-000000000002"
	line := `{"type":"user","uuid":"u1","sessionId":"` + uuid1 + `","timestamp":"2026-04-04T10:00:00.000Z","message":{"role":"user","content":"hi"}}` + "\n"
	for _, id := range []string{uuid1, uuid2} {
		path := filepath.Join(projDir, id+".jsonl")
		content := `{"type":"user","uuid":"u","sessionId":"` + id + `","timestamp":"2026-04-04T10:00:00.000Z","message":{"role":"user","content":"hi"}}` + "\n"
		_ = line
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}

	_, err := findSessionByPrefix("aabbccdd", "")
	if err == nil {
		t.Fatal("expected ambiguous error, got nil")
	}
	if !strings.Contains(err.Error(), "ambiguous") {
		t.Errorf("error = %q; want 'ambiguous'", err.Error())
	}
}

// TestFindSessionByPrefixExact verifies a unique prefix resolves to one session.
func TestFindSessionByPrefixExact(t *testing.T) {
	tmpHome := t.TempDir()
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", oldHome)

	projDir := filepath.Join(tmpHome, ".claude", "projects", "-home-user-dev-test")
	if err := os.MkdirAll(projDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	uuid := "11223344-5566-7788-99aa-bbccddeeff00"
	content := `{"type":"user","uuid":"u","sessionId":"` + uuid + `","timestamp":"2026-04-04T10:00:00.000Z","message":{"role":"user","content":"hi"}}` + "\n"
	path := filepath.Join(projDir, uuid+".jsonl")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	sum, err := findSessionByPrefix("11223344", "")
	if err != nil {
		t.Fatalf("findSessionByPrefix: %v", err)
	}
	if sum.ID != uuid {
		t.Errorf("ID = %q, want %q", sum.ID, uuid)
	}
}
