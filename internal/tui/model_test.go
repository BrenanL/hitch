package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func sendKey(m Model, key string) Model {
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)})
	return updated.(Model)
}

func sendSpecialKey(m Model, keyType tea.KeyType) Model {
	updated, _ := m.Update(tea.KeyMsg{Type: keyType})
	return updated.(Model)
}

func TestTabSwitchForward(t *testing.T) {
	m := New()
	if m.activeTab != 0 {
		t.Fatalf("initial activeTab = %d, want 0", m.activeTab)
	}

	// Tab key advances one at a time.
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = updated.(Model)
	if m.activeTab != 1 {
		t.Errorf("after Tab: activeTab = %d, want 1", m.activeTab)
	}

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = updated.(Model)
	if m.activeTab != 2 {
		t.Errorf("after second Tab: activeTab = %d, want 2", m.activeTab)
	}
}

func TestTabSwitchBackward(t *testing.T) {
	m := New()
	m.activeTab = 2

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	m = updated.(Model)
	if m.activeTab != 1 {
		t.Errorf("after Shift+Tab: activeTab = %d, want 1", m.activeTab)
	}
}

func TestTabSwitchWrapForward(t *testing.T) {
	m := New()
	m.activeTab = len(m.tabs) - 1

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = updated.(Model)
	if m.activeTab != 0 {
		t.Errorf("wrap forward: activeTab = %d, want 0", m.activeTab)
	}
}

func TestTabSwitchWrapBackward(t *testing.T) {
	m := New()
	m.activeTab = 0

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	m = updated.(Model)
	if m.activeTab != len(m.tabs)-1 {
		t.Errorf("wrap backward: activeTab = %d, want %d", m.activeTab, len(m.tabs)-1)
	}
}

func TestNumberKeyJumps(t *testing.T) {
	tests := []struct {
		key  string
		want int
	}{
		{"1", 0},
		{"2", 1},
		{"3", 2},
		{"4", 3},
		{"5", 4},
	}

	for _, tc := range tests {
		m := New()
		updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(tc.key)})
		m = updated.(Model)
		if m.activeTab != tc.want {
			t.Errorf("key %q: activeTab = %d, want %d", tc.key, m.activeTab, tc.want)
		}
	}
}

func TestHelpToggle(t *testing.T) {
	m := New()
	if m.showHelp {
		t.Fatal("showHelp should be false initially")
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("?")})
	m = updated.(Model)
	if !m.showHelp {
		t.Error("showHelp should be true after pressing ?")
	}

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("?")})
	m = updated.(Model)
	if m.showHelp {
		t.Error("showHelp should be false after pressing ? again")
	}
}

func TestQuitKey(t *testing.T) {
	m := New()

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	if cmd == nil {
		t.Fatal("q key should return a quit command")
	}
	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Errorf("q key: cmd returned %T, want tea.QuitMsg", msg)
	}
}

func TestCtrlCQuit(t *testing.T) {
	m := New()

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Fatal("ctrl+c should return a quit command")
	}
	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Errorf("ctrl+c: cmd returned %T, want tea.QuitMsg", msg)
	}
}

func TestWindowResizeUpdatesDimensions(t *testing.T) {
	m := New()

	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = updated.(Model)
	if m.width != 120 {
		t.Errorf("width = %d, want 120", m.width)
	}
	if m.height != 40 {
		t.Errorf("height = %d, want 40", m.height)
	}
}
