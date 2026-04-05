package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/BrenanL/hitch/pkg/envvars"
)

var (
	styleEnvVarSet = lipgloss.NewStyle().
			Foreground(lipgloss.Color("10")) // green

	styleEnvVarUnset = lipgloss.NewStyle().
				Foreground(lipgloss.Color("245")) // dim

	styleEnvVarDeprecated = lipgloss.NewStyle().
				Foreground(lipgloss.Color("240")) // darker dim

	styleEnvCategory = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("214")) // orange

	styleEnvCursor = lipgloss.NewStyle().
			Foreground(lipgloss.Color("15")).
			Background(lipgloss.Color("62"))

	styleEnvBadge = lipgloss.NewStyle().
			Foreground(lipgloss.Color("39")) // blue
)

// envVarRow represents a single displayable row in the env vars tab.
type envVarRow struct {
	isCategory bool
	category   string    // set if isCategory
	ev         envvars.EnvVar // set if not isCategory
	value      string    // current OS value, empty if unset
	isSet      bool
}

// EnvVarsTab implements tabModel for Tab 2 (Env Vars).
type EnvVarsTab struct {
	rows      []envVarRow
	filtered  []envVarRow
	cursor    int
	filter    string
	filtering bool
}

// NewEnvVarsTab creates an EnvVarsTab loaded with all env vars from the registry.
func NewEnvVarsTab() *EnvVarsTab {
	t := &EnvVarsTab{}
	t.load()
	t.applyFilter()
	return t
}

func (t *EnvVarsTab) load() {
	current := envvars.GetAllCurrent()
	categories := envvars.Categories()

	var rows []envVarRow
	for _, cat := range categories {
		rows = append(rows, envVarRow{isCategory: true, category: cat})
		vars := envvars.GetByCategory(cat)
		for _, ev := range vars {
			val, isSet := current[ev.Name]
			rows = append(rows, envVarRow{
				ev:    ev,
				value: val,
				isSet: isSet,
			})
		}
	}
	t.rows = rows
}

func (t *EnvVarsTab) applyFilter() {
	if t.filter == "" {
		t.filtered = make([]envVarRow, len(t.rows))
		copy(t.filtered, t.rows)
		return
	}
	q := strings.ToLower(t.filter)
	var result []envVarRow
	for _, r := range t.rows {
		if r.isCategory {
			// Include category header only if it has matching vars below.
			// We'll add it when we encounter matching vars.
			continue
		}
		if strings.Contains(strings.ToLower(r.ev.Name), q) ||
			strings.Contains(strings.ToLower(r.ev.Description), q) {
			result = append(result, r)
		}
	}
	t.filtered = result
}

// Init implements tabModel.
func (t *EnvVarsTab) Init() tea.Cmd {
	return nil
}

// Update implements tabModel.
func (t *EnvVarsTab) Update(msg tea.Msg) (tabModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if t.filtering {
			return t.updateFilterInput(msg)
		}
		switch msg.String() {
		case "j", "down":
			if t.cursor < len(t.filtered)-1 {
				t.cursor++
				// Skip category headers when navigating
				for t.cursor < len(t.filtered)-1 && t.filtered[t.cursor].isCategory {
					t.cursor++
				}
			}
		case "k", "up":
			if t.cursor > 0 {
				t.cursor--
				for t.cursor > 0 && t.filtered[t.cursor].isCategory {
					t.cursor--
				}
			}
		case "/":
			t.filtering = true
		}
	}
	return t, nil
}

func (t *EnvVarsTab) updateFilterInput(msg tea.KeyMsg) (tabModel, tea.Cmd) {
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
func (t *EnvVarsTab) View() string {
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
		sb.WriteString(styleEnvVarUnset.Render("  (no results)"))
		return styleContent.Render(sb.String())
	}

	for i, row := range t.filtered {
		if row.isCategory {
			sb.WriteString(styleEnvCategory.Render("  "+strings.ToUpper(row.category)) + "\n")
			continue
		}

		line := t.renderVarRow(row)
		if i == t.cursor {
			sb.WriteString(styleEnvCursor.Render("> " + line))
		} else {
			sb.WriteString("  " + line)
		}
		sb.WriteString("\n")
	}

	return styleContent.Render(sb.String())
}

func (t *EnvVarsTab) renderVarRow(row envVarRow) string {
	const nameWidth = 48

	name := row.ev.Name
	padded := name
	if len(name) < nameWidth {
		padded = name + strings.Repeat(" ", nameWidth-len(name))
	}

	var valueStr string
	if row.isSet {
		display := row.value
		if isSensitive(name) {
			display = "●●●●●●●●"
		}
		if row.ev.Deprecated {
			valueStr = styleEnvVarDeprecated.Render(padded) + "  " +
				styleEnvBadge.Render(display) + "  " +
				styleEnvVarDeprecated.Render("[deprecated]")
		} else {
			valueStr = styleEnvVarSet.Render(padded) + "  " +
				styleEnvBadge.Render(display) + "  " +
				styleEnvBadge.Render("[os-env]")
		}
	} else {
		defaultHint := ""
		if row.ev.Default != "" {
			defaultHint = "  default: " + row.ev.Default
		}
		if row.ev.Deprecated {
			valueStr = styleEnvVarDeprecated.Render(padded) + "  " +
				styleEnvVarDeprecated.Render("(unset)") + defaultHint
		} else {
			valueStr = styleEnvVarUnset.Render(padded) + "  " +
				styleEnvVarUnset.Render("(unset)") + defaultHint
		}
	}
	return valueStr
}

// Title implements tabModel.
func (t *EnvVarsTab) Title() string {
	return "Env Vars"
}

// isSensitive returns true if the var name suggests a sensitive value.
func isSensitive(name string) bool {
	lower := strings.ToLower(name)
	return strings.Contains(lower, "key") ||
		strings.Contains(lower, "token") ||
		strings.Contains(lower, "secret") ||
		strings.Contains(lower, "password")
}
