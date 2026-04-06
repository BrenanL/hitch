package dsl

import "time"

// Rule is the top-level AST node for a DSL rule.
type Rule struct {
	Event     Event
	Actions   []Action
	Condition Condition // nil if no condition
	Line      int
	Raw       string // original DSL text
}

// Event represents the hook event in a rule.
type Event struct {
	Name    string // DSL event name: stop, pre-tool, pre-bash, etc.
	Matcher string // optional matcher after colon
	// Resolved fields (filled by validator)
	HookEvent      string // Claude Code event: Stop, PreToolUse, etc.
	DefaultMatcher string // default matcher for shorthand events
}

// Action is the interface for all action types.
type Action interface {
	actionNode()
}

// NotifyAction sends a notification to a channel.
type NotifyAction struct {
	Channel string
	Message string // optional custom message
}

func (NotifyAction) actionNode() {}

// RunAction executes a shell command.
type RunAction struct {
	Command string
	Async   bool
}

func (RunAction) actionNode() {}

// DenyAction blocks the current action.
type DenyAction struct {
	Reason string // optional reason
}

func (DenyAction) actionNode() {}

// RequireAction runs a check and blocks if it fails.
type RequireAction struct {
	Check string // e.g., "tests-pass", "lint"
}

func (RequireAction) actionNode() {}

// SummarizeAction generates a session summary.
type SummarizeAction struct{}

func (SummarizeAction) actionNode() {}

// LogAction logs the event.
type LogAction struct {
	Target string // optional target
}

func (LogAction) actionNode() {}

// Condition is the interface for all condition types.
type Condition interface {
	conditionNode()
}

// ElapsedCondition checks elapsed time since session start.
type ElapsedCondition struct {
	Op       string // >, <, >=, <=, =
	Duration time.Duration
}

func (ElapsedCondition) conditionNode() {}

// FocusCondition checks terminal focus state.
type FocusCondition struct {
	State string // "away", "focused", "idle"
	// For idle: optional comparison
	Op       string
	Duration time.Duration
}

func (FocusCondition) conditionNode() {}

// MatchCondition checks a regex pattern against input.
type MatchCondition struct {
	Kind    string // "", "file", "command"
	Pattern string // regex pattern or deny-list reference
	// If IsDenyList is true, Pattern is the deny list name
	IsDenyList bool
}

func (MatchCondition) conditionNode() {}

// NotCondition negates a condition.
type NotCondition struct {
	Cond Condition
}

func (NotCondition) conditionNode() {}

// AndCondition requires all sub-conditions to be true.
type AndCondition struct {
	Left  Condition
	Right Condition
}

func (AndCondition) conditionNode() {}

// OrCondition requires any sub-condition to be true.
type OrCondition struct {
	Left  Condition
	Right Condition
}

func (OrCondition) conditionNode() {}

// BurnRateCondition checks the current token burn rate.
type BurnRateCondition struct {
	Op        string  // >, <, >=, <=, =
	Threshold float64 // fraction (0-1) or absolute tokens/min (>1)
}

func (BurnRateCondition) conditionNode() {}

// ModelCondition checks the model identifier.
type ModelCondition struct {
	Substring string // case-insensitive
}

func (ModelCondition) conditionNode() {}

// ContextSizeCondition checks the context token count.
type ContextSizeCondition struct {
	Op        string // >, <, >=, <=, =
	Threshold int
}

func (ContextSizeCondition) conditionNode() {}

// ContextUsageCondition checks the percentage of context window filled (0–100).
type ContextUsageCondition struct {
	Op        string  // >, <, >=, <=, =
	Threshold float64 // percentage, 0–100
}

func (ContextUsageCondition) conditionNode() {}

// FieldEqCondition checks an input field against a literal string value.
// Used for error_type, task_status, and similar field equality checks.
type FieldEqCondition struct {
	Field string // "error_type", "task_status", etc.
	Value string
}

func (FieldEqCondition) conditionNode() {}

// SwitchProfileAction switches the active Hitch profile.
type SwitchProfileAction struct {
	Profile string
}

func (SwitchProfileAction) actionNode() {}

// InjectContextAction adds text to the hook response's additionalContext.
type InjectContextAction struct {
	Text string
}

func (InjectContextAction) actionNode() {}

// PruneAction signals SPEC-10's context pruning integration.
// Tier is one of "gentle", "moderate", "aggressive", "emergency".
type PruneAction struct {
	Tier string
}

func (PruneAction) actionNode() {}
