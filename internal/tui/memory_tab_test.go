package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestMemoryTabMissingFile(t *testing.T) {
	dir := t.TempDir()
	tab := NewMemoryTab(dir)
	if tab == nil {
		t.Fatal("NewMemoryTab returned nil")
	}

	view := tab.View()
	if view == "" {
		t.Error("View() returned empty string")
	}

	// Should mention file not found, without panicking.
	if !strings.Contains(view, "not found") {
		t.Errorf("View() should indicate missing file, got:\n%s", view)
	}
}

func TestMemoryTabWithContent(t *testing.T) {
	// Create a fake home-like structure.
	fakeHome := t.TempDir()
	dir := t.TempDir()

	// Patch HOME so os.UserHomeDir returns fakeHome.
	origHome := os.Getenv("HOME")
	t.Setenv("HOME", fakeHome)
	defer os.Setenv("HOME", origHome)

	// Create the MEMORY.md at the expected path.
	encodedCWD := urlEncodePath(dir)
	memDir := filepath.Join(fakeHome, ".claude", "projects", encodedCWD, "memory")
	if err := os.MkdirAll(memDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	content := "# Memory\n\n- Entry 1\n- Entry 2\n"
	memPath := filepath.Join(memDir, "MEMORY.md")
	if err := os.WriteFile(memPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write MEMORY.md: %v", err)
	}

	tab := NewMemoryTab(dir)
	view := tab.View()

	if !strings.Contains(view, "Entry 1") {
		t.Errorf("View() should contain memory content, got:\n%s", view)
	}
	if !strings.Contains(view, "Entry 2") {
		t.Errorf("View() should contain entry 2, got:\n%s", view)
	}
}

func TestMemoryTabTitle(t *testing.T) {
	tab := NewMemoryTab(t.TempDir())
	if tab.Title() != "Memory" {
		t.Errorf("Title() = %q, want %q", tab.Title(), "Memory")
	}
}

func TestMemoryTabScrolling(t *testing.T) {
	dir := t.TempDir()
	tab := NewMemoryTab(dir)

	initial := tab.scroll

	// j should scroll down (if there are lines).
	updated, _ := tab.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	tab = updated.(*MemoryTab)

	// Should not go negative.
	if tab.scroll < 0 {
		t.Errorf("scroll went negative: %d", tab.scroll)
	}

	_ = initial

	// k should not go below 0.
	tab.scroll = 0
	updated, _ = tab.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	tab = updated.(*MemoryTab)
	if tab.scroll < 0 {
		t.Errorf("scroll went below 0: %d", tab.scroll)
	}

	// G should jump to end.
	updated, _ = tab.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("G")})
	tab = updated.(*MemoryTab)
	if tab.scroll < 0 {
		t.Errorf("scroll below 0 after G: %d", tab.scroll)
	}

	// g should jump to top.
	updated, _ = tab.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("g")})
	tab = updated.(*MemoryTab)
	if tab.scroll != 0 {
		t.Errorf("scroll should be 0 after g, got %d", tab.scroll)
	}
}

func TestURLEncodePath(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"/home/user/project", "%2Fhome%2Fuser%2Fproject"},
		{"simple", "simple"},
		{"/path with spaces", "%2Fpath%20with%20spaces"},
	}
	for _, tc := range tests {
		got := urlEncodePath(tc.input)
		if got != tc.want {
			t.Errorf("urlEncodePath(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestMemoryTabIntegratedInModel(t *testing.T) {
	m := New()
	if _, ok := m.tabs[3].(*MemoryTab); !ok {
		t.Errorf("tabs[3] type = %T, want *MemoryTab", m.tabs[3])
	}
	if m.tabs[3].Title() != "Memory" {
		t.Errorf("tabs[3].Title() = %q, want Memory", m.tabs[3].Title())
	}
}

func TestMemoryTabScrollIncrement(t *testing.T) {
	// Create a tab with real content so there are multiple lines to scroll through.
	fakeHome := t.TempDir()
	dir := t.TempDir()
	t.Setenv("HOME", fakeHome)

	encodedCWD := urlEncodePath(dir)
	memDir := filepath.Join(fakeHome, ".claude", "projects", encodedCWD, "memory")
	if err := os.MkdirAll(memDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	// Write enough lines to allow scrolling.
	var content strings.Builder
	for i := 0; i < 20; i++ {
		content.WriteString("- Line entry number\n")
	}
	if err := os.WriteFile(filepath.Join(memDir, "MEMORY.md"), []byte(content.String()), 0o644); err != nil {
		t.Fatalf("write MEMORY.md: %v", err)
	}

	tab := NewMemoryTab(dir)
	initialScroll := tab.scroll // should be 0

	updated, _ := tab.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	tab = updated.(*MemoryTab)
	if tab.scroll != initialScroll+1 {
		t.Errorf("scroll after j = %d, want %d", tab.scroll, initialScroll+1)
	}

	// k should decrement.
	updated, _ = tab.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	tab = updated.(*MemoryTab)
	if tab.scroll != initialScroll {
		t.Errorf("scroll after k = %d, want %d", tab.scroll, initialScroll)
	}
}

func TestMemoryTabGoToEnd(t *testing.T) {
	fakeHome := t.TempDir()
	dir := t.TempDir()
	t.Setenv("HOME", fakeHome)

	encodedCWD := urlEncodePath(dir)
	memDir := filepath.Join(fakeHome, ".claude", "projects", encodedCWD, "memory")
	if err := os.MkdirAll(memDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	var content strings.Builder
	for i := 0; i < 10; i++ {
		content.WriteString("- Entry\n")
	}
	if err := os.WriteFile(filepath.Join(memDir, "MEMORY.md"), []byte(content.String()), 0o644); err != nil {
		t.Fatalf("write MEMORY.md: %v", err)
	}

	tab := NewMemoryTab(dir)
	nLines := len(tab.lines)
	if nLines == 0 {
		t.Fatal("expected non-empty lines")
	}

	// G jumps to end.
	updated, _ := tab.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("G")})
	tab = updated.(*MemoryTab)
	if tab.scroll != nLines-1 {
		t.Errorf("scroll after G = %d, want %d", tab.scroll, nLines-1)
	}

	// g jumps back to top.
	updated, _ = tab.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("g")})
	tab = updated.(*MemoryTab)
	if tab.scroll != 0 {
		t.Errorf("scroll after g = %d, want 0", tab.scroll)
	}
}
