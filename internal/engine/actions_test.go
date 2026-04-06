package engine

import (
	"context"
	"fmt"
	"testing"

	"github.com/BrenanL/hitch/internal/adapters"
	"github.com/BrenanL/hitch/internal/dsl"
	"github.com/BrenanL/hitch/internal/state"
	"github.com/BrenanL/hitch/pkg/hookio"
)

// mockAdapter is a test adapter that records sent messages.
type mockAdapter struct {
	name     string
	messages []adapters.Message
	fail     bool
}

func (m *mockAdapter) Name() string { return m.name }
func (m *mockAdapter) Send(ctx context.Context, msg adapters.Message) adapters.SendResult {
	m.messages = append(m.messages, msg)
	if m.fail {
		return adapters.SendResult{Error: fmt.Errorf("send failed")}
	}
	return adapters.SendResult{Success: true}
}
func (m *mockAdapter) Test(ctx context.Context) adapters.SendResult {
	return m.Send(ctx, adapters.Message{Title: "Test"})
}
func (m *mockAdapter) ValidateConfig() error { return nil }

func TestExecuteNotifyAction(t *testing.T) {
	mock := &mockAdapter{name: "discord"}

	actx := &ActionContext{
		GetAdapter: func(name string) (adapters.Adapter, error) { return mock, nil },
		SessionID:  "s1",
		HookEvent:  "Stop",
		RuleID:     "r1",
		Input:      &hookio.HookInput{HookEventName: "Stop"},
	}

	result := ExecuteAction(context.Background(), dsl.NotifyAction{Channel: "discord"}, actx)
	if result.Error != nil {
		t.Fatalf("ExecuteAction: %v", result.Error)
	}
	if result.ActionTaken != "notified:discord" {
		t.Errorf("action = %q", result.ActionTaken)
	}
	if len(mock.messages) != 1 {
		t.Errorf("messages = %d", len(mock.messages))
	}
}

func TestExecuteNotifyWithCustomMessage(t *testing.T) {
	mock := &mockAdapter{name: "ntfy"}

	actx := &ActionContext{
		GetAdapter: func(name string) (adapters.Adapter, error) { return mock, nil },
		HookEvent:  "Stop",
		Input:      &hookio.HookInput{},
	}

	ExecuteAction(context.Background(), dsl.NotifyAction{Channel: "ntfy", Message: "Custom msg"}, actx)
	if len(mock.messages) == 0 {
		t.Fatal("no messages sent")
	}
	if mock.messages[0].Body != "Custom msg" {
		t.Errorf("body = %q", mock.messages[0].Body)
	}
}

func TestExecuteNotifyAdapterNotFound(t *testing.T) {
	actx := &ActionContext{
		GetAdapter: func(name string) (adapters.Adapter, error) {
			return nil, fmt.Errorf("channel %q not found", name)
		},
		HookEvent: "Stop",
		Input:     &hookio.HookInput{},
	}

	result := ExecuteAction(context.Background(), dsl.NotifyAction{Channel: "missing"}, actx)
	if result.Error == nil {
		t.Error("expected error when adapter not found")
	}
	if result.ActionTaken != "notify-failed:missing" {
		t.Errorf("action = %q", result.ActionTaken)
	}
}

func TestExecuteNotifySendFailure(t *testing.T) {
	mock := &mockAdapter{name: "discord", fail: true}

	actx := &ActionContext{
		GetAdapter: func(name string) (adapters.Adapter, error) { return mock, nil },
		HookEvent:  "Stop",
		Input:      &hookio.HookInput{},
	}

	result := ExecuteAction(context.Background(), dsl.NotifyAction{Channel: "discord"}, actx)
	if result.Error == nil {
		t.Error("expected error when send fails")
	}
	if result.ActionTaken != "notify-failed:discord" {
		t.Errorf("action = %q", result.ActionTaken)
	}
}

