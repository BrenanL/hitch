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
	tests := []struct{ n int; want string }{
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
