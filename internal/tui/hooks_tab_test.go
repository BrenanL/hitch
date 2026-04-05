package tui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// writeSettingsJSON writes a settings.json file with the given hooks structure.
func writeSettingsJSON(t *testing.T, path string, hooks map[string]any) {
	t.Helper()
	data, err := json.MarshalIndent(map[string]any{"hooks": hooks}, "", "  ")
	if err != nil {
		t.Fatalf("marshal settings: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write settings: %v", err)
	}
}

func TestHooksTabLoadsWithoutError(t *testing.T) {
	// Create a temp dir as a fake project with no settings files.
	dir := t.TempDir()
	tab := NewHooksTab(dir)
	if tab == nil {
		t.Fatal("NewHooksTab returned nil")
	}
	// View should not panic.
	view := tab.View()
	if view == "" {
		t.Error("View() returned empty string")
	}
}

func TestHooksTabWithHooks(t *testing.T) {
	dir := t.TempDir()

	// Write a user-scope settings.json using the home-relative path isn't practical in tests,
	// so we write to the project .claude/settings.json (project scope).
	projectSettings := filepath.Join(dir, ".claude", "settings.json")
	writeSettingsJSON(t, projectSettings, map[string]any{
		"PreToolUse": []map[string]any{
			{
				"matcher": "Bash",
				"hooks": []map[string]any{
					{"type": "command", "command": "/usr/local/bin/audit-hook.sh"},
				},
			},
		},
	})

	tab := NewHooksTab(dir)
	view := tab.View()

	// The view should contain the event name.
	if !strings.Contains(view, "PreToolUse") {
		t.Errorf("View() missing PreToolUse event, got:\n%s", view)
	}

	// The command should appear.
	if !strings.Contains(view, "audit-hook.sh") {
		t.Errorf("View() missing command, got:\n%s", view)
	}
}

func TestHooksTabTitle(t *testing.T) {
	tab := NewHooksTab(t.TempDir())
	if tab.Title() != "Hooks" {
		t.Errorf("Title() = %q, want %q", tab.Title(), "Hooks")
	}
}

func TestHooksTabFilter(t *testing.T) {
	dir := t.TempDir()
	projectSettings := filepath.Join(dir, ".claude", "settings.json")
	writeSettingsJSON(t, projectSettings, map[string]any{
		"PreToolUse": []map[string]any{
			{
				"matcher": "Bash",
				"hooks": []map[string]any{
					{"type": "command", "command": "/usr/bin/guard.sh"},
				},
			},
		},
		"PostToolUse": []map[string]any{
			{
				"matcher": "",
				"hooks": []map[string]any{
					{"type": "command", "command": "/usr/bin/log.sh"},
				},
			},
		},
	})

	tab := NewHooksTab(dir)

	// Apply filter for "pre".
	tab.filter = "pre"
	tab.applyFilter()

	view := tab.View()
	if !strings.Contains(view, "PreToolUse") {
		t.Errorf("filtered view should contain PreToolUse, got:\n%s", view)
	}
	if strings.Contains(view, "PostToolUse") {
		t.Errorf("filtered view should not contain PostToolUse, got:\n%s", view)
	}
}

func TestHooksTabEmptyNoProjectSettings(t *testing.T) {
	// When no project-scope or local-scope settings exist and there are no hooks
	// in any settings file, all rows from project/local scopes should be empty.
	// We can't easily mock the user-scope path (~/.claude/settings.json), so we
	// verify only that NewHooksTab returns a valid tab without panicking, and
	// that the view renders without error.
	dir := t.TempDir()
	tab := NewHooksTab(dir)
	view := tab.View()
	if view == "" {
		t.Error("View() returned empty string")
	}
}

func TestHooksTabScrolling(t *testing.T) {
	dir := t.TempDir()
	tab := NewHooksTab(dir)

	// Navigation on empty list should not panic.
	updated, _ := tab.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	if updated == nil {
		t.Error("Update returned nil")
	}
	updated, _ = tab.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	if updated == nil {
		t.Error("Update returned nil")
	}
}

func TestSortedKeys(t *testing.T) {
	input := []string{"zebra", "apple", "mango", "apple"}
	result := sortedKeys(input)
	if len(result) != 3 {
		t.Errorf("sortedKeys: expected 3 unique entries, got %d", len(result))
	}
	if result[0] != "apple" || result[1] != "mango" || result[2] != "zebra" {
		t.Errorf("sortedKeys: wrong order: %v", result)
	}
}

