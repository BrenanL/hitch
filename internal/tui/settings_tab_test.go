package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/BrenanL/hitch/pkg/settings"
)

// makeTestSettingsTab creates a SettingsTab loaded from a temp directory
// that has a minimal project settings file.
func makeTestSettingsTab(t *testing.T) SettingsTab {
	t.Helper()
	dir := t.TempDir()
	claude := filepath.Join(dir, ".claude")
	if err := os.MkdirAll(claude, 0o755); err != nil {
		t.Fatalf("mkdir .claude: %v", err)
	}
	// Write a minimal project settings file with one known key.
	b := []byte(`{"effortLevel":"high"}`)
	if err := os.WriteFile(filepath.Join(claude, "settings.json"), b, 0o644); err != nil {
		t.Fatalf("write settings.json: %v", err)
	}
	tab := SettingsTab{collapsed: make(map[string]bool)}
	tab.load(dir)
	return tab
}

// sendKeyToTab sends a rune key to the tab and returns the updated tab.
func sendKeyToTab(t *testing.T, tab SettingsTab, key string) SettingsTab {
	t.Helper()
	updated, _ := tab.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)})
	return updated.(SettingsTab)
}

// sendSpecialKeyToTab sends a special (non-rune) key to the tab.
func sendSpecialKeyToTab(t *testing.T, tab SettingsTab, keyType tea.KeyType) SettingsTab {
	t.Helper()
	updated, _ := tab.Update(tea.KeyMsg{Type: keyType})
	return updated.(SettingsTab)
}

// TestSettingsTabShowsCategories verifies that after loading, the tab has
// category header rows for at least the core categories.
func TestSettingsTabShowsCategories(t *testing.T) {
	tab := makeTestSettingsTab(t)

	if len(tab.rows) == 0 {
		t.Fatal("rows is empty after load")
	}

	categoryNames := make(map[string]bool)
	for _, row := range tab.rows {
		if row.kind == rowKindCategory {
			categoryNames[row.categoryName] = true
		}
	}

	required := []string{"GENERAL", "PERMISSIONS", "DISPLAY", "MANAGED (read-only)"}
	for _, cat := range required {
		if !categoryNames[cat] {
			t.Errorf("missing category %q in rows", cat)
		}
	}
}

// TestSettingsTabInitialState verifies the tab starts with cursor=0, no filter, no filterMode.
func TestSettingsTabInitialState(t *testing.T) {
	tab := makeTestSettingsTab(t)

	if tab.cursor != 0 {
		t.Errorf("cursor = %d, want 0", tab.cursor)
	}
	if tab.filter != "" {
		t.Errorf("filter = %q, want empty", tab.filter)
	}
	if tab.filterMode {
		t.Error("filterMode = true, want false")
	}
	if tab.err != nil {
		t.Errorf("err = %v, want nil", tab.err)
	}
}

// TestSettingsTabTitle verifies the tab returns the expected title.
func TestSettingsTabTitle(t *testing.T) {
	tab := makeTestSettingsTab(t)
	if tab.Title() != "Settings" {
		t.Errorf("Title() = %q, want %q", tab.Title(), "Settings")
	}
}

// TestSettingsTabScrollDown verifies j/down moves the cursor forward.
func TestSettingsTabScrollDown(t *testing.T) {
	tab := makeTestSettingsTab(t)

	visible := tab.visibleRows()
	if len(visible) < 2 {
		t.Fatalf("not enough visible rows (%d) to test scroll", len(visible))
	}

	tab = sendKeyToTab(t, tab, "j")
	if tab.cursor != 1 {
		t.Errorf("after j: cursor = %d, want 1", tab.cursor)
	}

	tab = sendKeyToTab(t, tab, "j")
	if tab.cursor != 2 {
		t.Errorf("after second j: cursor = %d, want 2", tab.cursor)
	}
}

// TestSettingsTabScrollUp verifies k/up moves the cursor backward.
func TestSettingsTabScrollUp(t *testing.T) {
	tab := makeTestSettingsTab(t)
	tab.cursor = 3

	tab = sendKeyToTab(t, tab, "k")
	if tab.cursor != 2 {
		t.Errorf("after k: cursor = %d, want 2", tab.cursor)
	}
}

// TestSettingsTabScrollClampAtTop verifies cursor doesn't go below 0.
func TestSettingsTabScrollClampAtTop(t *testing.T) {
	tab := makeTestSettingsTab(t)
	tab.cursor = 0

	tab = sendKeyToTab(t, tab, "k")
	if tab.cursor != 0 {
		t.Errorf("cursor went below 0: got %d", tab.cursor)
	}
}

// TestSettingsTabScrollClampAtBottom verifies cursor doesn't exceed visible rows.
func TestSettingsTabScrollClampAtBottom(t *testing.T) {
	tab := makeTestSettingsTab(t)
	visible := tab.visibleRows()
	tab.cursor = len(visible) - 1

	tab = sendKeyToTab(t, tab, "j")
	if tab.cursor != len(visible)-1 {
		t.Errorf("cursor exceeded last row: got %d, want %d", tab.cursor, len(visible)-1)
	}
}

