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
