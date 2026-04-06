package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BrenanL/hitch/pkg/sessions"
)

// TestAutopsyCommandRegistered verifies the command is set up with the right fields.
func TestAutopsyCommandRegistered(t *testing.T) {
	cmd := newAutopsyCmd()
	if cmd.Use != "autopsy <session-id>" {
		t.Errorf("Use = %q, want %q", cmd.Use, "autopsy <session-id>")
	}
	if cmd.Short == "" {
		t.Error("Short description should not be empty")
	}
	if cmd.Long == "" {
		t.Error("Long description should not be empty")
	}
	if cmd.RunE == nil {
		t.Error("RunE should be set")
	}
}

// TestAutopsyCommandRequiresOneArg verifies cobra arg validation.
func TestAutopsyCommandRequiresOneArg(t *testing.T) {
	cmd := newAutopsyCmd()
	if err := cmd.Args(cmd, []string{}); err == nil {
		t.Error("expected error for zero args")
	}
	if err := cmd.Args(cmd, []string{"a", "b"}); err == nil {
		t.Error("expected error for two args")
	}
	if err := cmd.Args(cmd, []string{"abc123"}); err != nil {
		t.Errorf("unexpected error for one arg: %v", err)
	}
}

// TestAutopsyUnknownSession verifies that findSessionByPrefix returns a clear error.
func TestAutopsyUnknownSession(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	if err := os.MkdirAll(filepath.Join(home, ".claude", "projects"), 0o755); err != nil {
		t.Fatalf("creating .claude/projects: %v", err)
	}

	_, err := findSessionByPrefix("nosuchsession123", "")
	if err == nil {
		t.Fatal("expected error for unknown session")
	}
	if !strings.Contains(err.Error(), "no session found") {
		t.Errorf("error = %q, want message containing 'no session found'", err.Error())
	}
}

// TestAutopsySessionParsing writes a minimal JSONL transcript, discovers the session,
// parses it, and asserts on the token economics.
func TestAutopsySessionParsing(t *testing.T) {
	home := t.TempDir()

	sessionID := "aabbccdd-0000-0000-0000-000000000001"
	projectEncoded := "-home-user-dev-testproject"
	sessDir := filepath.Join(home, ".claude", "projects", projectEncoded)
	if err := os.MkdirAll(sessDir, 0o755); err != nil {
		t.Fatalf("creating session dir: %v", err)
	}

	indexJSON := `{"projectPath": "/home/user/dev/testproject"}`
	if err := os.WriteFile(filepath.Join(sessDir, "sessions-index.json"), []byte(indexJSON), 0o644); err != nil {
		t.Fatalf("writing sessions-index.json: %v", err)
	}

	// Minimal JSONL with one assistant turn carrying token usage
	line := `{"type":"assistant","uuid":"u1","sessionId":"` + sessionID + `","timestamp":"2024-01-01T10:00:00Z","message":{"id":"msg1","role":"assistant","model":"claude-3-5-sonnet-20241022","stop_reason":"end_turn","usage":{"input_tokens":1000,"output_tokens":200,"cache_read_input_tokens":500,"cache_creation_input_tokens":100},"content":[{"type":"text","text":"Hello"}]}}`
	transcriptPath := filepath.Join(sessDir, sessionID+".jsonl")
	if err := os.WriteFile(transcriptPath, []byte(line+"\n"), 0o644); err != nil {
		t.Fatalf("writing transcript: %v", err)
	}

	t.Setenv("HOME", home)

	sum, err := findSessionByPrefix(sessionID[:8], "")
	if err != nil {
		t.Fatalf("findSessionByPrefix: %v", err)
	}
	if sum.ID != sessionID {
		t.Errorf("session ID = %q, want %q", sum.ID, sessionID)
	}

	// Parse with zero-cost estimator
	s, err := sessions.ParseSession(sum.TranscriptPath, nil, func(model string, in, out, cr, cc int) float64 {
		return 0
	})
	if err != nil {
		t.Fatalf("ParseSession: %v", err)
	}

	if s.TokenUsage.InputTokens != 1000 {
		t.Errorf("InputTokens = %d, want 1000", s.TokenUsage.InputTokens)
	}
	if s.TokenUsage.OutputTokens != 200 {
		t.Errorf("OutputTokens = %d, want 200", s.TokenUsage.OutputTokens)
	}
	if s.TokenUsage.CacheReadTokens != 500 {
		t.Errorf("CacheReadTokens = %d, want 500", s.TokenUsage.CacheReadTokens)
	}
	if s.TokenUsage.CacheCreationTokens != 100 {
		t.Errorf("CacheCreationTokens = %d, want 100", s.TokenUsage.CacheCreationTokens)
	}
	if s.Model != "claude-3-5-sonnet-20241022" {
		t.Errorf("Model = %q, want %q", s.Model, "claude-3-5-sonnet-20241022")
	}
}

// TestAutopsyFormatHelpers verifies the display helpers used by autopsy output.
func TestAutopsyFormatHelpers(t *testing.T) {
	cases := []struct {
		n    int
		want string
	}{
		{0, "0"},
		{999, "999"},
		{1000, "1K"},
		{1500, "2K"},
		{1_000_000, "1.0M"},
	}
	for _, c := range cases {
		got := formatTokens(c.n)
		if got != c.want {
			t.Errorf("formatTokens(%d) = %q, want %q", c.n, got, c.want)
		}
	}
}
