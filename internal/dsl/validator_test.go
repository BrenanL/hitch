package dsl

import "testing"

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
