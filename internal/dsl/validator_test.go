package dsl

import (
	"strings"
	"testing"
)

func TestValidateUnknownChannel(t *testing.T) {
	rules, err := Parse(`on stop -> notify discord`)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	result := Validate(rules, []string{"ntfy", "slack"})
	if len(result.Warnings) == 0 {
		t.Error("expected warning for unknown channel 'discord'")
	}
	if result.HasErrors() {
		t.Error("should not have errors, only warnings")
	}
}

func TestValidateKnownChannel(t *testing.T) {
	rules, err := Parse(`on stop -> notify discord`)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	result := Validate(rules, []string{"discord"})
	if len(result.Warnings) != 0 {
		t.Errorf("unexpected warnings: %v", result.Warnings)
	}
}

func TestValidateDenyOnNonBlockingEvent(t *testing.T) {
	rules, err := Parse(`on post-tool -> deny`)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	result := Validate(rules, nil)
	if len(result.Warnings) == 0 {
		t.Error("expected warning for deny on non-blocking event")
	}
}

func TestValidateDenyOnBlockingEvent(t *testing.T) {
	rules, err := Parse(`on pre-bash -> deny`)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	result := Validate(rules, nil)
	if len(result.Warnings) != 0 {
		t.Errorf("unexpected warnings for deny on pre-bash: %v", result.Warnings)
	}
}

func TestValidateNoChannelsConfigured(t *testing.T) {
	rules, err := Parse(`on stop -> notify ntfy`)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	// Empty channel list = skip channel validation
	result := Validate(rules, nil)
	if len(result.Warnings) != 0 {
		t.Errorf("unexpected warnings with nil channels: %v", result.Warnings)
	}
}

func TestValidateMultipleRules(t *testing.T) {
	input := `
on stop -> notify discord if elapsed > 30s
on pre-bash -> deny if matches "rm -rf"
on post-edit -> run "npm test" async
`
	rules, err := Parse(input)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	result := Validate(rules, []string{"discord"})
	if result.HasErrors() {
		t.Errorf("unexpected errors: %v", result.Errors)
	}
}

// TestValidateDenyOnNonBlockingEventSessionEnd checks that session-end (non-blocking) produces a warning.
func TestValidateDenyOnNonBlockingEventSessionEnd(t *testing.T) {
	rules, err := Parse(`on session-end -> deny "test"`)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	result := Validate(rules, nil)
	if len(result.Warnings) == 0 {
		t.Error("expected warning for deny on session-end (non-blocking)")
	}
}

// TestValidateDenyOnAllBlockingEvents checks all 12 blocking events produce no deny warning.
func TestValidateDenyOnAllBlockingEvents(t *testing.T) {
	blockingRules := []string{
		`on pre-bash -> deny`,
		`on pre-tool -> deny`,
		`on stop -> deny`,
		`on subagent-stop -> deny`,
		`on permission -> deny`,
		`on user-prompt -> deny`,
		`on task-created -> deny`,
		`on task-completed -> deny`,
		`on teammate-idle -> deny`,
		`on config-change -> deny`,
		`on worktree-create -> deny`,
		`on elicitation -> deny`,
		`on elicitation-result -> deny`,
	}

	for _, input := range blockingRules {
		rules, err := Parse(input)
		if err != nil {
			t.Fatalf("Parse %q: %v", input, err)
		}
		result := Validate(rules, nil)
		for _, w := range result.Warnings {
			if strings.Contains(w.Message, "deny action on non-blocking") {
				t.Errorf("unexpected deny warning for %q: %s", input, w.Message)
			}
		}
	}
}

// TestValidateDefaultRules checks that all suggested default rules from SPEC-02 section 6 parse and validate cleanly.
func TestValidateDefaultRules(t *testing.T) {
	input := `
on subagent-start -> log
on subagent-start -> deny "Opus not permitted for subagents" if model contains "opus"
on stop-failure -> notify discord if error_type == "rate_limit"
on stop-failure -> log
on pre-compact -> log
on post-compact -> log
on config-change -> log
on permission-denied -> log
on worktree-create -> log
on worktree-remove -> log
on task-created -> log
on task-completed -> log
`
	rules, err := Parse(input)
	if err != nil {
		t.Fatalf("Parse default rules: %v", err)
	}

	result := Validate(rules, []string{"discord"})
	if result.HasErrors() {
		t.Errorf("default rules should not have errors: %v", result.Errors)
	}
}

// TestValidateErrorTypeOnWrongEvent checks that error_type on a non-stop-failure event emits a warning.
func TestValidateErrorTypeOnWrongEvent(t *testing.T) {
	rules, err := Parse(`on stop -> notify discord if error_type == "rate_limit"`)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	result := Validate(rules, []string{"discord"})
	if len(result.Warnings) == 0 {
		t.Error("expected warning for error_type on non-stop-failure event")
	}
}

// TestValidateErrorTypeOnCorrectEvent checks that error_type on stop-failure produces no warning.
func TestValidateErrorTypeOnCorrectEvent(t *testing.T) {
	rules, err := Parse(`on stop-failure -> notify discord if error_type == "rate_limit"`)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	result := Validate(rules, []string{"discord"})
	if len(result.Warnings) != 0 {
		t.Errorf("unexpected warnings for error_type on stop-failure: %v", result.Warnings)
	}
}

// TestValidateTaskStatusOnWrongEvent checks that task_status on a non-task event emits a warning.
func TestValidateTaskStatusOnWrongEvent(t *testing.T) {
	rules, err := Parse(`on stop -> log if task_status == "completed"`)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	result := Validate(rules, nil)
	if len(result.Warnings) == 0 {
		t.Error("expected warning for task_status on non-task event")
	}
}

// TestValidateTaskStatusOnCorrectEvent checks that task_status on task-completed produces no warning.
func TestValidateTaskStatusOnCorrectEvent(t *testing.T) {
	rules, err := Parse(`on task-completed -> log if task_status == "completed"`)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	result := Validate(rules, nil)
	if len(result.Warnings) != 0 {
		t.Errorf("unexpected warnings for task_status on task-completed: %v", result.Warnings)
	}
}

// TestValidatePruneOnNonCompactEvent checks that prune on a non-compact event emits a warning.
func TestValidatePruneOnNonCompactEvent(t *testing.T) {
	rules, err := Parse(`on task-created -> prune gentle`)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	result := Validate(rules, nil)
	if len(result.Warnings) == 0 {
		t.Error("expected warning for prune on task-created (non-compact event)")
	}
}

// TestValidatePruneOnPreCompact checks that prune on pre-compact produces no warning.
func TestValidatePruneOnPreCompact(t *testing.T) {
	rules, err := Parse(`on pre-compact -> prune gentle`)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	result := Validate(rules, nil)
	if len(result.Warnings) != 0 {
		t.Errorf("unexpected warnings for prune on pre-compact: %v", result.Warnings)
	}
}
