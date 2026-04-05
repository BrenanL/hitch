package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/BrenanL/hitch/internal/adapters"
	"github.com/BrenanL/hitch/internal/state"
	"github.com/BrenanL/hitch/pkg/hookio"
)

func TestExecutorDenyRule(t *testing.T) {
	db, err := state.OpenInMemory()
	if err != nil {
		t.Fatalf("OpenInMemory: %v", err)
	}
	defer db.Close()

	executor := &Executor{
		DB: db,
		DenyLists: map[string][]string{
			"destructive": {"rm -rf /", "DROP DATABASE"},
		},
	}

	rule := &state.Rule{
		ID:  "d4e5f6",
		DSL: `on pre-bash -> deny if matches deny-list:destructive`,
	}

	input := &hookio.HookInput{
		SessionID:     "test-sess",
		HookEventName: "PreToolUse",
		ToolName:      "Bash",
		ToolInput:     json.RawMessage(`{"command": "rm -rf /"}`),
	}

	result := executor.Execute(context.Background(), rule, input)
	if !result.Blocked {
		t.Error("expected blocked")
	}
	if result.Output == nil || result.Output.Decision != "deny" {
		t.Error("expected deny output")
	}
}

func TestExecutorAllowRule(t *testing.T) {
	db, err := state.OpenInMemory()
	if err != nil {
		t.Fatalf("OpenInMemory: %v", err)
	}
	defer db.Close()

	executor := &Executor{
		DB: db,
		DenyLists: map[string][]string{
			"destructive": {"rm -rf /"},
		},
	}

	rule := &state.Rule{
		ID:  "d4e5f6",
		DSL: `on pre-bash -> deny if matches deny-list:destructive`,
	}

	input := &hookio.HookInput{
		SessionID:     "test-sess",
		HookEventName: "PreToolUse",
		ToolName:      "Bash",
		ToolInput:     json.RawMessage(`{"command": "npm test"}`),
	}

	result := executor.Execute(context.Background(), rule, input)
	if result.Blocked {
		t.Error("should not be blocked")
	}
}

func TestExecutorNotifyRule(t *testing.T) {
	db, err := state.OpenInMemory()
	if err != nil {
		t.Fatalf("OpenInMemory: %v", err)
	}
	defer db.Close()

	mock := &mockAdapter{name: "discord"}

	executor := &Executor{
		DB: db,
		GetAdapter: func(name string) (adapters.Adapter, error) {
			return mock, nil
		},
	}

	// Set session start time 45s ago
	db.KVSet("session_start:s1", "2020-01-01T00:00:00Z", "s1", "")

	rule := &state.Rule{
		ID:  "a1b2c3",
		DSL: `on stop -> notify discord if elapsed > 30s`,
	}

	input := &hookio.HookInput{
		SessionID:     "s1",
		HookEventName: "Stop",
	}

	result := executor.Execute(context.Background(), rule, input)
	if result.Error != nil {
		t.Errorf("error: %v", result.Error)
	}
	if len(mock.messages) != 1 {
		t.Errorf("messages = %d, want 1", len(mock.messages))
	}
}

func TestExecutorConditionFalse(t *testing.T) {
	db, err := state.OpenInMemory()
	if err != nil {
		t.Fatalf("OpenInMemory: %v", err)
	}
	defer db.Close()

	mock := &mockAdapter{name: "discord"}

	executor := &Executor{
		DB: db,
		GetAdapter: func(name string) (adapters.Adapter, error) {
			return mock, nil
		},
	}

	// Session just started (elapsed < 30s)
	db.KVSet("session_start:s1", "2099-01-01T00:00:00Z", "s1", "")

	rule := &state.Rule{
		ID:  "a1b2c3",
		DSL: `on stop -> notify discord if elapsed > 30s`,
	}

	input := &hookio.HookInput{
		SessionID:     "s1",
		HookEventName: "Stop",
	}

	result := executor.Execute(context.Background(), rule, input)
	if len(mock.messages) != 0 {
		t.Error("should not notify when condition is false")
	}
	if result.Blocked {
		t.Error("should not block when condition is false")
	}
}

