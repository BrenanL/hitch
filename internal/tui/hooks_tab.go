package tui

import (
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/BrenanL/hitch/pkg/settings"
)

var (
	styleHookEventHeader = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("214")) // orange

	styleHookCursor = lipgloss.NewStyle().
			Foreground(lipgloss.Color("15")).
			Background(lipgloss.Color("62"))

	styleHookScopeBadge = lipgloss.NewStyle().
				Foreground(lipgloss.Color("39")) // blue

	styleHookType = lipgloss.NewStyle().
			Foreground(lipgloss.Color("245")) // dim
)

// hookDisplayRow is a single displayable row in the hooks tab.
type hookDisplayRow struct {
	isEvent   bool
	eventName string // set if isEvent
	count     int    // handler count, set if isEvent

	isEntry   bool
	hookType  string // "command", "http", "prompt", "agent"
	command   string
	matcher   string
	scope     string // "user", "project", "local", "managed"
}

// HooksTab implements tabModel for Tab 3 (Hooks).
type HooksTab struct {
	rows     []hookDisplayRow
	filtered []hookDisplayRow
	cursor   int
	filter   string
	filtering bool
	cwd      string
}

// NewHooksTab creates a HooksTab loaded from all settings scopes.
// cwd is the project directory for loading project/local scopes.
func NewHooksTab(cwd string) *HooksTab {
	t := &HooksTab{cwd: cwd}
	t.load()
	t.applyFilter()
	return t
}

func (t *HooksTab) load() {
	// Attempt to get cwd if not set.
	cwd := t.cwd
	if cwd == "" {
		var err error
		cwd, err = os.Getwd()
		if err != nil {
			cwd = "."
		}
	}

	allSettings, err := settings.LoadAll(cwd)
	if err != nil {
		// On error, show empty list.
		t.rows = nil
		return
	}

	scopes := []string{"user", "project", "local", "managed"}

	// Collect all hooks grouped by event, across all scopes.
	// eventEntries maps event name -> list of hookDisplayRow (entry rows)
	eventEntries := make(map[string][]hookDisplayRow)
	var eventOrder []string
	seen := make(map[string]bool)

	for i, s := range allSettings {
		scope := scopes[i]
		for event, groups := range s.Hooks {
			if !seen[event] {
				seen[event] = true
				eventOrder = append(eventOrder, event)
			}
			for _, group := range groups {
				matcher := group.Matcher
				for _, hook := range group.Hooks {
					htype := hook.Type
					if htype == "" {
						htype = "command"
					}
					cmd := hook.Command
					if len(cmd) > 60 {
						cmd = cmd[:57] + "..."
					}
					eventEntries[event] = append(eventEntries[event], hookDisplayRow{
						isEntry:  true,
						hookType: htype,
						command:  cmd,
						matcher:  matcher,
						scope:    scope,
					})
				}
			}
		}
	}

	// Sort event names for stable output.
	sortedEvents := sortedKeys(eventOrder)

	var rows []hookDisplayRow
	for _, event := range sortedEvents {
		entries := eventEntries[event]
		rows = append(rows, hookDisplayRow{
			isEvent:   true,
			eventName: event,
			count:     len(entries),
		})
		rows = append(rows, entries...)
	}
	t.rows = rows
}

// sortedKeys returns the slice deduplicated and sorted.
func sortedKeys(order []string) []string {
	seen := make(map[string]bool)
	result := make([]string, 0, len(order))
	for _, k := range order {
		if !seen[k] {
			seen[k] = true
			result = append(result, k)
		}
	}
	// Simple insertion sort for stable alphabetical output.
	for i := 1; i < len(result); i++ {
		for j := i; j > 0 && result[j] < result[j-1]; j-- {
			result[j], result[j-1] = result[j-1], result[j]
		}
	}
	return result
}

func (t *HooksTab) applyFilter() {
	if t.filter == "" {
		t.filtered = make([]hookDisplayRow, len(t.rows))
		copy(t.filtered, t.rows)
		return
	}
	q := strings.ToLower(t.filter)
	var result []hookDisplayRow
	for _, r := range t.rows {
		if r.isEvent {
			if strings.Contains(strings.ToLower(r.eventName), q) {
				result = append(result, r)
			}
			continue
		}
		if strings.Contains(strings.ToLower(r.command), q) ||
			strings.Contains(strings.ToLower(r.matcher), q) ||
			strings.Contains(strings.ToLower(r.hookType), q) {
			result = append(result, r)
		}
	}
	t.filtered = result
}

