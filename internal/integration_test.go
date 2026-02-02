package internal_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BrenanL/hitch/internal/adapters"
	"github.com/BrenanL/hitch/internal/dsl"
	"github.com/BrenanL/hitch/internal/engine"
	"github.com/BrenanL/hitch/internal/generator"
	"github.com/BrenanL/hitch/internal/state"
	"github.com/BrenanL/hitch/pkg/hookio"
)

// TestEndToEnd exercises the full pipeline:
// init DB → add channel → add rule → sync → simulate hook → verify output + logs.
func TestEndToEnd(t *testing.T) {
	// --- 1. Init database (in-memory for testing) ---
	db, err := state.OpenInMemory()
	if err != nil {
		t.Fatalf("opening in-memory DB: %v", err)
	}
	defer db.Close()

	// --- 2. Set up a mock adapter for testing notifications ---
	mockAdapter := &mockNotifyAdapter{name: "test-ntfy"}

	// --- 3. Add a channel to the database ---
	err = db.ChannelAdd(state.Channel{
		ID:      "test-ntfy",
		Adapter: "ntfy",
		Name:    "test-ntfy",
		Config:  `{"topic":"test-topic","server":"http://localhost:9999"}`,
		Enabled: true,
	})
	if err != nil {
		t.Fatalf("adding channel: %v", err)
	}

	// Verify channel was stored
	ch, err := db.ChannelGet("test-ntfy")
	if err != nil {
		t.Fatalf("getting channel: %v", err)
	}
	if ch == nil {
		t.Fatal("channel not found after add")
	}
	if ch.Adapter != "ntfy" {
		t.Errorf("channel adapter = %q, want %q", ch.Adapter, "ntfy")
	}

	// --- 4. Add rules ---
	// Rule 1: notify on stop
	rule1DSL := `on stop -> notify test-ntfy`
	err = db.RuleAdd(state.Rule{
		ID:      "r-stop-notify",
		DSL:     rule1DSL,
		Scope:   "global",
		Enabled: true,
	})
	if err != nil {
		t.Fatalf("adding rule 1: %v", err)
	}

	// Rule 2: deny destructive commands
	rule2DSL := `on pre-bash -> deny "blocked by safety rule" if command matches deny-list:destructive`
	err = db.RuleAdd(state.Rule{
		ID:      "r-deny-destructive",
		DSL:     rule2DSL,
		Scope:   "global",
		Enabled: true,
	})
	if err != nil {
		t.Fatalf("adding rule 2: %v", err)
	}

	// Rule 3: disabled rule (should not fire)
	err = db.RuleAdd(state.Rule{
		ID:      "r-disabled",
		DSL:     `on stop -> deny "should not fire"`,
		Scope:   "global",
		Enabled: false,
	})
	if err != nil {
		t.Fatalf("adding rule 3: %v", err)
	}

	// Verify rules are stored
	rules, err := db.RuleList()
	if err != nil {
		t.Fatalf("listing rules: %v", err)
	}
	if len(rules) != 3 {
		t.Fatalf("expected 3 rules, got %d", len(rules))
	}

	// --- 5. Test DSL parsing of our rules ---
	parsed1, err := dsl.ParseRule(rule1DSL)
	if err != nil {
		t.Fatalf("parsing rule 1 DSL: %v", err)
	}
	if parsed1.Event.HookEvent != "Stop" {
		t.Errorf("rule 1 event = %q, want %q", parsed1.Event.HookEvent, "Stop")
	}

	parsed2, err := dsl.ParseRule(rule2DSL)
	if err != nil {
		t.Fatalf("parsing rule 2 DSL: %v", err)
	}
	if parsed2.Event.HookEvent != "PreToolUse" {
		t.Errorf("rule 2 event = %q, want %q", parsed2.Event.HookEvent, "PreToolUse")
	}
	if parsed2.Event.Matcher != "Bash" {
		t.Errorf("rule 2 matcher = %q, want %q", parsed2.Event.Matcher, "Bash")
	}

	// --- 6. Test generator: rule → hook entry ---
	htBinary := "/usr/local/bin/ht"
	entry1, err := generator.RuleToHookEntry(state.Rule{
		ID:  "r-stop-notify",
		DSL: rule1DSL,
	}, htBinary)
	if err != nil {
		t.Fatalf("generating hook entry for rule 1: %v", err)
	}
	if entry1.Event != "Stop" {
		t.Errorf("entry 1 event = %q, want %q", entry1.Event, "Stop")
	}
	if !strings.Contains(entry1.Entry.Command, "r-stop-notify") {
		t.Errorf("entry 1 command should contain rule ID, got: %s", entry1.Entry.Command)
	}
	if !strings.Contains(entry1.Entry.Command, "# ht:rule-r-stop-notify") {
		t.Errorf("entry 1 command should contain marker, got: %s", entry1.Entry.Command)
	}

	// --- 7. Test settings.json sync ---
	tmpDir := t.TempDir()
	settingsPath := filepath.Join(tmpDir, "settings.json")
	manifestPath := filepath.Join(tmpDir, "manifest.json")

	// Write initial settings with a pre-existing non-hitch hook
	initialSettings := `{
		"theme": "dark",
		"hooks": {
			"Stop": [{
				"matcher": "",
				"hooks": [{"type": "command", "command": "echo goodbye"}]
			}]
		}
	}`
	if err := os.WriteFile(settingsPath, []byte(initialSettings), 0o644); err != nil {
		t.Fatalf("writing initial settings: %v", err)
	}

	// Read settings
	settings, err := generator.ReadSettings(settingsPath)
	if err != nil {
		t.Fatalf("reading settings: %v", err)
	}

	// Read manifest (empty on first run)
	manifest, err := generator.ReadManifest(manifestPath)
	if err != nil {
		t.Fatalf("reading manifest: %v", err)
	}

	// Generate entries for enabled rules
	enabledRules, _ := db.RuleList()
	var entries []*generator.HookEntryInfo
	// Add system hooks
	entries = append(entries, generator.SystemHooks(htBinary)...)
	// Add user rules
	for _, r := range enabledRules {
		if !r.Enabled {
			continue
		}
		e, err := generator.RuleToHookEntry(r, htBinary)
		if err != nil {
			t.Fatalf("generating entry for %s: %v", r.ID, err)
		}
		entries = append(entries, e)
	}

	// Merge and write
	generator.MergeHooks(settings, manifest, entries)
	generator.UpdateManifest(manifest, entries, "global", settingsPath)

	if err := generator.WriteSettings(settingsPath, settings); err != nil {
		t.Fatalf("writing settings: %v", err)
	}
	if err := generator.WriteManifest(manifestPath, manifest); err != nil {
		t.Fatalf("writing manifest: %v", err)
	}

	// Verify settings.json was written correctly
	settingsData, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("reading written settings: %v", err)
	}
	settingsStr := string(settingsData)

	// Non-hitch hook should be preserved
	if !strings.Contains(settingsStr, "echo goodbye") {
		t.Error("non-hitch hook was not preserved in settings.json")
	}
	// Hitch entries should be present
	if !strings.Contains(settingsStr, "ht:rule-r-stop-notify") {
		t.Error("rule r-stop-notify not found in settings.json")
	}
	if !strings.Contains(settingsStr, "ht:rule-r-deny-destructive") {
		t.Error("rule r-deny-destructive not found in settings.json")
	}
	if !strings.Contains(settingsStr, "ht:system:session-start") {
		t.Error("system hook session-start not found in settings.json")
	}
	// Disabled rule should NOT be in settings
	if strings.Contains(settingsStr, "r-disabled") {
		t.Error("disabled rule should not be in settings.json")
	}

	// --- 8. Re-sync is idempotent ---
	settings2, err := generator.ReadSettings(settingsPath)
	if err != nil {
		t.Fatalf("reading settings for re-sync: %v", err)
	}
	manifest2, err := generator.ReadManifest(manifestPath)
	if err != nil {
		t.Fatalf("reading manifest for re-sync: %v", err)
	}
	generator.MergeHooks(settings2, manifest2, entries)
	settingsData2, err := generator.MarshalSettings(settings2)
	if err != nil {
		t.Fatalf("marshaling re-synced settings: %v", err)
	}

	// Should still have exactly one instance of each marker (not duplicated)
	if strings.Count(string(settingsData2), "ht:rule-r-stop-notify") != 1 {
		t.Error("re-sync created duplicate entries for r-stop-notify")
	}
	if strings.Count(string(settingsData2), "ht:rule-r-deny-destructive") != 1 {
		t.Error("re-sync created duplicate entries for r-deny-destructive")
	}

	// --- 9. Simulate hook execution: system hooks ---
	exec := &engine.Executor{
		DB: db,
		GetAdapter: func(name string) (adapters.Adapter, error) {
			if name == "test-ntfy" {
				return mockAdapter, nil
			}
			return nil, fmt.Errorf("unknown adapter: %s", name)
		},
		DenyLists: engine.LoadDenyLists(),
	}

	// System hook: session-start
	sessionInput := &hookio.HookInput{
		SessionID:     "test-session-123",
		HookEventName: "SessionStart",
		Cwd:           "/home/user/project",
	}
	sysResult := exec.ExecuteSystemHook("session-start", sessionInput)
	if sysResult.Output == nil {
		t.Fatal("system hook returned nil output")
	}
	// System hooks should not block
	if sysResult.Blocked {
		t.Error("system hook should not block")
	}
	// Output should be an allow (empty output = allow in Claude Code protocol)
	if sysResult.Output.Decision == "deny" {
		t.Error("system hook should not deny")
	}

	// Verify session was created
	session, err := db.SessionGet("test-session-123")
	if err != nil {
		t.Fatalf("getting session: %v", err)
	}
	if session == nil {
		t.Fatal("session not created by system hook")
	}

	// System hook: user-prompt
	promptInput := &hookio.HookInput{
		SessionID:     "test-session-123",
		HookEventName: "UserPromptSubmit",
	}
	exec.ExecuteSystemHook("user-prompt", promptInput)

	// --- 10. Simulate hook execution: rule 1 (notify on stop) ---
	rule1, _ := db.RuleGet("r-stop-notify")
	stopInput := &hookio.HookInput{
		SessionID:     "test-session-123",
		HookEventName: "Stop",
	}
	result1 := exec.Execute(context.Background(), rule1, stopInput)
	if result1.Error != nil {
		t.Errorf("rule 1 execution error: %v", result1.Error)
	}
	if result1.Blocked {
		t.Error("rule 1 should not block")
	}
	// The mock adapter should have been called
	if !mockAdapter.sent {
		t.Error("notify adapter was not called for rule 1")
	}
	if mockAdapter.lastMsg.Event != "Stop" {
		t.Errorf("notify event = %q, want %q", mockAdapter.lastMsg.Event, "Stop")
	}

	// --- 11. Simulate hook execution: rule 2 (deny destructive) ---
	rule2, _ := db.RuleGet("r-deny-destructive")

	// Test with destructive command: rm -rf /
	destructiveInput := &hookio.HookInput{
		SessionID:     "test-session-123",
		HookEventName: "PreToolUse",
		ToolName:      "Bash",
		ToolInput:     json.RawMessage(`{"command":"rm -rf /"}`),
	}
	result2 := exec.Execute(context.Background(), rule2, destructiveInput)
	if !result2.Blocked {
		t.Error("destructive command should be blocked")
	}
	if result2.Output == nil {
		t.Fatal("blocked result should have output")
	}

	// Test with safe command: npm test
	safeInput := &hookio.HookInput{
		SessionID:     "test-session-123",
		HookEventName: "PreToolUse",
		ToolName:      "Bash",
		ToolInput:     json.RawMessage(`{"command":"npm test"}`),
	}
	result2safe := exec.Execute(context.Background(), rule2, safeInput)
	if result2safe.Blocked {
		t.Error("safe command should not be blocked")
	}

	// --- 12. Verify event logging ---
	events, err := db.EventQuery(state.EventFilter{
		SessionID: "test-session-123",
		Limit:     20,
	})
	if err != nil {
		t.Fatalf("querying events: %v", err)
	}
	if len(events) < 2 {
		t.Fatalf("expected at least 2 events, got %d", len(events))
	}

	// Check we have events for both rules
	var foundStopEvent, foundDenyEvent, foundSafeEvent bool
	for _, e := range events {
		if e.RuleID == "r-stop-notify" {
			foundStopEvent = true
		}
		if e.RuleID == "r-deny-destructive" && e.ActionTaken == "denied" {
			foundDenyEvent = true
		}
		if e.RuleID == "r-deny-destructive" && e.ActionTaken == "condition-false" {
			foundSafeEvent = true
		}
	}
	if !foundStopEvent {
		t.Error("no event logged for stop-notify rule")
	}
	if !foundDenyEvent {
		t.Error("no event logged for deny-destructive rule (blocked)")
	}
	if !foundSafeEvent {
		t.Error("no event logged for deny-destructive rule (allowed)")
	}

	// --- 13. Verify disabled rules don't fire ---
	disabledRule, _ := db.RuleGet("r-disabled")
	if disabledRule.Enabled {
		t.Error("disabled rule should report as disabled")
	}

	// --- 14. Test rule enable/disable ---
	if err := db.RuleDisable("r-stop-notify"); err != nil {
		t.Fatalf("disabling rule: %v", err)
	}
	r, _ := db.RuleGet("r-stop-notify")
	if r.Enabled {
		t.Error("rule should be disabled")
	}

	if err := db.RuleEnable("r-stop-notify"); err != nil {
		t.Fatalf("enabling rule: %v", err)
	}
	r, _ = db.RuleGet("r-stop-notify")
	if !r.Enabled {
		t.Error("rule should be re-enabled")
	}

	// --- 15. Test channel removal ---
	if err := db.ChannelRemove("test-ntfy"); err != nil {
		t.Fatalf("removing channel: %v", err)
	}
	ch, _ = db.ChannelGet("test-ntfy")
	if ch != nil {
		t.Error("channel should be nil after removal")
	}

	// --- 16. Test mute functionality ---
	muted, _ := db.IsMuted()
	if muted {
		t.Error("should not be muted by default")
	}
	if err := db.MuteSet("2099-01-01T00:00:00Z"); err != nil {
		t.Fatalf("setting mute: %v", err)
	}
	muted, _ = db.IsMuted()
	if !muted {
		t.Error("should be muted after MuteSet")
	}
	if err := db.MuteClear(); err != nil {
		t.Fatalf("clearing mute: %v", err)
	}
	muted, _ = db.IsMuted()
	if muted {
		t.Error("should not be muted after clear")
	}
}