func TestExecuteNotifyUpdatesLastUsed(t *testing.T) {
	db, err := state.OpenInMemory()
	if err != nil {
		t.Fatalf("OpenInMemory: %v", err)
	}
	defer db.Close()

	// Add a channel
	db.ChannelAdd(state.Channel{ID: "test-ch", Adapter: "ntfy", Name: "test", Config: "{}", Enabled: true})

	mock := &mockAdapter{name: "ntfy"}
	actx := &ActionContext{
		DB:         db,
		GetAdapter: func(name string) (adapters.Adapter, error) { return mock, nil },
		HookEvent:  "Stop",
		Input:      &hookio.HookInput{},
	}

	ExecuteAction(context.Background(), dsl.NotifyAction{Channel: "test-ch"}, actx)

	ch, _ := db.ChannelGet("test-ch")
	if ch.LastUsedAt == "" {
		t.Error("LastUsedAt should be set after successful notify")
	}
}

func TestExecuteDenyAction(t *testing.T) {
	result := ExecuteAction(context.Background(), dsl.DenyAction{Reason: "blocked"}, nil)
	if !result.ShouldBlock {
		t.Error("should block")
	}
	if result.Output == nil {
		t.Fatal("output is nil")
	}
	if result.Output.Decision != "deny" {
		t.Errorf("decision = %q", result.Output.Decision)
	}
	if result.ActionTaken != "denied" {
		t.Errorf("action = %q", result.ActionTaken)
	}
}

func TestExecuteDenyDefaultReason(t *testing.T) {
	result := ExecuteAction(context.Background(), dsl.DenyAction{}, nil)
	if result.Output.Reason != "blocked by hitch rule" {
		t.Errorf("reason = %q", result.Output.Reason)
	}
}

func TestExecuteSummarizeAction(t *testing.T) {
	actx := &ActionContext{HookEvent: "Stop"}
	result := ExecuteAction(context.Background(), dsl.SummarizeAction{}, actx)
	if result.ActionTaken != "summarized" {
		t.Errorf("action = %q", result.ActionTaken)
	}
}

func TestExecuteLogAction(t *testing.T) {
	actx := &ActionContext{
		HookEvent: "Stop",
		Input:     &hookio.HookInput{},
	}
	result := ExecuteAction(context.Background(), dsl.LogAction{}, actx)
	if result.ActionTaken != "logged" {
		t.Errorf("action = %q", result.ActionTaken)
	}
}

func TestExecuteLogActionWithDB(t *testing.T) {
	db, err := state.OpenInMemory()
	if err != nil {
		t.Fatalf("OpenInMemory: %v", err)
	}
	defer db.Close()

	actx := &ActionContext{
		DB:        db,
		SessionID: "log-sess",
		HookEvent: "PreToolUse",
		RuleID:    "log-rule",
		Input:     &hookio.HookInput{ToolName: "Bash"},
	}

	ExecuteAction(context.Background(), dsl.LogAction{}, actx)

	events, _ := db.EventQuery(state.EventFilter{SessionID: "log-sess"})
	if len(events) == 0 {
		t.Error("log action should write an event to the database")
	}
}

func TestExecuteInjectContextAction(t *testing.T) {
	actx := &ActionContext{
		HookEvent: "InstructionsLoaded",
		Input:     &hookio.HookInput{},
	}

	result := ExecuteAction(context.Background(), dsl.InjectContextAction{Text: "hello world"}, actx)
	if result.Error != nil {
		t.Fatalf("ExecuteAction: %v", result.Error)
	}
	if result.ActionTaken != "inject-context" {
		t.Errorf("action = %q, want inject-context", result.ActionTaken)
	}
	if result.Output == nil {
		t.Fatal("output is nil")
	}
	if result.Output.AdditionalContext != "hello world" {
		t.Errorf("AdditionalContext = %q, want %q", result.Output.AdditionalContext, "hello world")
	}
}

func TestExecuteSwitchProfileAction(t *testing.T) {
	actx := &ActionContext{
		HookEvent: "SubagentStart",
		Input:     &hookio.HookInput{},
	}

	result := ExecuteAction(context.Background(), dsl.SwitchProfileAction{Profile: "conservative"}, actx)
	if result.Error != nil {
		t.Fatalf("ExecuteAction: %v", result.Error)
	}
	if result.ActionTaken != "switch-profile:conservative" {
		t.Errorf("action = %q, want switch-profile:conservative", result.ActionTaken)
	}
}

