package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

type tabModel interface {
	Init() tea.Cmd
	Update(tea.Msg) (tabModel, tea.Cmd)
	View() string
	Title() string
}

// Model is the top-level Bubbletea model for the hitch TUI.
type Model struct {
	tabs      []tabModel
	activeTab int
	width     int
	height    int
	showHelp  bool
}

// New creates a Model with placeholder tabs.
func New() Model {
	return Model{
		tabs: []tabModel{
			placeholderTab{title: "Settings"},
			placeholderTab{title: "Env Vars"},
			placeholderTab{title: "Hooks"},
			placeholderTab{title: "Memory"},
			placeholderTab{title: "Explorer"},
		},
	}
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd {
	var cmds []tea.Cmd
	for _, t := range m.tabs {
		if cmd := t.Init(); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	return tea.Batch(cmds...)
}

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "?":
			m.showHelp = !m.showHelp
			return m, nil
		case "tab":
			m.activeTab = (m.activeTab + 1) % len(m.tabs)
			return m, nil
		case "shift+tab":
			m.activeTab = (m.activeTab - 1 + len(m.tabs)) % len(m.tabs)
			return m, nil
		case "1", "2", "3", "4", "5":
			idx := int(msg.String()[0]-'1')
			if idx >= 0 && idx < len(m.tabs) {
				m.activeTab = idx
			}
			return m, nil
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	}

	// Pass message to active tab.
	updated, cmd := m.tabs[m.activeTab].Update(msg)
	m.tabs[m.activeTab] = updated
	return m, cmd
}

// View implements tea.Model.
func (m Model) View() string {
	if len(m.tabs) == 0 {
		return ""
	}

	var sb strings.Builder

	// Tab bar.
	tabBar := m.renderTabBar()
	sb.WriteString(tabBar)
	sb.WriteString("\n")

	// Active tab content.
	content := m.tabs[m.activeTab].View()
	sb.WriteString(content)
	sb.WriteString("\n")

	// Status bar.
	sb.WriteString(m.renderStatusBar())

	// Help overlay (rendered on top when visible).
	if m.showHelp {
		sb.WriteString("\n")
		sb.WriteString(m.renderHelp())
	}

	return sb.String()
}

func (m Model) renderTabBar() string {
	var parts []string
	for i, t := range m.tabs {
		label := fmt.Sprintf("%d. %s", i+1, t.Title())
		if i == m.activeTab {
			parts = append(parts, styleActiveTab.Render(label))
		} else {
			parts = append(parts, styleInactiveTab.Render(label))
		}
	}
	bar := strings.Join(parts, " ")
	if m.width > 0 {
		bar = styleTabBar.Width(m.width).Render(bar)
	} else {
		bar = styleTabBar.Render(bar)
	}
	return bar
}

func (m Model) renderStatusBar() string {
	status := "Tab/Shift+Tab: switch tab  1-5: jump  ?: help  q: quit"
	if m.width > 0 {
		return styleStatusBar.Width(m.width).Render(status)
	}
	return styleStatusBar.Render(status)
}

func (m Model) renderHelp() string {
	help := `Key Bindings

  Tab / Shift+Tab   Cycle through tabs
  1 - 5             Jump to tab by number
  ?                 Toggle this help overlay
  q / Ctrl+C        Quit`
	return styleHelpOverlay.Render(help)
}
