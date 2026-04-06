package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/BrenanL/hitch/pkg/envvars"
)

func TestEnvVarsTabCategoriesLoaded(t *testing.T) {
	tab := NewEnvVarsTab()

	// Collect category headers from rows
	var gotCategories []string
	for _, r := range tab.rows {
		if r.isCategory {
			gotCategories = append(gotCategories, r.category)
		}
	}

	wantCategories := envvars.Categories()
	if len(gotCategories) != len(wantCategories) {
		t.Fatalf("categories loaded: got %d, want %d", len(gotCategories), len(wantCategories))
	}

	// Verify all expected categories are present (Categories() returns sorted list)
	catSet := make(map[string]bool, len(gotCategories))
	for _, c := range gotCategories {
		catSet[c] = true
	}
	for _, c := range wantCategories {
		if !catSet[c] {
			t.Errorf("category %q not loaded", c)
		}
	}
}

func TestEnvVarsTabAllVarsLoaded(t *testing.T) {
	tab := NewEnvVarsTab()

	all := envvars.All()
	var varRows int
	for _, r := range tab.rows {
		if !r.isCategory {
			varRows++
		}
	}

	if varRows != len(all) {
		t.Errorf("var rows loaded: got %d, want %d", varRows, len(all))
	}
}

func TestEnvVarsTabScrollDown(t *testing.T) {
	tab := NewEnvVarsTab()

	// Find the first non-category row index (cursor starts at 0, but may be on a category)
	initial := tab.cursor

	// Press j to move down
	updated, _ := tab.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	tab = updated.(*EnvVarsTab)

	if tab.cursor <= initial {
		t.Errorf("cursor did not advance: initial=%d, after j=%d", initial, tab.cursor)
	}

	// Cursor should not land on a category header
	if tab.cursor < len(tab.filtered) && tab.filtered[tab.cursor].isCategory {
		t.Errorf("cursor landed on a category header at index %d", tab.cursor)
	}
}

func TestEnvVarsTabScrollUp(t *testing.T) {
	tab := NewEnvVarsTab()

	// Move down a few times first
	for i := 0; i < 3; i++ {
		updated, _ := tab.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
		tab = updated.(*EnvVarsTab)
	}
	afterDown := tab.cursor

	// Now move up
	updated, _ := tab.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	tab = updated.(*EnvVarsTab)

	if tab.cursor >= afterDown {
		t.Errorf("cursor did not move up: after down=%d, after k=%d", afterDown, tab.cursor)
	}

	// Cursor should not land on a category header
	if tab.cursor < len(tab.filtered) && tab.filtered[tab.cursor].isCategory {
		t.Errorf("cursor landed on a category header at index %d", tab.cursor)
	}
}

func TestEnvVarsTabScrollBoundary(t *testing.T) {
	tab := NewEnvVarsTab()

	// Pressing k at the top should not go below 0
	updated, _ := tab.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	tab = updated.(*EnvVarsTab)
	if tab.cursor < 0 {
		t.Errorf("cursor went below 0: got %d", tab.cursor)
	}

	// Press j past the end should stop at last row
	for i := 0; i < len(tab.filtered)+10; i++ {
		updated, _ = tab.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
		tab = updated.(*EnvVarsTab)
	}
	if tab.cursor >= len(tab.filtered) {
		t.Errorf("cursor exceeded filtered length: got %d, len=%d", tab.cursor, len(tab.filtered))
	}
}

func TestEnvVarsTabFilterByName(t *testing.T) {
	tab := NewEnvVarsTab()

	// Type "/" to start filtering, then type a known var name fragment
	updated, _ := tab.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
	tab = updated.(*EnvVarsTab)
	if !tab.filtering {
		t.Fatal("expected filtering mode after /")
	}

	// Type "ANTHROPIC_API_KEY"
	for _, ch := range "ANTHROPIC_API" {
		updated, _ = tab.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{ch}})
		tab = updated.(*EnvVarsTab)
	}

	// Filter applied; no category headers in results when filter is active
	for _, r := range tab.filtered {
		if r.isCategory {
			t.Errorf("unexpected category header in filtered results")
		}
		if !strings.Contains(strings.ToLower(r.ev.Name), strings.ToLower(tab.filter)) &&
			!strings.Contains(strings.ToLower(r.ev.Description), strings.ToLower(tab.filter)) {
			t.Errorf("row %q does not match filter %q", r.ev.Name, tab.filter)
		}
	}

	if len(tab.filtered) == 0 {
		t.Error("expected at least one result for ANTHROPIC_API filter")
	}
}