func TestExecutorSystemHookSessionStart(t *testing.T) {
	db, err := state.OpenInMemory()
	if err != nil {
		t.Fatalf("OpenInMemory: %v", err)
	}
	defer db.Close()

	executor := &Executor{DB: db}

	input := &hookio.HookInput{
		SessionID:     "sess-123",
		HookEventName: "SessionStart",
		Cwd:           "/home/user/project",
	}

	result := executor.ExecuteSystemHook("session-start", input)
	if result.Error != nil {
		t.Fatalf("error: %v", result.Error)
	}

	// Verify session was created
	sess, err := db.SessionGet("sess-123")
	if err != nil {
		t.Fatalf("SessionGet: %v", err)
	}
	if sess == nil {
		t.Fatal("session not created")
	}
	if sess.ProjectDir != "/home/user/project" {
		t.Errorf("project_dir = %q", sess.ProjectDir)
	}

	// Verify kv was set
	val, _ := db.KVGet("session_start:sess-123", "sess-123")
	if val == "" {
		t.Error("session_start kv not set")
	}
}

func TestExecutorChainedActions(t *testing.T) {
	db, err := state.OpenInMemory()
	if err != nil {
		t.Fatalf("OpenInMemory: %v", err)
	}
	defer db.Close()

	mock := &mockAdapter{name: "slack"}

	executor := &Executor{
		DB: db,
		GetAdapter: func(name string) (adapters.Adapter, error) {
			return mock, nil
		},
	}

	rule := &state.Rule{
		ID:  "chain1",
		DSL: `on stop -> summarize -> notify slack`,
	}

	input := &hookio.HookInput{
		SessionID:     "s1",
		HookEventName: "Stop",
	}

	result := executor.Execute(context.Background(), rule, input)
	if result.Error != nil {
		t.Errorf("error: %v", result.Error)
	}
	if len(result.ActionsTaken) != 2 {
		t.Errorf("actions = %v, want 2", result.ActionsTaken)
	}
}

// --- Error path tests ---

func TestExecutorCorruptedDSL(t *testing.T) {
	db, err := state.OpenInMemory()
	if err != nil {
		t.Fatalf("OpenInMemory: %v", err)
	}
	defer db.Close()

	executor := &Executor{DB: db}

	rule := &state.Rule{
		ID:  "corrupt1",
		DSL: "this is not valid DSL at all",
	}

	input := &hookio.HookInput{
		SessionID:     "s1",
		HookEventName: "Stop",
	}

	result := executor.Execute(context.Background(), rule, input)
	if result.Error == nil {
		t.Error("expected error for corrupted DSL")
	}
	// Should not panic, should not block
	if result.Blocked {
		t.Error("corrupted DSL should not result in a block")
	}
}

func TestExecutorAdapterFailure(t *testing.T) {
	db, err := state.OpenInMemory()
	if err != nil {
		t.Fatalf("OpenInMemory: %v", err)
	}
	defer db.Close()

	executor := &Executor{
		DB: db,
		GetAdapter: func(name string) (adapters.Adapter, error) {
			return nil, fmt.Errorf("channel %q not found", name)
		},
	}

	rule := &state.Rule{
		ID:  "notify-fail",
		DSL: `on stop -> notify nonexistent-channel`,
	}

	input := &hookio.HookInput{
		SessionID:     "s1",
		HookEventName: "Stop",
	}

	result := executor.Execute(context.Background(), rule, input)
	// Should report error but not crash
	if result.Error == nil {
		t.Error("expected error when adapter is not found")
	}
	// Should not block (notify failure is not a block)
	if result.Blocked {
		t.Error("adapter failure should not block")
	}
}

func TestExecutorNilDB(t *testing.T) {
	executor := &Executor{
		DB:        nil,
		DenyLists: map[string][]string{},
	}

	rule := &state.Rule{
		ID:  "nodb",
		DSL: `on stop -> log`,
	}

	input := &hookio.HookInput{
		SessionID:     "s1",
		HookEventName: "Stop",
	}

	// Should not panic with nil DB
	result := executor.Execute(context.Background(), rule, input)
	if result.Blocked {
		t.Error("should not block with nil DB")
	}
}

