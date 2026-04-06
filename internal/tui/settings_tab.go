package tui

import (
	"encoding/json"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/BrenanL/hitch/pkg/settings"
)

// categoryDef groups schema keys into a named display category.
type categoryDef struct {
	name string
	keys []string
}

// settingsCategories defines the display order and key membership for each
// category shown in the Settings tab. Keys not listed fall into "Advanced".
var settingsCategories = []categoryDef{
	{
		name: "GENERAL",
		keys: []string{
			"effortLevel", "autoUpdatesChannel", "cleanupPeriodDays",
			"model", "language", "defaultShell", "alwaysThinkingEnabled",
			"outputStyle", "voiceEnabled", "feedbackSurveyRate", "companyAnnouncements",
		},
	},
	{
		name: "PERMISSIONS",
		keys: []string{
			"permissions", "disableBypassPermissionsMode",
		},
	},
	{
		name: "DISPLAY",
		keys: []string{
			"showThinkingSummaries", "spinnerTipsEnabled", "spinnerTipsOverride",
			"spinnerVerbs", "prefersReducedMotion", "statusLine",
		},
	},
	{
		name: "MEMORY & CONTEXT",
		keys: []string{
			"autoMemoryDirectory", "autoMode", "plansDirectory",
			"includeGitInstructions", "respectGitignore",
		},
	},
	{
		name: "UPDATES & AUTH",
		keys: []string{
			"apiKeyHelper", "awsCredentialExport", "awsAuthRefresh",
		},
	},
	{
		name: "SANDBOX",
		keys: []string{
			"sandbox",
		},
	},
	{
		name: "WORKTREES",
		keys: []string{
			"worktree.symlinkDirectories", "worktree.sparsePaths",
		},
	},
	{
		name: "HOOKS",
		keys: []string{
			"disableAllHooks", "hooks",
		},
	},
	{
		name: "MANAGED (read-only)",
		keys: []string{
			"allowManagedHooksOnly", "allowManagedPermissionRulesOnly",
			"allowManagedMcpServersOnly", "channelsEnabled",
			"forceLoginOrgUUID", "blockedMarketplaces", "strictKnownMarketplaces",
			"pluginTrustMessage", "forceRemoteSettingsRefresh", "forceLoginMethod",
			"allowedMcpServers", "allowedChannelPlugins", "deniedMcpServers",
		},
	},
}

// scopeBadge returns the single-letter scope badge and associated style.
func scopeBadge(s settings.Scope) string {
	switch s {
	case settings.ScopeUser:
		return "U"
	case settings.ScopeProject:
		return "P"
	case settings.ScopeLocal:
		return "L"
	case settings.ScopeManaged:
		return "M"
	default:
		return "?"
	}
}

var (
	styleScopeBadgeUser    = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))   // green
	styleScopeBadgeProject = lipgloss.NewStyle().Foreground(lipgloss.Color("12"))   // blue
	styleScopeBadgeLocal   = lipgloss.NewStyle().Foreground(lipgloss.Color("11"))   // yellow
	styleScopeBadgeManaged = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))    // red
	styleScopeDefault      = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))  // dim grey
	styleCategoryHeader    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("15"))
	styleCursor            = lipgloss.NewStyle().Foreground(lipgloss.Color("62"))
	styleKeyName           = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	styleValue             = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	styleFilterPrompt      = lipgloss.NewStyle().Foreground(lipgloss.Color("62"))
)

// rowKind distinguishes category headers from key rows.
type rowKind int

const (
	rowKindCategory rowKind = iota
	rowKindKey
)

// settingsRow is one display line in the Settings tab.
type settingsRow struct {
	kind         rowKind
	categoryName string     // for rowKindCategory
	keyName      string     // for rowKindKey
	hasValue     bool       // effective value found
	rawValue     string     // JSON-decoded display value (empty if unset)
	scope        settings.Scope
	isDefault    bool   // true when value comes from schema default, not any settings file
	managedOnly  bool
	def          settings.KeyDef
}

// SettingsTab is the Tab 1 implementation for the hitch TUI.
type SettingsTab struct {
	rows       []settingsRow
	cursor     int
	collapsed  map[string]bool // category name → collapsed
	filter     string
	filterMode bool
	err        error
}

// NewSettingsTab creates and loads a SettingsTab.
func NewSettingsTab() SettingsTab {
	t := SettingsTab{
		collapsed: make(map[string]bool),
	}
	t.load(".")
	return t
}