func TestEnvVarsTabFilterEscapeClears(t *testing.T) {
	tab := NewEnvVarsTab()

	// Start filtering
	updated, _ := tab.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
	tab = updated.(*EnvVarsTab)

	for _, ch := range "MODEL" {
		updated, _ = tab.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{ch}})
		tab = updated.(*EnvVarsTab)
	}
	if len(tab.filtered) == 0 {
		t.Fatal("expected filtered results before Escape")
	}
	filteredCount := len(tab.filtered)

	// Escape clears filter
	updated, _ = tab.Update(tea.KeyMsg{Type: tea.KeyEscape})
	tab = updated.(*EnvVarsTab)

	if tab.filtering {
		t.Error("expected filtering=false after Escape")
	}
	if tab.filter != "" {
		t.Errorf("expected filter cleared, got %q", tab.filter)
	}
	if len(tab.filtered) <= filteredCount {
		t.Errorf("expected more rows after clearing filter: got %d, had %d", len(tab.filtered), filteredCount)
	}
}

func TestEnvVarsTabFilterByDescription(t *testing.T) {
	tab := NewEnvVarsTab()

	// "prompt caching" appears in description of several vars
	updated, _ := tab.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
	tab = updated.(*EnvVarsTab)

	for _, ch := range "prompt caching" {
		updated, _ = tab.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{ch}})
		tab = updated.(*EnvVarsTab)
	}

	if len(tab.filtered) == 0 {
		t.Error("expected results when filtering by description text 'prompt caching'")
	}
}

func TestEnvVarsTabTitle(t *testing.T) {
	tab := NewEnvVarsTab()
	if tab.Title() != "Env Vars" {
		t.Errorf("Title() = %q, want %q", tab.Title(), "Env Vars")
	}
}

func TestEnvVarsTabViewNotEmpty(t *testing.T) {
	tab := NewEnvVarsTab()
	view := tab.View()
	if view == "" {
		t.Error("View() returned empty string")
	}
}

func TestEnvVarsTabIntegratedInModel(t *testing.T) {
	m := New()

	// Tab 2 (index 1) should be the EnvVarsTab
	if _, ok := m.tabs[1].(*EnvVarsTab); !ok {
		t.Errorf("tabs[1] type = %T, want *EnvVarsTab", m.tabs[1])
	}

	if m.tabs[1].Title() != "Env Vars" {
		t.Errorf("tabs[1].Title() = %q, want %q", m.tabs[1].Title(), "Env Vars")
	}
}

func TestIsSensitive(t *testing.T) {
	sensitive := []string{
		"ANTHROPIC_API_KEY",
		"CLAUDE_TOKEN",
		"MY_SECRET",
		"DB_PASSWORD",
		"github_token",
	}
	for _, name := range sensitive {
		if !isSensitive(name) {
			t.Errorf("isSensitive(%q) = false, want true", name)
		}
	}

	notSensitive := []string{
		"ANTHROPIC_MODEL",
		"CLAUDE_DEBUG",
		"HOME",
		"MAX_REQUESTS",
	}
	for _, name := range notSensitive {
		if isSensitive(name) {
			t.Errorf("isSensitive(%q) = true, want false", name)
		}
	}
}

func TestEnvVarsTabSensitiveMasked(t *testing.T) {
	// Set a sensitive env var so it appears as set.
	t.Setenv("ANTHROPIC_API_KEY", "sk-real-secret-value")

	tab := NewEnvVarsTab()
	view := tab.View()

	// Real value must not appear in the view.
	if strings.Contains(view, "sk-real-secret-value") {
		t.Error("sensitive value leaked into View()")
	}
	// Masked value should appear.
	if !strings.Contains(view, "●●●●●●●●") {
		t.Error("masked placeholder not found in View() for sensitive var")
	}
}

func TestEnvVarsTabFilterBackspace(t *testing.T) {
	tab := NewEnvVarsTab()

	// Enter filter mode and type "ABC".
	updated, _ := tab.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
	tab = updated.(*EnvVarsTab)
	for _, ch := range "ABC" {
		updated, _ = tab.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{ch}})
		tab = updated.(*EnvVarsTab)
	}
	if tab.filter != "ABC" {
		t.Fatalf("filter before backspace = %q, want ABC", tab.filter)
	}

	// Backspace removes last character.
	updated, _ = tab.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	tab = updated.(*EnvVarsTab)
	if tab.filter != "AB" {
		t.Errorf("filter after backspace = %q, want AB", tab.filter)
	}
}

func TestEnvVarsTabFilterEnterExitsFilterMode(t *testing.T) {
	tab := NewEnvVarsTab()

	updated, _ := tab.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
	tab = updated.(*EnvVarsTab)
	if !tab.filtering {
		t.Fatal("expected filtering=true after /")
	}

	// Enter commits and exits filter mode while keeping the filter string.
	updated, _ = tab.Update(tea.KeyMsg{Type: tea.KeyEnter})
	tab = updated.(*EnvVarsTab)
	if tab.filtering {
		t.Error("expected filtering=false after Enter")
	}
}