// TestSettingsTabPageDown verifies pgdown advances cursor by ~10.
func TestSettingsTabPageDown(t *testing.T) {
	tab := makeTestSettingsTab(t)
	visible := tab.visibleRows()
	if len(visible) < 11 {
		t.Skip("not enough rows to test page-down")
	}

	updated, _ := tab.Update(tea.KeyMsg{Type: tea.KeyPgDown})
	tab = updated.(SettingsTab)
	if tab.cursor != 10 {
		t.Errorf("after pgdown: cursor = %d, want 10", tab.cursor)
	}
}

// TestSettingsTabPageUp verifies pgup decrements cursor by ~10.
func TestSettingsTabPageUp(t *testing.T) {
	tab := makeTestSettingsTab(t)
	tab.cursor = 15

	updated, _ := tab.Update(tea.KeyMsg{Type: tea.KeyPgUp})
	tab = updated.(SettingsTab)
	if tab.cursor != 5 {
		t.Errorf("after pgup: cursor = %d, want 5", tab.cursor)
	}
}

// TestSettingsTabGoToTop verifies g moves cursor to 0.
func TestSettingsTabGoToTop(t *testing.T) {
	tab := makeTestSettingsTab(t)
	tab.cursor = 5

	tab = sendKeyToTab(t, tab, "g")
	if tab.cursor != 0 {
		t.Errorf("after g: cursor = %d, want 0", tab.cursor)
	}
}

// TestSettingsTabGoToBottom verifies G moves cursor to last visible row.
func TestSettingsTabGoToBottom(t *testing.T) {
	tab := makeTestSettingsTab(t)
	tab.cursor = 0

	tab = sendKeyToTab(t, tab, "G")
	visible := tab.visibleRows()
	if tab.cursor != len(visible)-1 {
		t.Errorf("after G: cursor = %d, want %d", tab.cursor, len(visible)-1)
	}
}

// TestSettingsTabFilterEnter verifies / activates filter mode.
func TestSettingsTabFilterEnter(t *testing.T) {
	tab := makeTestSettingsTab(t)

	tab = sendKeyToTab(t, tab, "/")
	if !tab.filterMode {
		t.Error("filterMode should be true after pressing /")
	}
}

// TestSettingsTabFilterType verifies typing in filter mode updates the filter string.
func TestSettingsTabFilterType(t *testing.T) {
	tab := makeTestSettingsTab(t)
	tab = sendKeyToTab(t, tab, "/")

	tab = sendKeyToTab(t, tab, "e")
	tab = sendKeyToTab(t, tab, "f")
	tab = sendKeyToTab(t, tab, "f")

	if tab.filter != "eff" {
		t.Errorf("filter = %q, want %q", tab.filter, "eff")
	}
}

// TestSettingsTabFilterReducesRows verifies that filtering hides non-matching rows.
func TestSettingsTabFilterReducesRows(t *testing.T) {
	tab := makeTestSettingsTab(t)
	totalVisible := len(tab.visibleRows())

	// Filter for a specific key name that exists.
	tab = sendKeyToTab(t, tab, "/")
	for _, ch := range "effortLevel" {
		tab = sendKeyToTab(t, tab, string(ch))
	}

	filteredVisible := len(tab.visibleRows())
	if filteredVisible >= totalVisible {
		t.Errorf("filter did not reduce rows: before=%d after=%d", totalVisible, filteredVisible)
	}

	// The filtered result should still contain the effortLevel key.
	found := false
	for _, row := range tab.visibleRows() {
		if row.kind == rowKindKey && row.keyName == "effortLevel" {
			found = true
			break
		}
	}
	if !found {
		t.Error("effortLevel not found after filtering for it")
	}
}

// TestSettingsTabFilterClear verifies Esc clears the filter and restores all rows.
func TestSettingsTabFilterClear(t *testing.T) {
	tab := makeTestSettingsTab(t)
	totalVisible := len(tab.visibleRows())

	tab = sendKeyToTab(t, tab, "/")
	for _, ch := range "effortLevel" {
		tab = sendKeyToTab(t, tab, string(ch))
	}

	tab = sendSpecialKeyToTab(t, tab, tea.KeyEsc)
	if tab.filterMode {
		t.Error("filterMode should be false after Esc")
	}
	if tab.filter != "" {
		t.Errorf("filter should be empty after Esc, got %q", tab.filter)
	}

	restoredVisible := len(tab.visibleRows())
	if restoredVisible != totalVisible {
		t.Errorf("visible rows after clear = %d, want %d", restoredVisible, totalVisible)
	}
}

