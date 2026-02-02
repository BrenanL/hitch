package generator

import (
	"strings"
	"testing"

	"github.com/BrenanL/hitch/internal/state"
)

func TestRuleToHookEntry(t *testing.T) {
	rule := state.Rule{
		ID:      "a1b2c3",
		DSL:     "on stop -> notify discord if elapsed > 30s",
		Scope:   "global",
		Enabled: true,
	}

	entry, err := RuleToHookEntry(rule, "ht")
	if err != nil {
		t.Fatalf("RuleToHookEntry: %v", err)
	}

	if entry.Event != "Stop" {
		t.Errorf("event = %q, want Stop", entry.Event)
	}
	if !strings.Contains(entry.Entry.Command, "ht hook exec a1b2c3") {
		t.Errorf("command = %q, missing rule ID", entry.Entry.Command)
	}
	if !strings.Contains(entry.Entry.Command, "# ht:rule-a1b2c3") {
		t.Errorf("command = %q, missing marker", entry.Entry.Command)
	}
	if entry.Entry.Type != "command" {
		t.Errorf("type = %q, want command", entry.Entry.Type)
	}
}

func TestRuleToHookEntryAsync(t *testing.T) {
	rule := state.Rule{
		ID:  "x1y2z3",
		DSL: `on post-edit -> run "npm test" async`,
	}

	entry, err := RuleToHookEntry(rule, "ht")
	if err != nil {
		t.Fatalf("RuleToHookEntry: %v", err)
	}

	if entry.Event != "PostToolUse" {
		t.Errorf("event = %q", entry.Event)
	}
	if entry.Matcher != "Edit|Write" {
		t.Errorf("matcher = %q", entry.Matcher)
	}
	// The hook entry itself is not async — async is for the run action within the hook
	// The hook entry command always runs ht hook exec synchronously
}

func TestRuleToHookEntryPreBash(t *testing.T) {
	rule := state.Rule{
		ID:  "d4e5f6",
		DSL: `on pre-bash -> deny if matches deny-list:destructive`,
	}

	entry, err := RuleToHookEntry(rule, "/usr/local/bin/ht")
	if err != nil {
		t.Fatalf("RuleToHookEntry: %v", err)
	}

	if entry.Event != "PreToolUse" {
		t.Errorf("event = %q", entry.Event)
	}
	if entry.Matcher != "Bash" {
		t.Errorf("matcher = %q", entry.Matcher)
	}
	if !strings.Contains(entry.Entry.Command, "/usr/local/bin/ht") {
		t.Errorf("command should use full path: %q", entry.Entry.Command)
	}
}

func TestSystemHooks(t *testing.T) {
	hooks := SystemHooks("ht")

	if len(hooks) != 2 {
		t.Fatalf("got %d system hooks, want 2", len(hooks))
	}

	// Session start
	if hooks[0].Event != "SessionStart" {
		t.Errorf("hook[0] event = %q", hooks[0].Event)
	}
	if !strings.Contains(hooks[0].Entry.Command, "# ht:system:session-start") {
		t.Errorf("hook[0] missing system marker: %q", hooks[0].Entry.Command)
	}

	// User prompt
	if hooks[1].Event != "UserPromptSubmit" {
		t.Errorf("hook[1] event = %q", hooks[1].Event)
	}
}
