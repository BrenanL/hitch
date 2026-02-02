package engine

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/BrenanL/hitch/internal/dsl"
	"github.com/BrenanL/hitch/pkg/hookio"
)

func TestEvalElapsed(t *testing.T) {
	ctx := &EvalContext{
		SessionStart: time.Now().Add(-45 * time.Second),
		Now:          time.Now(),
	}

	// elapsed > 30s — should be true (45s > 30s)
	cond := dsl.ElapsedCondition{Op: ">", Duration: 30 * time.Second}
	if !EvalCondition(cond, ctx) {
		t.Error("expected true for elapsed > 30s when 45s elapsed")
	}

	// elapsed > 60s — should be false (45s < 60s)
	cond = dsl.ElapsedCondition{Op: ">", Duration: 60 * time.Second}
	if EvalCondition(cond, ctx) {
		t.Error("expected false for elapsed > 60s when 45s elapsed")
	}

	// elapsed < 60s — should be true
	cond = dsl.ElapsedCondition{Op: "<", Duration: 60 * time.Second}
	if !EvalCondition(cond, ctx) {
		t.Error("expected true for elapsed < 60s when 45s elapsed")
	}
}

func TestEvalElapsedNoSession(t *testing.T) {
	ctx := &EvalContext{Now: time.Now()}
	cond := dsl.ElapsedCondition{Op: ">", Duration: 30 * time.Second}
	if EvalCondition(cond, ctx) {
		t.Error("expected false when session start is zero")
	}
}

func TestEvalMatchCommand(t *testing.T) {
	ctx := &EvalContext{
		Input: &hookio.HookInput{
			ToolName:  "Bash",
			ToolInput: json.RawMessage(`{"command": "rm -rf /tmp/test"}`),
		},
	}

	// Should match
	cond := dsl.MatchCondition{Kind: "command", Pattern: "rm -rf"}
	if !EvalCondition(cond, ctx) {
		t.Error("expected match for 'rm -rf'")
	}

	// Should not match
	cond = dsl.MatchCondition{Kind: "command", Pattern: "^npm"}
	if EvalCondition(cond, ctx) {
		t.Error("expected no match for '^npm'")
	}
}

func TestEvalMatchFile(t *testing.T) {
	ctx := &EvalContext{
		Input: &hookio.HookInput{
			ToolName:  "Edit",
			ToolInput: json.RawMessage(`{"file_path": "/home/user/.env"}`),
		},
	}

	cond := dsl.MatchCondition{Kind: "file", Pattern: "\\.env$"}
	if !EvalCondition(cond, ctx) {
		t.Error("expected match for .env file")
	}
}

func TestEvalMatchDefaultBash(t *testing.T) {
	ctx := &EvalContext{
		Input: &hookio.HookInput{
			ToolName:  "Bash",
			ToolInput: json.RawMessage(`{"command": "npm test"}`),
		},
	}

	// Default kind "" should use command for Bash
	cond := dsl.MatchCondition{Kind: "", Pattern: "npm"}
	if !EvalCondition(cond, ctx) {
		t.Error("expected default match against command for Bash tool")
	}
}

func TestEvalDenyList(t *testing.T) {
	ctx := &EvalContext{
		Input: &hookio.HookInput{
			ToolName:  "Bash",
			ToolInput: json.RawMessage(`{"command": "rm -rf /"}`),
		},
		DenyLists: map[string][]string{
			"destructive": {"rm -rf /", "DROP DATABASE", "mkfs"},
		},
	}

	cond := dsl.MatchCondition{Kind: "", Pattern: "destructive", IsDenyList: true}
	if !EvalCondition(cond, ctx) {
		t.Error("expected deny list match for 'rm -rf /'")
	}

	// Safe command
	ctx.Input.ToolInput = json.RawMessage(`{"command": "npm test"}`)
	if EvalCondition(cond, ctx) {
		t.Error("expected no deny list match for 'npm test'")
	}
}

func TestEvalNot(t *testing.T) {
	ctx := &EvalContext{
		SessionStart: time.Now().Add(-45 * time.Second),
		Now:          time.Now(),
	}

	cond := dsl.NotCondition{
		Cond: dsl.ElapsedCondition{Op: ">", Duration: 60 * time.Second},
	}
	if !EvalCondition(cond, ctx) {
		t.Error("expected true for NOT (elapsed > 60s) when 45s elapsed")
	}
}

func TestEvalAnd(t *testing.T) {
	ctx := &EvalContext{
		SessionStart: time.Now().Add(-45 * time.Second),
		Now:          time.Now(),
		Input: &hookio.HookInput{
			ToolName:  "Bash",
			ToolInput: json.RawMessage(`{"command": "rm -rf /"}`),
		},
	}

	cond := dsl.AndCondition{
		Left:  dsl.ElapsedCondition{Op: ">", Duration: 30 * time.Second},
		Right: dsl.MatchCondition{Kind: "command", Pattern: "rm"},
	}
	if !EvalCondition(cond, ctx) {
		t.Error("expected true for AND")
	}

	// One side false
	cond = dsl.AndCondition{
		Left:  dsl.ElapsedCondition{Op: ">", Duration: 60 * time.Second},
		Right: dsl.MatchCondition{Kind: "command", Pattern: "rm"},
	}
	if EvalCondition(cond, ctx) {
		t.Error("expected false when left is false")
	}
}

func TestEvalOr(t *testing.T) {
	ctx := &EvalContext{
		SessionStart: time.Now().Add(-45 * time.Second),
		Now:          time.Now(),
	}

	cond := dsl.OrCondition{
		Left:  dsl.ElapsedCondition{Op: ">", Duration: 60 * time.Second}, // false
		Right: dsl.ElapsedCondition{Op: ">", Duration: 30 * time.Second}, // true
	}
	if !EvalCondition(cond, ctx) {
		t.Error("expected true for OR when right is true")
	}
}

func TestEvalNilCondition(t *testing.T) {
	ctx := &EvalContext{}
	if !EvalCondition(nil, ctx) {
		t.Error("nil condition should return true")
	}
}

func TestEvalFocusAway(t *testing.T) {
	// Away = elapsed > 60s (fallback)
	ctx := &EvalContext{
		SessionStart: time.Now().Add(-90 * time.Second),
		Now:          time.Now(),
	}
	cond := dsl.FocusCondition{State: "away"}
	if !EvalCondition(cond, ctx) {
		t.Error("expected away=true when 90s elapsed")
	}

	ctx.SessionStart = time.Now().Add(-30 * time.Second)
	if EvalCondition(cond, ctx) {
		t.Error("expected away=false when 30s elapsed")
	}
}

func TestEvalFocusIdle(t *testing.T) {
	ctx := &EvalContext{
		LastPrompt: time.Now().Add(-120 * time.Second),
		Now:        time.Now(),
	}

	cond := dsl.FocusCondition{State: "idle", Op: ">", Duration: 60 * time.Second}
	if !EvalCondition(cond, ctx) {
		t.Error("expected idle > 60s when 120s since last prompt")
	}

	cond = dsl.FocusCondition{State: "idle", Op: ">", Duration: 300 * time.Second}
	if EvalCondition(cond, ctx) {
		t.Error("expected not idle > 300s when 120s since last prompt")
	}
}