// TestSettingsTabCategoryCollapse verifies Enter on a category header collapses it.
func TestSettingsTabCategoryCollapse(t *testing.T) {
	tab := makeTestSettingsTab(t)

	// Navigate cursor to the first category header row.
	// Row 0 should be the GENERAL category header.
	if len(tab.rows) == 0 || tab.rows[0].kind != rowKindCategory {
		t.Fatal("first row is not a category header")
	}

	// Confirm cursor is on the category header.
	tab.cursor = 0
	visible := tab.visibleRows()
	if visible[0].kind != rowKindCategory {
		t.Fatalf("visible[0] kind = %v, want category", visible[0].kind)
	}
	categoryName := visible[0].categoryName

	// Count visible rows in this category before collapse.
	beforeCount := len(tab.visibleRows())

	// Press Enter to collapse.
	updated, _ := tab.Update(tea.KeyMsg{Type: tea.KeyEnter})
	tab = updated.(SettingsTab)

	if !tab.collapsed[categoryName] {
		t.Errorf("category %q should be collapsed after Enter", categoryName)
	}

	afterCount := len(tab.visibleRows())
	if afterCount >= beforeCount {
		t.Errorf("collapsing category did not reduce row count: before=%d after=%d", beforeCount, afterCount)
	}
}

// TestSettingsTabCategoryExpand verifies Enter on a collapsed category expands it.
func TestSettingsTabCategoryExpand(t *testing.T) {
	tab := makeTestSettingsTab(t)

	// Collapse the first category manually.
	visible := tab.visibleRows()
	if len(visible) == 0 || visible[0].kind != rowKindCategory {
		t.Fatal("first visible row is not a category header")
	}
	categoryName := visible[0].categoryName
	tab.collapsed[categoryName] = true
	tab.cursor = 0

	collapsedCount := len(tab.visibleRows())

	// Press Enter to expand.
	updated, _ := tab.Update(tea.KeyMsg{Type: tea.KeyEnter})
	tab = updated.(SettingsTab)

	if tab.collapsed[categoryName] {
		t.Errorf("category %q should be expanded after Enter", categoryName)
	}

	expandedCount := len(tab.visibleRows())
	if expandedCount <= collapsedCount {
		t.Errorf("expanding category did not increase row count: collapsed=%d expanded=%d", collapsedCount, expandedCount)
	}
}

// TestSettingsTabViewRendersContent verifies View() returns non-empty string with
// key names and category headers.
func TestSettingsTabViewRendersContent(t *testing.T) {
	tab := makeTestSettingsTab(t)
	view := tab.View()

	if view == "" {
		t.Fatal("View() returned empty string")
	}
	if !strings.Contains(view, "GENERAL") {
		t.Error("view does not contain GENERAL category")
	}
	if !strings.Contains(view, "effortLevel") {
		t.Error("view does not contain effortLevel key")
	}
}

// TestSettingsTabViewShowsEffectiveValue verifies that a key set in a settings file
// shows its value in the view.
func TestSettingsTabViewShowsEffectiveValue(t *testing.T) {
	tab := makeTestSettingsTab(t)
	view := tab.View()

	// The project settings file sets effortLevel = "high".
	if !strings.Contains(view, "high") {
		t.Errorf("view does not contain effective value 'high':\n%s", view)
	}
}

// TestSettingsTabViewShowsScopeBadge verifies that a key with a value shows a scope badge.
func TestSettingsTabViewShowsScopeBadge(t *testing.T) {
	tab := makeTestSettingsTab(t)
	view := tab.View()

	// effortLevel was set in project scope, so the badge should be [P].
	if !strings.Contains(view, "[P]") {
		t.Errorf("view does not contain project scope badge [P]:\n%s", view)
	}
}

// TestSettingsTabProjectScopeKeyPresent verifies that a key set in project scope
// appears in the rows with hasValue=true and the correct scope.
func TestSettingsTabProjectScopeKeyPresent(t *testing.T) {
	// makeTestSettingsTab writes effortLevel = "high" to project scope.
	tab := makeTestSettingsTab(t)

	found := false
	for _, row := range tab.rows {
		if row.kind == rowKindKey && row.keyName == "effortLevel" {
			found = true
			if !row.hasValue {
				t.Error("effortLevel.hasValue should be true")
			}
			if row.isDefault {
				t.Error("effortLevel.isDefault should be false when set in project settings")
			}
			if row.scope != settings.ScopeProject {
				t.Errorf("effortLevel.scope = %v, want ScopeProject", row.scope)
			}
			break
		}
	}
	if !found {
		t.Error("effortLevel key not found in rows")
	}
}

// TestSettingsTabFilterBackspace verifies Backspace removes the last character from the filter.
func TestSettingsTabFilterBackspace(t *testing.T) {
	tab := makeTestSettingsTab(t)
	tab = sendKeyToTab(t, tab, "/")
	tab = sendKeyToTab(t, tab, "a")
	tab = sendKeyToTab(t, tab, "b")
	tab = sendKeyToTab(t, tab, "c")

	if tab.filter != "abc" {
		t.Fatalf("filter = %q before backspace, want %q", tab.filter, "abc")
	}

	tab = sendSpecialKeyToTab(t, tab, tea.KeyBackspace)
	if tab.filter != "ab" {
		t.Errorf("filter = %q after backspace, want %q", tab.filter, "ab")
	}
}

// TestSettingsTabImplementsTabModel verifies the interface at compile time.
func TestSettingsTabImplementsTabModel(t *testing.T) {
	var _ tabModel = SettingsTab{}
}