// load reads settings from disk and populates rows.
func (t *SettingsTab) load(projectDir string) {
	all, err := settings.LoadAll(projectDir)
	if err != nil {
		t.err = err
		return
	}
	effective := settings.Compute(all)
	t.rows = buildRows(effective)
}

// buildRows constructs the flat list of display rows from the effective settings.
func buildRows(effective *settings.EffectiveSettings) []settingsRow {
	schema := settings.Schema()

	// Build a lookup map: key name → KeyDef
	defs := make(map[string]settings.KeyDef, len(schema))
	for _, def := range schema {
		defs[def.Name] = def
	}

	// Track which keys have been assigned to a named category.
	assigned := make(map[string]bool)
	var rows []settingsRow

	for _, cat := range settingsCategories {
		// Category header row
		rows = append(rows, settingsRow{
			kind:         rowKindCategory,
			categoryName: cat.name,
		})
		for _, key := range cat.keys {
			def := defs[key]
			row := makeKeyRow(key, def, effective)
			rows = append(rows, row)
			assigned[key] = true
		}
	}

	// "Advanced" category: all keys not already assigned, excluding GlobalConfig.
	var advancedKeys []settingsRow
	for _, def := range schema {
		if assigned[def.Name] || def.GlobalConfig {
			continue
		}
		row := makeKeyRow(def.Name, def, effective)
		advancedKeys = append(advancedKeys, row)
	}
	if len(advancedKeys) > 0 {
		rows = append(rows, settingsRow{
			kind:         rowKindCategory,
			categoryName: "ADVANCED",
		})
		rows = append(rows, advancedKeys...)
	}

	return rows
}

// makeKeyRow creates a rowKindKey for the given key.
func makeKeyRow(key string, def settings.KeyDef, effective *settings.EffectiveSettings) settingsRow {
	row := settingsRow{
		kind:        rowKindKey,
		keyName:     key,
		def:         def,
		managedOnly: def.ManagedOnly,
	}

	raw, scope, found := effective.GetEffective(key)
	if found {
		row.hasValue = true
		row.scope = scope
		row.rawValue = prettyValue(raw)
	} else if def.Default != nil {
		row.hasValue = true
		row.isDefault = true
		row.rawValue = fmt.Sprintf("%v", def.Default)
	}

	return row
}

// prettyValue formats a raw JSON value for display (strips outer quotes for strings).
func prettyValue(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return fmt.Sprintf("%q", s)
	}
	var n float64
	if err := json.Unmarshal(raw, &n); err == nil {
		if n == float64(int64(n)) {
			return fmt.Sprintf("%d", int64(n))
		}
		return fmt.Sprintf("%g", n)
	}
	var b bool
	if err := json.Unmarshal(raw, &b); err == nil {
		if b {
			return "true"
		}
		return "false"
	}
	var arr []json.RawMessage
	if err := json.Unmarshal(raw, &arr); err == nil {
		return fmt.Sprintf("[%d items]", len(arr))
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err == nil {
		if len(obj) == 0 {
			return "{}"
		}
		return fmt.Sprintf("{%d keys}", len(obj))
	}
	return string(raw)
}

// visibleRows returns only the rows that should be shown given current filter
// and collapse state.
func (t *SettingsTab) visibleRows() []settingsRow {
	var result []settingsRow
	var currentCat string
	var catCollapsed bool

	for _, row := range t.rows {
		switch row.kind {
		case rowKindCategory:
			currentCat = row.categoryName
			catCollapsed = t.collapsed[currentCat]
			// Only include the category header if it has visible children (or always show it).
			if t.filter != "" {
				// Check whether any child matches; if not, skip header.
				if !t.categoryHasMatch(row.categoryName) {
					continue
				}
			}
			result = append(result, row)

		case rowKindKey:
			if catCollapsed {
				continue
			}
			if t.filter != "" && !strings.Contains(strings.ToLower(row.keyName), strings.ToLower(t.filter)) {
				continue
			}
			_ = currentCat
			result = append(result, row)
		}
	}
	return result
}

// categoryHasMatch returns true if the named category has at least one key
// matching the current filter.
func (t *SettingsTab) categoryHasMatch(catName string) bool {
	inCat := false
	for _, row := range t.rows {
		if row.kind == rowKindCategory {
			inCat = row.categoryName == catName
			continue
		}
		if inCat && strings.Contains(strings.ToLower(row.keyName), strings.ToLower(t.filter)) {
			return true
		}
	}
	return false
}

// Init implements tabModel.
func (t SettingsTab) Init() tea.Cmd {
	return nil
}