func TestExecutorSystemHookNilDB(t *testing.T) {
	executor := &Executor{DB: nil}

	input := &hookio.HookInput{
		SessionID:     "s1",
		HookEventName: "SessionStart",
	}

	// Should not panic
	result := executor.ExecuteSystemHook("session-start", input)
	if result.Output == nil {
		t.Error("should return non-nil output even with nil DB")
	}
}

func TestExecutorLogsEvent(t *testing.T) {
	db, err := state.OpenInMemory()
	if err != nil {
		t.Fatalf("OpenInMemory: %v", err)
	}
	defer db.Close()

	executor := &Executor{
		DB:        db,
		DenyLists: map[string][]string{},
	}

	rule := &state.Rule{
		ID:  "log-test",
		DSL: `on stop -> log`,
	}

	input := &hookio.HookInput{
		SessionID:     "log-session",
		HookEventName: "Stop",
	}

	executor.Execute(context.Background(), rule, input)

	// Verify event was logged
	events, err := db.EventQuery(state.EventFilter{SessionID: "log-session"})
	if err != nil {
		t.Fatalf("EventQuery: %v", err)
	}
	if len(events) == 0 {
		t.Error("expected at least one event to be logged")
	}
	found := false
	for _, e := range events {
		if e.RuleID == "log-test" {
			found = true
		}
	}
	if !found {
		t.Error("event for rule 'log-test' not found")
	}
}

func TestExecutorDenyWithReason(t *testing.T) {
	db, err := state.OpenInMemory()
	if err != nil {
		t.Fatalf("OpenInMemory: %v", err)
	}
	defer db.Close()

	executor := &Executor{DB: db}

	rule := &state.Rule{
		ID:  "deny-reason",
		DSL: `on pre-bash -> deny "custom reason"`,
	}

	input := &hookio.HookInput{
		SessionID:     "s1",
		HookEventName: "PreToolUse",
		ToolName:      "Bash",
		ToolInput:     json.RawMessage(`{"command":"anything"}`),
	}

	result := executor.Execute(context.Background(), rule, input)
	if !result.Blocked {
		t.Error("should be blocked")
	}
	if result.Output.Reason != "custom reason" {
		t.Errorf("reason = %q, want %q", result.Output.Reason, "custom reason")
	}
}

func TestExecutorInjectContextSetsAdditionalContext(t *testing.T) {
	db, err := state.OpenInMemory()
	if err != nil {
		t.Fatalf("OpenInMemory: %v", err)
	}
	defer db.Close()

	executor := &Executor{DB: db}

	rule := &state.Rule{
		ID:  "inject1",
		DSL: `on stop -> inject_context "context text"`,
	}

	input := &hookio.HookInput{
		SessionID:     "s1",
		HookEventName: "Stop",
	}

	result := executor.Execute(context.Background(), rule, input)
	if result.Error != nil {
		t.Fatalf("Execute: %v", result.Error)
	}
	if result.Output == nil {
		t.Fatal("output is nil")
	}
	if result.Output.AdditionalContext != "context text" {
		t.Errorf("AdditionalContext = %q, want %q", result.Output.AdditionalContext, "context text")
	}
}

func TestExecutorInjectContextChaining(t *testing.T) {
	db, err := state.OpenInMemory()
	if err != nil {
		t.Fatalf("OpenInMemory: %v", err)
	}
	defer db.Close()

	executor := &Executor{DB: db}

	rule := &state.Rule{
		ID:  "inject2",
		DSL: `on stop -> inject_context "first" -> inject_context "second"`,
	}

	input := &hookio.HookInput{
		SessionID:     "s1",
		HookEventName: "Stop",
	}

	result := executor.Execute(context.Background(), rule, input)
	if result.Error != nil {
		t.Fatalf("Execute: %v", result.Error)
	}
	if result.Output == nil {
		t.Fatal("output is nil")
	}
	want := "first\nsecond"
	if result.Output.AdditionalContext != want {
		t.Errorf("AdditionalContext = %q, want %q", result.Output.AdditionalContext, want)
	}
}

