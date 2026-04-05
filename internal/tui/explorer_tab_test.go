package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestExplorerTabLoadsWithoutError(t *testing.T) {
	dir := t.TempDir()
	tab := NewExplorerTab(dir)
	if tab == nil {
		t.Fatal("NewExplorerTab returned nil")
	}

	view := tab.View()
	if view == "" {
		t.Error("View() returned empty string")
	}
}

func TestExplorerTabShowsKeyDirectories(t *testing.T) {
	dir := t.TempDir()

	// Create key directories.
	for _, d := range []string{"internal", "pkg", "cmd", "docs"} {
		if err := os.MkdirAll(filepath.Join(dir, d), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", d, err)
		}
		// Add a file inside each dir.
		if err := os.WriteFile(filepath.Join(dir, d, "file.go"), []byte("package test"), 0o644); err != nil {
			t.Fatalf("write file in %s: %v", d, err)
		}
	}

	tab := NewExplorerTab(dir)
	view := tab.View()

	for _, d := range []string{"internal", "pkg", "cmd", "docs"} {
		if !strings.Contains(view, d) {
			t.Errorf("View() missing directory %q, got:\n%s", d, view)
		}
	}
}

func TestExplorerTabMissingClaudeDir(t *testing.T) {
	// Use a fake HOME so the global ~/.claude/ section is also empty.
	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)

	dir := t.TempDir()
	tab := NewExplorerTab(dir)
	view := tab.View()

	// Should indicate no .claude/ dir without panicking.
	if !strings.Contains(view, "no .claude") {
		t.Errorf("View() should indicate missing .claude dir, got:\n%s", view)
	}
}

func TestExplorerTabWithClaudeDir(t *testing.T) {
	// Use a fake HOME so global ~/.claude/ content is minimal and predictable.
	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)

	dir := t.TempDir()

	// Create a .claude directory with some files.
	claudeDir := filepath.Join(dir, ".claude")
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		t.Fatalf("mkdir .claude: %v", err)
	}
	if err := os.WriteFile(filepath.Join(claudeDir, "settings.json"), []byte(`{}`), 0o644); err != nil {
		t.Fatalf("write settings.json: %v", err)
	}

	tab := NewExplorerTab(dir)
	view := tab.View()

	if !strings.Contains(view, "settings.json") {
		t.Errorf("View() should show settings.json, got:\n%s", view)
	}
}

func TestExplorerTabTitle(t *testing.T) {
	tab := NewExplorerTab(t.TempDir())
	if tab.Title() != "Explorer" {
		t.Errorf("Title() = %q, want %q", tab.Title(), "Explorer")
	}
}

func TestExplorerTabScrolling(t *testing.T) {
	dir := t.TempDir()
	tab := NewExplorerTab(dir)

	// j scroll should not panic or go negative.
	updated, _ := tab.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	tab = updated.(*ExplorerTab)
	if tab.scroll < 0 {
		t.Errorf("scroll went negative: %d", tab.scroll)
	}

	// k should not go below 0.
	tab.scroll = 0
	updated, _ = tab.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	tab = updated.(*ExplorerTab)
	if tab.scroll < 0 {
		t.Errorf("scroll went below 0: %d", tab.scroll)
	}

	// g goes to top.
	updated, _ = tab.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("g")})
	tab = updated.(*ExplorerTab)
	if tab.scroll != 0 {
		t.Errorf("scroll should be 0 after g, got %d", tab.scroll)
	}

	// G jumps to end.
	updated, _ = tab.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("G")})
	tab = updated.(*ExplorerTab)
	if tab.scroll < 0 {
		t.Errorf("scroll below 0 after G: %d", tab.scroll)
	}
}

func TestExplorerTabCountFiles(t *testing.T) {
	dir := t.TempDir()

	// Create a directory with 3 files and 1 subdir with 2 files.
	for _, name := range []string{"a.txt", "b.txt", "c.txt"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	subDir := filepath.Join(dir, "sub")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatalf("mkdir sub: %v", err)
	}
	for _, name := range []string{"d.txt", "e.txt"} {
		if err := os.WriteFile(filepath.Join(subDir, name), []byte("x"), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	count := countFiles(dir)
	if count != 5 {
		t.Errorf("countFiles = %d, want 5", count)
	}
}

func TestFormatSize(t *testing.T) {
	tests := []struct {
		bytes int64
		want  string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{1024, "1 KB"},
		{2048, "2 KB"},
		{1048576, "1 MB"},
	}
	for _, tc := range tests {
		got := formatSize(tc.bytes)
		if got != tc.want {
			t.Errorf("formatSize(%d) = %q, want %q", tc.bytes, got, tc.want)
		}
	}
}

func TestShortenPath(t *testing.T) {
	home := "/home/user"
	tests := []struct {
		path string
		want string
	}{
		{"/home/user/project", "~/project"},
		{"/other/path", "/other/path"},
		{"/home/user", "~"},
	}
	for _, tc := range tests {
		got := shortenPath(tc.path, home)
		if got != tc.want {
			t.Errorf("shortenPath(%q, %q) = %q, want %q", tc.path, home, got, tc.want)
		}
	}
}

func TestExplorerTabIntegratedInModel(t *testing.T) {
	m := New()
	if _, ok := m.tabs[4].(*ExplorerTab); !ok {
		t.Errorf("tabs[4] type = %T, want *ExplorerTab", m.tabs[4])
	}
	if m.tabs[4].Title() != "Explorer" {
		t.Errorf("tabs[4].Title() = %q, want Explorer", m.tabs[4].Title())
	}
}