// Update implements tabModel.
func (t SettingsTab) Update(msg tea.Msg) (tabModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if t.filterMode {
			return t.handleFilterKey(msg)
		}
		return t.handleNavKey(msg)
	}
	return t, nil
}

func (t SettingsTab) handleFilterKey(msg tea.KeyMsg) (tabModel, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		t.filterMode = false
		t.filter = ""
		t.cursor = 0
		return t, nil
	case tea.KeyBackspace:
		if len(t.filter) > 0 {
			t.filter = t.filter[:len(t.filter)-1]
		}
		return t, nil
	case tea.KeyRunes:
		t.filter += string(msg.Runes)
		t.cursor = 0
		return t, nil
	}
	return t, nil
}

func (t SettingsTab) handleNavKey(msg tea.KeyMsg) (tabModel, tea.Cmd) {
	visible := t.visibleRows()
	switch msg.String() {
	case "j", "down":
		if t.cursor < len(visible)-1 {
			t.cursor++
		}
	case "k", "up":
		if t.cursor > 0 {
			t.cursor--
		}
	case "pgdown", "ctrl+d":
		t.cursor += 10
		if t.cursor >= len(visible) {
			t.cursor = len(visible) - 1
		}
		if t.cursor < 0 {
			t.cursor = 0
		}
	case "pgup", "ctrl+u":
		t.cursor -= 10
		if t.cursor < 0 {
			t.cursor = 0
		}
	case "g":
		t.cursor = 0
	case "G":
		if len(visible) > 0 {
			t.cursor = len(visible) - 1
		}
	case "/":
		t.filterMode = true
		t.filter = ""
	case "enter":
		if t.cursor < len(visible) {
			row := visible[t.cursor]
			if row.kind == rowKindCategory {
				t.collapsed[row.categoryName] = !t.collapsed[row.categoryName]
			}
		}
	}
	return t, nil
}

// View implements tabModel.
func (t SettingsTab) View() string {
	var sb strings.Builder

	// Filter bar.
	if t.filterMode {
		sb.WriteString(styleFilterPrompt.Render("Filter: ") + t.filter + "_\n\n")
	} else if t.filter != "" {
		sb.WriteString(styleFilterPrompt.Render("Filter: ") + styleValue.Render(t.filter) + "  (Esc to clear)\n\n")
	} else {
		sb.WriteString(styleValue.Render("/ to filter") + "\n\n")
	}

	if t.err != nil {
		sb.WriteString("Error loading settings: " + t.err.Error() + "\n")
		return styleContent.Render(sb.String())
	}

	visible := t.visibleRows()

	for i, row := range visible {
		cursor := "  "
		if i == t.cursor {
			cursor = styleCursor.Render("> ")
		}

		switch row.kind {
		case rowKindCategory:
			collapse := "v "
			if t.collapsed[row.categoryName] {
				collapse = "> "
			}
			line := cursor + collapse + styleCategoryHeader.Render(row.categoryName)
			sb.WriteString(line + "\n")

		case rowKindKey:
			keyPart := styleKeyName.Render(fmt.Sprintf("  %-34s", row.keyName))
			valuePart := formatValuePart(row)
			line := cursor + keyPart + valuePart
			sb.WriteString(line + "\n")
		}
	}

	if len(visible) == 0 && t.filter != "" {
		sb.WriteString(styleValue.Render("  (no matches)") + "\n")
	}

	return styleContent.Render(sb.String())
}

// formatValuePart formats the value and scope badge for a key row.
func formatValuePart(row settingsRow) string {
	var valuePart, badge string

	if !row.hasValue {
		valuePart = styleValue.Render(fmt.Sprintf("%-24s", "(unset)"))
		badge = styleValue.Render("—")
	} else if row.isDefault {
		valuePart = styleValue.Render(fmt.Sprintf("%-24s", row.rawValue))
		badge = styleScopeDefault.Render("(default)")
	} else {
		valuePart = styleValue.Render(fmt.Sprintf("%-24s", row.rawValue))
		letter := scopeBadge(row.scope)
		switch row.scope {
		case settings.ScopeUser:
			badge = styleScopeBadgeUser.Render("[" + letter + "]")
		case settings.ScopeProject:
			badge = styleScopeBadgeProject.Render("[" + letter + "]")
		case settings.ScopeLocal:
			badge = styleScopeBadgeLocal.Render("[" + letter + "]")
		case settings.ScopeManaged:
			badge = styleScopeBadgeManaged.Render("[" + letter + "]")
		}
	}

	return valuePart + "  " + badge
}

// Title implements tabModel.
func (t SettingsTab) Title() string {
	return "Settings"
}
