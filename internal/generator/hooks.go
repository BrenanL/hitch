package generator

import (
	"fmt"

	"github.com/BrenanL/hitch/internal/dsl"
	"github.com/BrenanL/hitch/internal/state"
)

// HookEntry represents a single hook entry in settings.json.
type HookEntry struct {
	Type    string `json:"type"`
	Command string `json:"command"`
	Async   bool   `json:"async,omitempty"`
}

// MatcherGroup represents a matcher group in settings.json.
type MatcherGroup struct {
	Matcher string      `json:"matcher"`
	Hooks   []HookEntry `json:"hooks"`
}

// RuleToHookEntry converts a stored rule to a settings.json hook entry.
func RuleToHookEntry(rule state.Rule, htBinary string) (*HookEntryInfo, error) {
	parsed, err := dsl.ParseRule(rule.DSL)
	if err != nil {
		return nil, fmt.Errorf("parsing rule %s: %w", rule.ID, err)
	}

	marker := fmt.Sprintf("# ht:rule-%s", rule.ID)
	command := fmt.Sprintf("%s hook exec %s %s", htBinary, rule.ID, marker)

	// Check if any action is async
	async := false
	for _, action := range parsed.Actions {
		if run, ok := action.(dsl.RunAction); ok && run.Async {
			async = true
		}
	}

	entry := &HookEntryInfo{
		Event:   parsed.Event.HookEvent,
		Matcher: parsed.Event.Matcher,
		Marker:  marker,
		Entry: HookEntry{
			Type:    "command",
			Command: command,
			Async:   async,
		},
	}

	return entry, nil
}

// HookEntryInfo contains a hook entry with its metadata.
type HookEntryInfo struct {
	Event   string    // Claude Code event name
	Matcher string    // matcher pattern
	Marker  string    // ownership marker
	Entry   HookEntry // the actual hook entry
}

// SystemHookEntry creates a system hook entry (e.g., session tracking).
func SystemHookEntry(name, event, matcher, htBinary string) *HookEntryInfo {
	marker := fmt.Sprintf("# ht:system:%s", name)
	command := fmt.Sprintf("%s hook exec system:%s %s", htBinary, name, marker)

	return &HookEntryInfo{
		Event:   event,
		Matcher: matcher,
		Marker:  marker,
		Entry: HookEntry{
			Type:    "command",
			Command: command,
		},
	}
}

// SystemHooks returns the set of system hooks that hitch always installs.
func SystemHooks(htBinary string) []*HookEntryInfo {
	return []*HookEntryInfo{
		SystemHookEntry("session-start", "SessionStart", "*", htBinary),
		SystemHookEntry("user-prompt", "UserPromptSubmit", "", htBinary),
	}
}