// TestDenyListEndToEnd tests the deny list pipeline from embedded lists through condition evaluation.
func TestDenyListEndToEnd(t *testing.T) {
	lists := engine.LoadDenyLists()

	// Should have the built-in destructive list
	destructive, ok := lists["destructive"]
	if !ok {
		t.Fatal("destructive deny list not found")
	}
	if len(destructive) == 0 {
		t.Fatal("destructive deny list is empty")
	}

	// Test matching
	dangerousCommands := []string{
		"rm -rf /",
		"sudo rm -rf /var/data",
		"DROP DATABASE production",
		"git push --force origin main",
	}
	for _, cmd := range dangerousCommands {
		if !engine.MatchesDenyList(cmd, lists, "destructive") {
			t.Errorf("expected %q to match destructive deny list", cmd)
		}
	}

	safeCommands := []string{
		"npm test",
		"go build ./...",
		"git commit -m 'fix bug'",
		"echo hello",
	}
	for _, cmd := range safeCommands {
		if engine.MatchesDenyList(cmd, lists, "destructive") {
			t.Errorf("expected %q NOT to match destructive deny list", cmd)
		}
	}
}

// TestDSLRoundTrip tests parsing multiple rule formats and verifying the AST.
func TestDSLRoundTrip(t *testing.T) {
	cases := []struct {
		name       string
		dslInput   string
		wantEvent  string
		wantAction string
	}{
		{
			name:       "simple notify",
			dslInput:   `on stop -> notify slack`,
			wantEvent:  "Stop",
			wantAction: "notify",
		},
		{
			name:       "deny with condition",
			dslInput:   `on pre-bash -> deny "not allowed" if command matches "rm -rf"`,
			wantEvent:  "PreToolUse",
			wantAction: "deny",
		},
		{
			name:       "conditional notify",
			dslInput:   `on stop -> notify ntfy if elapsed > 30s`,
			wantEvent:  "Stop",
			wantAction: "notify",
		},
		{
			name:       "log action",
			dslInput:   `on pre-tool -> log`,
			wantEvent:  "PreToolUse",
			wantAction: "log",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rule, err := dsl.ParseRule(tc.dslInput)
			if err != nil {
				t.Fatalf("parsing %q: %v", tc.dslInput, err)
			}
			if rule.Event.HookEvent != tc.wantEvent {
				t.Errorf("event = %q, want %q", rule.Event.HookEvent, tc.wantEvent)
			}
			if len(rule.Actions) == 0 {
				t.Fatal("expected at least one action")
			}
			switch tc.wantAction {
			case "notify":
				if _, ok := rule.Actions[0].(dsl.NotifyAction); !ok {
					t.Errorf("expected NotifyAction, got %T", rule.Actions[0])
				}
			case "deny":
				if _, ok := rule.Actions[0].(dsl.DenyAction); !ok {
					t.Errorf("expected DenyAction, got %T", rule.Actions[0])
				}
			case "log":
				if _, ok := rule.Actions[0].(dsl.LogAction); !ok {
					t.Errorf("expected LogAction, got %T", rule.Actions[0])
				}
			}
		})
	}
}

