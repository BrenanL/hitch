package engine

import (
	"regexp"
	"strings"
	"time"

	"github.com/BrenanL/hitch/internal/dsl"
	"github.com/BrenanL/hitch/pkg/hookio"
)

// EvalContext provides the data needed to evaluate conditions.
type EvalContext struct {
	Input        *hookio.HookInput
	SessionStart time.Time // when the session started
	LastPrompt   time.Time // last user interaction time
	Now          time.Time
	DenyLists    map[string][]string // deny list name → patterns
}

// EvalCondition evaluates a condition against the given context.
// Returns true if the condition is met (actions should fire).
func EvalCondition(cond dsl.Condition, ctx *EvalContext) bool {
	if cond == nil {
		return true // no condition = always fire
	}

	switch c := cond.(type) {
	case dsl.ElapsedCondition:
		return evalElapsed(c, ctx)
	case dsl.FocusCondition:
		return evalFocus(c, ctx)
	case dsl.MatchCondition:
		return evalMatch(c, ctx)
	case dsl.NotCondition:
		return !EvalCondition(c.Cond, ctx)
	case dsl.AndCondition:
		return EvalCondition(c.Left, ctx) && EvalCondition(c.Right, ctx)
	case dsl.OrCondition:
		return EvalCondition(c.Left, ctx) || EvalCondition(c.Right, ctx)
	default:
		return false
	}
}

func evalElapsed(c dsl.ElapsedCondition, ctx *EvalContext) bool {
	if ctx.SessionStart.IsZero() {
		return false
	}
	elapsed := ctx.Now.Sub(ctx.SessionStart)
	return compareTime(elapsed, c.Op, c.Duration)
}

func evalFocus(c dsl.FocusCondition, ctx *EvalContext) bool {
	switch c.State {
	case "away":
		// Best-effort: fall back to elapsed > 60s if detection unavailable
		if ctx.SessionStart.IsZero() {
			return false
		}
		return ctx.Now.Sub(ctx.SessionStart) > 60*time.Second
	case "focused":
		if ctx.SessionStart.IsZero() {
			return true
		}
		return ctx.Now.Sub(ctx.SessionStart) <= 60*time.Second
	case "idle":
		if ctx.LastPrompt.IsZero() {
			return false
		}
		idle := ctx.Now.Sub(ctx.LastPrompt)
		if c.Op != "" {
			return compareTime(idle, c.Op, c.Duration)
		}
		// No comparison = just check if idle at all (> 0)
		return idle > 0
	default:
		return false
	}
}

func evalMatch(c dsl.MatchCondition, ctx *EvalContext) bool {
	if c.IsDenyList {
		return evalDenyList(c, ctx)
	}

	target := matchTarget(c.Kind, ctx)
	if target == "" {
		return false
	}

	matched, err := regexp.MatchString(c.Pattern, target)
	if err != nil {
		return false
	}
	return matched
}

func evalDenyList(c dsl.MatchCondition, ctx *EvalContext) bool {
	patterns, ok := ctx.DenyLists[c.Pattern]
	if !ok {
		return false
	}

	target := matchTarget(c.Kind, ctx)
	if target == "" {
		return false
	}

	for _, pattern := range patterns {
		if strings.Contains(target, pattern) {
			return true
		}
	}
	return false
}

// matchTarget returns the string to match against based on the condition kind.
func matchTarget(kind string, ctx *EvalContext) string {
	if ctx.Input == nil {
		return ""
	}

	switch kind {
	case "file":
		return ctx.Input.FilePath()
	case "command":
		return ctx.Input.Command()
	default:
		// Default: use command for bash events, file for edit events
		if ctx.Input.ToolName == "Bash" {
			return ctx.Input.Command()
		}
		if ctx.Input.ToolName == "Edit" || ctx.Input.ToolName == "Write" {
			return ctx.Input.FilePath()
		}
		// For other events, try command first then file
		if cmd := ctx.Input.Command(); cmd != "" {
			return cmd
		}
		return ctx.Input.FilePath()
	}
}

func compareTime(actual time.Duration, op string, threshold time.Duration) bool {
	switch op {
	case ">":
		return actual > threshold
	case "<":
		return actual < threshold
	case ">=":
		return actual >= threshold
	case "<=":
		return actual <= threshold
	case "=":
		return actual == threshold
	default:
		return false
	}
}
