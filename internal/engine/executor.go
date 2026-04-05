package engine

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/BrenanL/hitch/internal/adapters"
	"github.com/BrenanL/hitch/internal/dsl"
	"github.com/BrenanL/hitch/internal/state"
	"github.com/BrenanL/hitch/pkg/hookio"
)

// Executor orchestrates the full hook execution pipeline.
type Executor struct {
	DB         *state.DB
	GetAdapter func(name string) (adapters.Adapter, error)
	DenyLists  map[string][]string
}

// ExecResult is the outcome of executing a rule.
type ExecResult struct {
	Output      *hookio.HookOutput
	ActionsTaken []string
	Blocked     bool
	Error       error
	DurationMs  int
}

// Execute runs a rule against hook input.
func (e *Executor) Execute(ctx context.Context, rule *state.Rule, input *hookio.HookInput) *ExecResult {
	start := time.Now()
	result := &ExecResult{}

	// Parse the rule's DSL
	parsed, err := dsl.ParseRule(rule.DSL)
	if err != nil {
		result.Error = fmt.Errorf("parsing rule %s: %w", rule.ID, err)
		result.DurationMs = int(time.Since(start).Milliseconds())
		return result
	}

	// Build eval context
	evalCtx := &EvalContext{
		Input:     input,
		Now:       time.Now(),
		DenyLists: e.DenyLists,
	}

	// Load session start time
	if e.DB != nil && input.SessionID != "" {
		if startStr, err := e.DB.KVGet("session_start:"+input.SessionID, input.SessionID); err == nil && startStr != "" {
			if t, err := time.Parse(time.RFC3339, startStr); err == nil {
				evalCtx.SessionStart = t
			}
		}
		if lastStr, err := e.DB.KVGet("last_prompt:"+input.SessionID, input.SessionID); err == nil && lastStr != "" {
			if t, err := time.Parse(time.RFC3339, lastStr); err == nil {
				evalCtx.LastPrompt = t
			}
		}
	}

	// Evaluate conditions
	if !EvalCondition(parsed.Condition, evalCtx) {
		result.Output = hookio.Allow()
		result.ActionsTaken = append(result.ActionsTaken, "condition-false")
		result.DurationMs = int(time.Since(start).Milliseconds())
		e.logEvent(input, rule.ID, "condition-false", result.DurationMs)
		return result
	}

	// Execute actions
	actx := &ActionContext{
		DB:         e.DB,
		GetAdapter: e.GetAdapter,
		SessionID:  input.SessionID,
		HookEvent:  input.HookEventName,
		RuleID:     rule.ID,
		Input:      input,
	}

	var allActionResults []*ActionResult
	for _, action := range parsed.Actions {
		actionResult := ExecuteAction(ctx, action, actx)
		allActionResults = append(allActionResults, actionResult)
		if actionResult.ActionTaken != "" {
			result.ActionsTaken = append(result.ActionsTaken, actionResult.ActionTaken)
		}
		if actionResult.Error != nil {
			result.Error = actionResult.Error
		}
		if actionResult.ShouldBlock {
			result.Blocked = true
			result.Output = actionResult.Output
		}
	}

	// Default output if not blocked
	if result.Output == nil {
		result.Output = hookio.Allow()
	}

	// Collect additionalContext from all inject_context actions and merge
	var contexts []string
	for _, ar := range allActionResults {
		if ar.Output != nil && ar.Output.AdditionalContext != "" {
			contexts = append(contexts, ar.Output.AdditionalContext)
		}
	}
	if len(contexts) > 0 {
		result.Output.AdditionalContext = strings.Join(contexts, "\n")
	}

	result.DurationMs = int(time.Since(start).Milliseconds())

	// Log event
	actionStr := ""
	if len(result.ActionsTaken) > 0 {
		actionStr = result.ActionsTaken[len(result.ActionsTaken)-1]
	}
	e.logEvent(input, rule.ID, actionStr, result.DurationMs)

	return result
}

// ExecuteSystemHook handles system hooks (session tracking, prompt tracking).
func (e *Executor) ExecuteSystemHook(name string, input *hookio.HookInput) *ExecResult {
	result := &ExecResult{Output: hookio.Allow()}

	if e.DB == nil {
		return result
	}

	switch name {
	case "session-start":
		now := time.Now().UTC().Format(time.RFC3339)
		e.DB.KVSet("session_start:"+input.SessionID, now, input.SessionID, "")
		e.DB.SessionCreate(state.Session{
			SessionID:  input.SessionID,
			ProjectDir: input.Cwd,
			StartedAt:  now,
		})
		result.ActionsTaken = []string{"session-tracked"}

	case "user-prompt":
		now := time.Now().UTC().Format(time.RFC3339)
		e.DB.KVSet("last_prompt:"+input.SessionID, now, input.SessionID, "")
		result.ActionsTaken = []string{"prompt-tracked"}
	}

	return result
}

func (e *Executor) logEvent(input *hookio.HookInput, ruleID, action string, durationMs int) {
	if e.DB == nil {
		return
	}
	e.DB.EventLog(state.Event{
		SessionID:   input.SessionID,
		HookEvent:   input.HookEventName,
		RuleID:      ruleID,
		ToolName:    input.ToolName,
		ActionTaken: action,
		DurationMs:  durationMs,
	})
	if input.SessionID != "" {
		e.DB.SessionIncrementEventCount(input.SessionID)
	}
}