// TestSettingsRoundTrip verifies that settings.json survives marshal/unmarshal.
func TestSettingsRoundTrip(t *testing.T) {
	original := `{
		"theme": "dark",
		"font_size": 14,
		"hooks": {
			"Stop": [{
				"matcher": "",
				"hooks": [{"type": "command", "command": "echo done"}]
			}]
		}
	}`

	settings, err := generator.ParseSettings([]byte(original))
	if err != nil {
		t.Fatalf("parsing settings: %v", err)
	}

	data, err := generator.MarshalSettings(settings)
	if err != nil {
		t.Fatalf("marshaling settings: %v", err)
	}

	result := string(data)
	if !strings.Contains(result, "dark") {
		t.Error("theme lost during round-trip")
	}
	if !strings.Contains(result, "14") {
		t.Error("font_size lost during round-trip")
	}
	if !strings.Contains(result, "echo done") {
		t.Error("hook command lost during round-trip")
	}
}

// mockNotifyAdapter is a test adapter that records calls.
type mockNotifyAdapter struct {
	name    string
	sent    bool
	lastMsg adapters.Message
}

func (m *mockNotifyAdapter) Name() string { return m.name }

func (m *mockNotifyAdapter) Send(_ context.Context, msg adapters.Message) adapters.SendResult {
	m.sent = true
	m.lastMsg = msg
	return adapters.SendResult{Success: true}
}

func (m *mockNotifyAdapter) Test(_ context.Context) adapters.SendResult {
	return adapters.SendResult{Success: true}
}

func (m *mockNotifyAdapter) ValidateConfig() error { return nil }