// Init implements tabModel.
func (t *HooksTab) Init() tea.Cmd {
	return nil
}

// Update implements tabModel.
func (t *HooksTab) Update(msg tea.Msg) (tabModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if t.filtering {
			return t.updateFilterInput(msg)
		}
		switch msg.String() {
		case "j", "down":
			if t.cursor < len(t.filtered)-1 {
				t.cursor++
			}
		case "k", "up":
			if t.cursor > 0 {
				t.cursor--
			}
		case "/":
			t.filtering = true
		}
	}
	return t, nil
}

func (t *HooksTab) updateFilterInput(msg tea.KeyMsg) (tabModel, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEscape:
		t.filtering = false
		t.filter = ""
		t.applyFilter()
		t.cursor = 0
	case tea.KeyEnter:
		t.filtering = false
	case tea.KeyBackspace:
		if len(t.filter) > 0 {
			t.filter = t.filter[:len(t.filter)-1]
			t.applyFilter()
			t.cursor = 0
		}
	default:
		if msg.Type == tea.KeyRunes {
			t.filter += string(msg.Runes)
			t.applyFilter()
			t.cursor = 0
		}
	}
	return t, nil
}

// View implements tabModel.
func (t *HooksTab) View() string {
	var sb strings.Builder

	// Filter bar
	if t.filtering {
		sb.WriteString("Filter: " + t.filter + "_\n\n")
	} else if t.filter != "" {
		sb.WriteString("Filter: " + t.filter + "  (press / to change, Esc to clear)\n\n")
	} else {
		sb.WriteString("Filter: _  (/: search  j/k: navigate)\n\n")
	}

	if len(t.filtered) == 0 {
		sb.WriteString(styleHookType.Render("  (no hooks configured)"))
		return styleContent.Render(sb.String())
	}

	for i, row := range t.filtered {
		if row.isEvent {
			countStr := ""
			if row.count == 1 {
				countStr = "  [1 handler]"
			} else if row.count > 1 {
				countStr = "  [" + itoa(row.count) + " handlers]"
			}
			line := styleHookEventHeader.Render(row.eventName) + styleHookType.Render(countStr)
			if i == t.cursor {
				sb.WriteString(styleHookCursor.Render("> " + stripAnsi(line)) + "\n")
			} else {
				sb.WriteString("  " + line + "\n")
			}
			continue
		}

		// Entry row
		parts := []string{}
		if row.hookType != "" && row.hookType != "command" {
			parts = append(parts, styleHookType.Render("["+row.hookType+"]"))
		}
		if row.matcher != "" {
			parts = append(parts, styleHookType.Render("matcher:"+row.matcher))
		}
		if row.command != "" {
			parts = append(parts, row.command)
		}
		parts = append(parts, styleHookScopeBadge.Render("["+row.scope+"]"))

		line := "    " + strings.Join(parts, "  ")
		if i == t.cursor {
			sb.WriteString(styleHookCursor.Render("> " + strings.TrimLeft(line, " ")) + "\n")
		} else {
			sb.WriteString(line + "\n")
		}
	}

	return styleContent.Render(sb.String())
}

// Title implements tabModel.
func (t *HooksTab) Title() string {
	return "Hooks"
}

// stripAnsi is a minimal helper to remove lipgloss ANSI codes for cursor highlighting.
// Since we can't double-render styled text inside a cursor block, we render plain text.
func stripAnsi(s string) string {
	var b strings.Builder
	inEscape := false
	for _, r := range s {
		if r == '\x1b' {
			inEscape = true
			continue
		}
		if inEscape {
			if r == 'm' {
				inEscape = false
			}
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// itoa converts an int to string without importing strconv.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := false
	if n < 0 {
		neg = true
		n = -n
	}
	buf := make([]byte, 20)
	pos := len(buf)
	for n > 0 {
		pos--
		buf[pos] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		pos--
		buf[pos] = '-'
	}
	return string(buf[pos:])
}

