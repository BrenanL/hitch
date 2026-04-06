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

func TestEvalBurnRate(t *testing.T) {
	ctx := &EvalContext{BurnRate: 1200.0}

	// burn_rate > 1000 — should be true
	cond := dsl.BurnRateCondition{Op: ">", Threshold: 1000.0}
	if !EvalCondition(cond, ctx) {
		t.Error("expected true for burn_rate > 1000 when rate is 1200")
	}

	// burn_rate > 1500 — should be false
	cond = dsl.BurnRateCondition{Op: ">", Threshold: 1500.0}
	if EvalCondition(cond, ctx) {
		t.Error("expected false for burn_rate > 1500 when rate is 1200")
	}

	// burn_rate < 1500 — should be true
	cond = dsl.BurnRateCondition{Op: "<", Threshold: 1500.0}
	if !EvalCondition(cond, ctx) {
		t.Error("expected true for burn_rate < 1500 when rate is 1200")
	}
}

func TestEvalModel(t *testing.T) {
	ctx := &EvalContext{Model: "claude-opus-4-5"}

	// model contains "opus" — should be true (case-insensitive)
	cond := dsl.ModelCondition{Substring: "opus"}
	if !EvalCondition(cond, ctx) {
		t.Error("expected true for model contains 'opus'")
	}

	// model contains "sonnet" — should be false
	cond = dsl.ModelCondition{Substring: "sonnet"}
	if EvalCondition(cond, ctx) {
		t.Error("expected false for model contains 'sonnet'")
	}

	// empty model — should be false
	emptyCtx := &EvalContext{Model: ""}
	cond = dsl.ModelCondition{Substring: "opus"}
	if EvalCondition(cond, emptyCtx) {
		t.Error("expected false when model is empty")
	}
}

func TestEvalContextSize(t *testing.T) {
	ctx := &EvalContext{ContextSize: 150000}

	// context_size > 100000 — should be true
	cond := dsl.ContextSizeCondition{Op: ">", Threshold: 100000}
	if !EvalCondition(cond, ctx) {
		t.Error("expected true for context_size > 100000 when size is 150000")
	}

	// context_size > 200000 — should be false
	cond = dsl.ContextSizeCondition{Op: ">", Threshold: 200000}
	if EvalCondition(cond, ctx) {
		t.Error("expected false for context_size > 200000 when size is 150000")
	}

	// context_size < 200000 — should be true
	cond = dsl.ContextSizeCondition{Op: "<", Threshold: 200000}
	if !EvalCondition(cond, ctx) {
		t.Error("expected true for context_size < 200000 when size is 150000")
	}
}

func TestEvalContextUsage(t *testing.T) {
	ctx := &EvalContext{ContextUsage: 85.0}

	// context_usage > 80 — should be true
	cond := dsl.ContextUsageCondition{Op: ">", Threshold: 80.0}
	if !EvalCondition(cond, ctx) {
		t.Error("expected true for context_usage > 80 when usage is 85")
	}

	// context_usage > 90 — should be false
	cond = dsl.ContextUsageCondition{Op: ">", Threshold: 90.0}
	if EvalCondition(cond, ctx) {
		t.Error("expected false for context_usage > 90 when usage is 85")
	}
}

func TestEvalErrorType(t *testing.T) {
	ctx := &EvalContext{ErrorType: "rate_limit"}

	// error_type == "rate_limit" — should be true
	cond := dsl.FieldEqCondition{Field: "error_type", Value: "rate_limit"}
	if !EvalCondition(cond, ctx) {
		t.Error("expected true for error_type == 'rate_limit'")
	}

	// error_type == "billing_error" — should be false
	cond = dsl.FieldEqCondition{Field: "error_type", Value: "billing_error"}
	if EvalCondition(cond, ctx) {
		t.Error("expected false for error_type == 'billing_error' when type is 'rate_limit'")
	}
}

func TestEvalTaskStatus(t *testing.T) {
	ctx := &EvalContext{TaskStatus: "completed"}

	// task_status == "completed" — should be true
	cond := dsl.FieldEqCondition{Field: "task_status", Value: "completed"}
	if !EvalCondition(cond, ctx) {
		t.Error("expected true for task_status == 'completed'")
	}

	// task_status == "failed" — should be false
	cond = dsl.FieldEqCondition{Field: "task_status", Value: "failed"}
	if EvalCondition(cond, ctx) {
		t.Error("expected false for task_status == 'failed' when status is 'completed'")
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

func TestEvalBurnRateZero(t *testing.T) {
	// When BurnRate is 0 (proxy unavailable or no data), burn_rate > 0 should be false.
	ctx := &EvalContext{BurnRate: 0}
	cond := dsl.BurnRateCondition{Op: ">", Threshold: 0}
	if EvalCondition(cond, ctx) {
		t.Error("expected false for burn_rate > 0 when rate is 0")
	}
}

func TestEvalFieldEqUnknownField(t *testing.T) {
	// Unknown field names in FieldEqCondition should return false (fail safe).
	ctx := &EvalContext{ErrorType: "rate_limit", TaskStatus: "completed"}
	cond := dsl.FieldEqCondition{Field: "unknown_field", Value: "anything"}
	if EvalCondition(cond, ctx) {
		t.Error("expected false for unknown field in FieldEqCondition")
	}
}

func TestEvalContextSizeZero(t *testing.T) {
	// When ContextSize is 0 (not populated), context_size > 0 should be false.
	ctx := &EvalContext{ContextSize: 0}
	cond := dsl.ContextSizeCondition{Op: ">", Threshold: 0}
	if EvalCondition(cond, ctx) {
		t.Error("expected false for context_size > 0 when size is 0")
	}
}

func TestEvalContextUsageZero(t *testing.T) {
	// When ContextUsage is 0 (not populated), context_usage > 0 should be false.
	ctx := &EvalContext{ContextUsage: 0.0}
	cond := dsl.ContextUsageCondition{Op: ">", Threshold: 0.0}
	if EvalCondition(cond, ctx) {
		t.Error("expected false for context_usage > 0 when usage is 0")
	}
}
