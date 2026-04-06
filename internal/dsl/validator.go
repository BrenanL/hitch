package dsl

import "fmt"

// ValidateResult holds the results of semantic validation.
type ValidateResult struct {
	Errors   []*ValidateError
	Warnings []*ValidateError
}

// HasErrors returns true if there are any errors (not warnings).
func (r *ValidateResult) HasErrors() bool {
	return len(r.Errors) > 0
}

// Validate performs semantic validation on parsed rules.
// knownChannels is the list of configured channel names (for warning on unknown channels).
func Validate(rules []Rule, knownChannels []string) *ValidateResult {
	result := &ValidateResult{}
	channelSet := make(map[string]bool)
	for _, c := range knownChannels {
		channelSet[c] = true
	}

	for _, rule := range rules {
		validateRule(&rule, channelSet, result)
	}

	return result
}

func validateRule(rule *Rule, knownChannels map[string]bool, result *ValidateResult) {
	// Validate event
	if rule.Event.HookEvent == "" {
		result.Errors = append(result.Errors, &ValidateError{
			Line:    rule.Line,
			Message: fmt.Sprintf("unresolved event %q", rule.Event.Name),
		})
	}

	// Validate actions
	for _, action := range rule.Actions {
		validateAction(action, rule, knownChannels, result)
	}

	// Validate conditions
	if rule.Condition != nil {
		validateCondition(rule.Condition, rule, result)
	}
}

func validateAction(action Action, rule *Rule, knownChannels map[string]bool, result *ValidateResult) {
	switch a := action.(type) {
	case NotifyAction:
		if a.Channel == "" {
			result.Errors = append(result.Errors, &ValidateError{
				Line:    rule.Line,
				Message: "notify action requires a channel name",
			})
		}
		if len(knownChannels) > 0 && !knownChannels[a.Channel] {
			channels := make([]string, 0, len(knownChannels))
			for c := range knownChannels {
				channels = append(channels, c)
			}
			result.Warnings = append(result.Warnings, &ValidateError{
				Line:      rule.Line,
				Message:   fmt.Sprintf("unknown channel %q", a.Channel),
				IsWarning: true,
				Suggestion: fmt.Sprintf("configured channels: %v", channels),
			})
		}

	case RunAction:
		if a.Command == "" {
			result.Errors = append(result.Errors, &ValidateError{
				Line:    rule.Line,
				Message: "run action requires a command",
			})
		}

	case DenyAction:
		// Deny is only meaningful for blocking events
		if !canBlock(rule.Event.HookEvent) {
			result.Warnings = append(result.Warnings, &ValidateError{
				Line:      rule.Line,
				Message:   fmt.Sprintf("deny action on non-blocking event %q will have no effect", rule.Event.Name),
				IsWarning: true,
			})
		}

	case RequireAction:
		if a.Check == "" {
			result.Errors = append(result.Errors, &ValidateError{
				Line:    rule.Line,
				Message: "require action needs a check name",
			})
		}

	case PruneAction:
		// prune is most useful on context lifecycle events; warn on others
		if rule.Event.HookEvent != "PreCompact" && rule.Event.HookEvent != "PostCompact" {
			result.Warnings = append(result.Warnings, &ValidateError{
				Line:      rule.Line,
				Message:   fmt.Sprintf("prune action on %q has no effect; prune is only meaningful on 'pre-compact' or 'post-compact' events", rule.Event.Name),
				IsWarning: true,
			})
		}
	}
}

func validateCondition(cond Condition, rule *Rule, result *ValidateResult) {
	switch c := cond.(type) {
	case ElapsedCondition:
		if c.Duration <= 0 {
			result.Errors = append(result.Errors, &ValidateError{
				Line:    rule.Line,
				Message: "elapsed duration must be positive",
			})
		}

	case MatchCondition:
		// "command matches" only makes sense on bash events
		if c.Kind == "command" && rule.Event.HookEvent != "PreToolUse" && rule.Event.HookEvent != "PostToolUse" && rule.Event.HookEvent != "PostToolUseFailure" {
			result.Warnings = append(result.Warnings, &ValidateError{
				Line:      rule.Line,
				Message:   "'command matches' is only useful on tool events",
				IsWarning: true,
			})
		}

	case FieldEqCondition:
		switch c.Field {
		case "error_type":
			if rule.Event.HookEvent != "StopFailure" {
				result.Warnings = append(result.Warnings, &ValidateError{
					Line:      rule.Line,
					Message:   "'error_type' condition is only meaningful on 'stop-failure' event",
					IsWarning: true,
				})
			}
		case "task_status":
			if rule.Event.HookEvent != "TaskCreated" && rule.Event.HookEvent != "TaskCompleted" {
				result.Warnings = append(result.Warnings, &ValidateError{
					Line:      rule.Line,
					Message:   "'task_status' condition is only meaningful on 'task-created' or 'task-completed' events",
					IsWarning: true,
				})
			}
		}

	case NotCondition:
		validateCondition(c.Cond, rule, result)

	case AndCondition:
		validateCondition(c.Left, rule, result)
		validateCondition(c.Right, rule, result)

	case OrCondition:
		validateCondition(c.Left, rule, result)
		validateCondition(c.Right, rule, result)
	}
}

// canBlock returns true if the given Claude Code event supports blocking.
func canBlock(hookEvent string) bool {
	switch hookEvent {
	case "PreToolUse", "UserPromptSubmit", "PermissionRequest", "Stop", "SubagentStop",
		"TaskCreated", "TaskCompleted", "TeammateIdle", "ConfigChange",
		"WorktreeCreate", "Elicitation", "ElicitationResult":
		return true
	default:
		return false
	}
}