// TestExecutorWorktreeCreatePassthrough verifies that a WorktreeCreate rule with no deny
// echoes back the worktree_path in hookSpecificOutput (pass-through behavior per spec section 7.5).
func TestExecutorWorktreeCreatePassthrough(t *testing.T) {
	db, err := state.OpenInMemory()
	if err != nil {
		t.Fatalf("OpenInMemory: %v", err)
	}
	defer db.Close()

	executor := &Executor{DB: db}

	rule := &state.Rule{
		ID:  "wt-log",
		DSL: `on worktree-create -> log`,
	}

	input := &hookio.HookInput{
		SessionID:     "s1",
		HookEventName: "WorktreeCreate",
		WorktreePath:  "/proj/.claude/worktrees/feature",
	}

	result := executor.Execute(context.Background(), rule, input)
	if result.Error != nil {
		t.Fatalf("Execute: %v", result.Error)
	}
	if result.Blocked {
		t.Error("should not be blocked for log action")
	}
	if result.Output == nil {
		t.Fatal("output is nil")
	}
	hso := result.Output.HookSpecificOutput
	if hso == nil {
		t.Fatal("HookSpecificOutput is nil — WorktreeCreate pass-through not set")
	}
	if hso["worktree_path"] != "/proj/.claude/worktrees/feature" {
		t.Errorf("worktree_path = %v, want %q", hso["worktree_path"], "/proj/.claude/worktrees/feature")
	}
}

// TestExecutorWorktreeCreateDenySkipsPassthrough verifies that a denied WorktreeCreate
// does NOT apply the pass-through (the deny output takes precedence).
func TestExecutorWorktreeCreateDenySkipsPassthrough(t *testing.T) {
	db, err := state.OpenInMemory()
	if err != nil {
		t.Fatalf("OpenInMemory: %v", err)
	}
	defer db.Close()

	executor := &Executor{DB: db}

	rule := &state.Rule{
		ID:  "wt-deny",
		DSL: `on worktree-create -> deny "not allowed"`,
	}

	input := &hookio.HookInput{
		SessionID:     "s1",
		HookEventName: "WorktreeCreate",
		WorktreePath:  "/proj/.claude/worktrees/feature",
	}

	result := executor.Execute(context.Background(), rule, input)
	if !result.Blocked {
		t.Error("expected blocked for deny action")
	}
	if result.Output == nil {
		t.Fatal("output is nil")
	}
	if result.Output.Decision != "deny" {
		t.Errorf("decision = %q, want %q", result.Output.Decision, "deny")
	}
	// Should not have worktree passthrough when blocked
	if result.Output.HookSpecificOutput != nil {
		t.Error("HookSpecificOutput should be nil when blocked")
	}
}

// TestExecutorTeammateIdleDenyOutput verifies that deny on teammate-idle produces
// {"continue": false, "stopReason": "..."} — not {"decision": "deny"} — per spec section 7.4.
func TestExecutorTeammateIdleDenyOutput(t *testing.T) {
	db, err := state.OpenInMemory()
	if err != nil {
		t.Fatalf("OpenInMemory: %v", err)
	}
	defer db.Close()

	executor := &Executor{DB: db}

	rule := &state.Rule{
		ID:  "idle-deny",
		DSL: `on teammate-idle -> deny "stop idle teammate"`,
	}

	input := &hookio.HookInput{
		SessionID:     "s1",
		HookEventName: "TeammateIdle",
	}

	result := executor.Execute(context.Background(), rule, input)
	if !result.Blocked {
		t.Error("expected blocked")
	}
	if result.Output == nil {
		t.Fatal("output is nil")
	}
	if result.Output.Decision != "" {
		t.Errorf("TeammateIdle deny should not set decision field, got %q", result.Output.Decision)
	}
	if result.Output.Continue == nil || *result.Output.Continue != false {
		t.Error("expected continue = false for TeammateIdle deny")
	}
	if result.Output.StopReason != "stop idle teammate" {
		t.Errorf("stopReason = %q, want %q", result.Output.StopReason, "stop idle teammate")
	}
}