func TestExecutePruneActionNoPruner(t *testing.T) {
	actx := &ActionContext{
		HookEvent:  "PreCompact",
		Input:      &hookio.HookInput{},
		PrunerFunc: nil,
	}

	result := ExecuteAction(context.Background(), dsl.PruneAction{Tier: "gentle"}, actx)
	if result.Error != nil {
		t.Fatalf("ExecuteAction: %v", result.Error)
	}
	if result.ActionTaken != "prune-noop" {
		t.Errorf("action = %q, want prune-noop", result.ActionTaken)
	}
}

func TestExecutePruneActionWithPruner(t *testing.T) {
	var gotTier string
	actx := &ActionContext{
		HookEvent: "PreCompact",
		Input:     &hookio.HookInput{},
		PrunerFunc: func(tier string) error {
			gotTier = tier
			return nil
		},
	}

	result := ExecuteAction(context.Background(), dsl.PruneAction{Tier: "standard"}, actx)
	if result.Error != nil {
		t.Fatalf("ExecuteAction: %v", result.Error)
	}
	if result.ActionTaken != "prune:standard" {
		t.Errorf("action = %q, want prune:standard", result.ActionTaken)
	}
	if gotTier != "standard" {
		t.Errorf("tier passed to PrunerFunc = %q, want %q", gotTier, "standard")
	}
}

// TestExecuteDenyTeammateIdleOutput verifies that deny on a TeammateIdle event
// produces {"continue": false, "stopReason": "..."} instead of {"decision": "deny"}.
// This is the blocking format required for teammate/task events per spec section 7.4.
func TestExecuteDenyTeammateIdleOutput(t *testing.T) {
	actx := &ActionContext{
		HookEvent: "TeammateIdle",
		Input:     &hookio.HookInput{HookEventName: "TeammateIdle"},
	}

	result := ExecuteAction(context.Background(), dsl.DenyAction{Reason: "too many tasks"}, actx)
	if !result.ShouldBlock {
		t.Error("expected ShouldBlock = true")
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
	if result.Output.StopReason != "too many tasks" {
		t.Errorf("stopReason = %q, want %q", result.Output.StopReason, "too many tasks")
	}
}

// TestExecuteDenyTaskCreatedOutput verifies that deny on TaskCreated also uses StopTeammate format.
func TestExecuteDenyTaskCreatedOutput(t *testing.T) {
	actx := &ActionContext{
		HookEvent: "TaskCreated",
		Input:     &hookio.HookInput{HookEventName: "TaskCreated"},
	}

	result := ExecuteAction(context.Background(), dsl.DenyAction{Reason: "task not allowed"}, actx)
	if !result.ShouldBlock {
		t.Error("expected ShouldBlock = true")
	}
	if result.Output == nil {
		t.Fatal("output is nil")
	}
	if result.Output.Decision != "" {
		t.Errorf("TaskCreated deny should not set decision field, got %q", result.Output.Decision)
	}
	if result.Output.Continue == nil || *result.Output.Continue != false {
		t.Error("expected continue = false for TaskCreated deny")
	}
}

// TestExecuteDenyStandardEventUsesDecision verifies that deny on a standard blocking event
// (e.g., PreToolUse) still produces the {"decision": "deny"} format.
func TestExecuteDenyStandardEventUsesDecision(t *testing.T) {
	actx := &ActionContext{
		HookEvent: "PreToolUse",
		Input:     &hookio.HookInput{HookEventName: "PreToolUse"},
	}

	result := ExecuteAction(context.Background(), dsl.DenyAction{Reason: "blocked"}, actx)
	if !result.ShouldBlock {
		t.Error("expected ShouldBlock = true")
	}
	if result.Output == nil {
		t.Fatal("output is nil")
	}
	if result.Output.Decision != "deny" {
		t.Errorf("decision = %q, want %q", result.Output.Decision, "deny")
	}
}
