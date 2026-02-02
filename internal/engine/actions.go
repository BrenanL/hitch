package engine

import (
	"context"
	"fmt"
	"os/exec"
	"time"

	"github.com/BrenanL/hitch/internal/adapters"
	"github.com/BrenanL/hitch/internal/dsl"
	"github.com/BrenanL/hitch/internal/state"
	"github.com/BrenanL/hitch/pkg/hookio"
)

// ActionContext provides dependencies for action execution.
type ActionContext struct {
	DB          *state.DB
	GetAdapter  func(name string) (adapters.Adapter, error)
	SessionID   string
	HookEvent   string
	RuleID      string
	Input       *hookio.HookInput
}

// ActionResult is the outcome of executing an action.
type ActionResult struct {
	Output      *hookio.HookOutput
	ActionTaken string
	ShouldBlock bool
	Error       error
}

// ExecuteAction runs a single action and returns the result.
func ExecuteAction(ctx context.Context, action dsl.Action, actx *ActionContext) *ActionResult {
	switch a := action.(type) {
	case dsl.NotifyAction:
		return executeNotify(ctx, a, actx)
	case dsl.DenyAction:
		return executeDeny(a)
	case dsl.RunAction:
		return executeRun(ctx, a, actx)
	case dsl.RequireAction:
		return executeRequire(ctx, a)
	case dsl.SummarizeAction:
		return executeSummarize(actx)
	case dsl.LogAction:
		return executeLog(actx)
	default:
		return &ActionResult{Error: fmt.Errorf("unknown action type: %T", action)}
	}
}

func executeNotify(ctx context.Context, a dsl.NotifyAction, actx *ActionContext) *ActionResult {
	adapter, err := actx.GetAdapter(a.Channel)
	if err != nil {
		return &ActionResult{
			Error:       fmt.Errorf("getting adapter %q: %w", a.Channel, err),
			ActionTaken: fmt.Sprintf("notify-failed:%s", a.Channel),
		}
	}

	body := a.Message
	if body == "" {
		body = fmt.Sprintf("Hook event: %s", actx.HookEvent)
	}

	msg := adapters.Message{
		Title:   fmt.Sprintf("Hitch: %s", actx.HookEvent),
		Body:    body,
		Level:   adapters.Info,
		Event:   actx.HookEvent,
		Session: actx.SessionID,
	}

	result := adapter.Send(ctx, msg)
	if !result.Success {
		return &ActionResult{
			Error:       result.Error,
			ActionTaken: fmt.Sprintf("notify-failed:%s", a.Channel),
		}
	}

	// Update last used timestamp
	if actx.DB != nil {
		actx.DB.ChannelUpdateLastUsed(a.Channel)
	}

	return &ActionResult{
		ActionTaken: fmt.Sprintf("notified:%s", a.Channel),
	}
}

func executeDeny(a dsl.DenyAction) *ActionResult {
	reason := a.Reason
	if reason == "" {
		reason = "blocked by hitch rule"
	}

	return &ActionResult{
		Output:      hookio.Deny(reason),
		ActionTaken: "denied",
		ShouldBlock: true,
	}
}

func executeRun(ctx context.Context, a dsl.RunAction, actx *ActionContext) *ActionResult {
	if a.Async {
		// Run in background — don't wait for result
		go func() {
			cmd := exec.CommandContext(context.Background(), "sh", "-c", a.Command)
			if actx.Input != nil {
				cmd.Dir = actx.Input.Cwd
			}
			cmd.Run()
		}()
		return &ActionResult{
			ActionTaken: fmt.Sprintf("run-async:%s", a.Command),
		}
	}

	// Run synchronously with timeout
	cmdCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(cmdCtx, "sh", "-c", a.Command)
	if actx.Input != nil {
		cmd.Dir = actx.Input.Cwd
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return &ActionResult{
			Error:       fmt.Errorf("command %q: %w\n%s", a.Command, err, output),
			ActionTaken: fmt.Sprintf("run-failed:%s", a.Command),
		}
	}

	return &ActionResult{
		ActionTaken: fmt.Sprintf("run:%s", a.Command),
	}
}

func executeRequire(ctx context.Context, a dsl.RequireAction) *ActionResult {
	// For now, require runs the check name as a command
	// In the future, this could map to well-known checks
	cmdCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(cmdCtx, "sh", "-c", a.Check)
	output, err := cmd.CombinedOutput()
	if err != nil {
		reason := fmt.Sprintf("requirement %q failed: %s", a.Check, string(output))
		return &ActionResult{
			Output:      hookio.ContinueWorking(reason),
			ActionTaken: fmt.Sprintf("require-failed:%s", a.Check),
			ShouldBlock: true,
		}
	}

	return &ActionResult{
		ActionTaken: fmt.Sprintf("require-passed:%s", a.Check),
	}
}

func executeSummarize(actx *ActionContext) *ActionResult {
	// Placeholder — summarization requires transcript reading
	return &ActionResult{
		ActionTaken: "summarized",
	}
}

func executeLog(actx *ActionContext) *ActionResult {
	if actx.DB != nil {
		actx.DB.EventLog(state.Event{
			SessionID:   actx.SessionID,
			HookEvent:   actx.HookEvent,
			RuleID:      actx.RuleID,
			ToolName:    actx.Input.ToolName,
			ActionTaken: "logged",
		})
	}
	return &ActionResult{
		ActionTaken: "logged",
	}
}