func TestItoa(t *testing.T) {
	tests := []struct {
		n    int
		want string
	}{
		{0, "0"},
		{1, "1"},
		{42, "42"},
		{-5, "-5"},
		{1000, "1000"},
	}
	for _, tc := range tests {
		got := itoa(tc.n)
		if got != tc.want {
			t.Errorf("itoa(%d) = %q, want %q", tc.n, got, tc.want)
		}
	}
}

func TestStripAnsi(t *testing.T) {
	// Plain string passes through unchanged.
	plain := "hello world"
	if got := stripAnsi(plain); got != plain {
		t.Errorf("stripAnsi(%q) = %q, want %q", plain, got, plain)
	}

	// String with ANSI codes should have codes removed.
	withAnsi := "\x1b[32mgreen text\x1b[0m"
	want := "green text"
	if got := stripAnsi(withAnsi); got != want {
		t.Errorf("stripAnsi(%q) = %q, want %q", withAnsi, got, want)
	}

	// Empty string.
	if got := stripAnsi(""); got != "" {
		t.Errorf("stripAnsi('') = %q, want ''", got)
	}
}

func TestHooksTabSingleHandlerLabel(t *testing.T) {
	dir := t.TempDir()
	projectSettings := filepath.Join(dir, ".claude", "settings.json")
	writeSettingsJSON(t, projectSettings, map[string]any{
		"PreToolUse": []map[string]any{
			{
				"matcher": "Bash",
				"hooks": []map[string]any{
					{"type": "command", "command": "/usr/bin/check.sh"},
				},
			},
		},
	})

	tab := NewHooksTab(dir)
	view := tab.View()

	// One handler: should display "[1 handler]" (singular).
	if !strings.Contains(view, "[1 handler]") {
		t.Errorf("view should contain '[1 handler]', got:\n%s", view)
	}
}

func TestHooksTabMultipleHandlersLabel(t *testing.T) {
	dir := t.TempDir()
	projectSettings := filepath.Join(dir, ".claude", "settings.json")
	writeSettingsJSON(t, projectSettings, map[string]any{
		"PostToolUse": []map[string]any{
			{
				"matcher": "",
				"hooks": []map[string]any{
					{"type": "command", "command": "/usr/bin/log1.sh"},
					{"type": "command", "command": "/usr/bin/log2.sh"},
				},
			},
		},
	})

	tab := NewHooksTab(dir)
	view := tab.View()

	// Two handlers: should display "[2 handlers]" (plural).
	if !strings.Contains(view, "[2 handlers]") {
		t.Errorf("view should contain '[2 handlers]', got:\n%s", view)
	}
}

func TestHooksTabFilterByCommand(t *testing.T) {
	dir := t.TempDir()
	projectSettings := filepath.Join(dir, ".claude", "settings.json")
	writeSettingsJSON(t, projectSettings, map[string]any{
		"PreToolUse": []map[string]any{
			{
				"matcher": "Bash",
				"hooks": []map[string]any{
					{"type": "command", "command": "/usr/bin/security-guard.sh"},
				},
			},
		},
		"Stop": []map[string]any{
			{
				"matcher": "",
				"hooks": []map[string]any{
					{"type": "command", "command": "/usr/bin/notify.sh"},
				},
			},
		},
	})

	tab := NewHooksTab(dir)
	tab.filter = "security"
	tab.applyFilter()

	view := tab.View()
	if !strings.Contains(view, "security-guard.sh") {
		t.Errorf("filtered view should contain security-guard.sh, got:\n%s", view)
	}
	if strings.Contains(view, "notify.sh") {
		t.Errorf("filtered view should not contain notify.sh, got:\n%s", view)
	}
}

func TestHooksTabScrollBoundary(t *testing.T) {
	dir := t.TempDir()
	tab := NewHooksTab(dir)

	// k at top should not go below 0.
	updated, _ := tab.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	tab = updated.(*HooksTab)
	if tab.cursor < 0 {
		t.Errorf("cursor went below 0: %d", tab.cursor)
	}

	// j past end should stop at last element.
	for i := 0; i < len(tab.filtered)+10; i++ {
		updated, _ = tab.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
		tab = updated.(*HooksTab)
	}
	if len(tab.filtered) > 0 && tab.cursor >= len(tab.filtered) {
		t.Errorf("cursor exceeded filtered length: got %d, len=%d", tab.cursor, len(tab.filtered))
	}
}
