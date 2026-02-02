package dsl

import (
	"testing"
	"time"
)

func TestParseStopNotify(t *testing.T) {
	rules, err := Parse(`on stop -> notify discord if elapsed > 30s`)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(rules) != 1 {
		t.Fatalf("got %d rules, want 1", len(rules))
	}

	r := rules[0]
	if r.Event.Name != "stop" {
		t.Errorf("event = %q", r.Event.Name)
	}
	if r.Event.HookEvent != "Stop" {
		t.Errorf("hookEvent = %q", r.Event.HookEvent)
	}

	if len(r.Actions) != 1 {
		t.Fatalf("actions = %d", len(r.Actions))
	}
	notify, ok := r.Actions[0].(NotifyAction)
	if !ok {
		t.Fatalf("action type = %T", r.Actions[0])
	}
	if notify.Channel != "discord" {
		t.Errorf("channel = %q", notify.Channel)
	}

	cond, ok := r.Condition.(ElapsedCondition)
	if !ok {
		t.Fatalf("condition type = %T", r.Condition)
	}
	if cond.Op != ">" {
		t.Errorf("op = %q", cond.Op)
	}
	if cond.Duration != 30*time.Second {
		t.Errorf("duration = %v", cond.Duration)
	}
}

func TestParsePreBashDeny(t *testing.T) {
	rules, err := Parse(`on pre-bash -> deny if matches "rm -rf"`)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	r := rules[0]

	if r.Event.HookEvent != "PreToolUse" {
		t.Errorf("hookEvent = %q, want PreToolUse", r.Event.HookEvent)
	}
	if r.Event.Matcher != "Bash" {
		t.Errorf("matcher = %q, want Bash", r.Event.Matcher)
	}

	_, ok := r.Actions[0].(DenyAction)
	if !ok {
		t.Fatalf("action type = %T", r.Actions[0])
	}

	cond, ok := r.Condition.(MatchCondition)
	if !ok {
		t.Fatalf("condition type = %T", r.Condition)
	}
	if cond.Pattern != "rm -rf" {
		t.Errorf("pattern = %q", cond.Pattern)
	}
}

func TestParsePostEditRunAsync(t *testing.T) {
	rules, err := Parse(`on post-edit -> run "npm test" async`)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	r := rules[0]

	if r.Event.HookEvent != "PostToolUse" {
		t.Errorf("hookEvent = %q", r.Event.HookEvent)
	}
	if r.Event.Matcher != "Edit|Write" {
		t.Errorf("matcher = %q", r.Event.Matcher)
	}

	run, ok := r.Actions[0].(RunAction)
	if !ok {
		t.Fatalf("action type = %T", r.Actions[0])
	}
	if run.Command != "npm test" {
		t.Errorf("command = %q", run.Command)
	}
	if !run.Async {
		t.Error("async should be true")
	}
}

func TestParseChainedActions(t *testing.T) {
	rules, err := Parse(`on stop -> summarize -> notify slack`)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	r := rules[0]

	if len(r.Actions) != 2 {
		t.Fatalf("actions = %d, want 2", len(r.Actions))
	}
	if _, ok := r.Actions[0].(SummarizeAction); !ok {
		t.Errorf("action[0] = %T, want SummarizeAction", r.Actions[0])
	}
	if a, ok := r.Actions[1].(NotifyAction); !ok || a.Channel != "slack" {
		t.Errorf("action[1] = %T %+v", r.Actions[1], r.Actions[1])
	}
}

func TestParseNotificationPermission(t *testing.T) {
	rules, err := Parse(`on notification:permission -> notify sms "Claude needs permission"`)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	r := rules[0]

	if r.Event.HookEvent != "Notification" {
		t.Errorf("hookEvent = %q", r.Event.HookEvent)
	}
	if r.Event.Matcher != "permission" {
		t.Errorf("matcher = %q, want permission", r.Event.Matcher)
	}

	notify := r.Actions[0].(NotifyAction)
	if notify.Channel != "sms" {
		t.Errorf("channel = %q", notify.Channel)
	}
	if notify.Message != "Claude needs permission" {
		t.Errorf("message = %q", notify.Message)
	}
}

func TestParseDenyListCondition(t *testing.T) {
	rules, err := Parse(`on pre-bash -> deny if matches deny-list:destructive`)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	r := rules[0]

	cond, ok := r.Condition.(MatchCondition)
	if !ok {
		t.Fatalf("condition type = %T", r.Condition)
	}
	if !cond.IsDenyList {
		t.Error("IsDenyList should be true")
	}
	if cond.Pattern != "destructive" {
		t.Errorf("pattern = %q", cond.Pattern)
	}
}

func TestParseCompoundCondition(t *testing.T) {
	rules, err := Parse(`on stop -> notify discord if elapsed > 30s and away`)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	r := rules[0]

	and, ok := r.Condition.(AndCondition)
	if !ok {
		t.Fatalf("condition type = %T", r.Condition)
	}

	_, ok = and.Left.(ElapsedCondition)
	if !ok {
		t.Errorf("left = %T", and.Left)
	}

	focus, ok := and.Right.(FocusCondition)
	if !ok {
		t.Errorf("right = %T", and.Right)
	}
	if focus.State != "away" {
		t.Errorf("state = %q", focus.State)
	}
}

func TestParseNotCondition(t *testing.T) {
	rules, err := Parse(`on stop -> notify ntfy if not focused`)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	r := rules[0]

	notCond, ok := r.Condition.(NotCondition)
	if !ok {
		t.Fatalf("condition type = %T", r.Condition)
	}

	focus, ok := notCond.Cond.(FocusCondition)
	if !ok {
		t.Errorf("inner = %T", notCond.Cond)
	}
	if focus.State != "focused" {
		t.Errorf("state = %q", focus.State)
	}
}

func TestParseMultipleRules(t *testing.T) {
	input := `
on stop -> notify discord if elapsed > 30s
on pre-bash -> deny if matches "rm -rf"
on post-edit -> run "npm test" async
`
	rules, err := Parse(input)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(rules) != 3 {
		t.Errorf("got %d rules, want 3", len(rules))
	}
}

func TestParsePreToolWithMatcher(t *testing.T) {
	rules, err := Parse(`on pre-tool:"Glob" -> log`)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	r := rules[0]

	if r.Event.HookEvent != "PreToolUse" {
		t.Errorf("hookEvent = %q", r.Event.HookEvent)
	}
	if r.Event.Matcher != "Glob" {
		t.Errorf("matcher = %q, want Glob", r.Event.Matcher)
	}
}

func TestParseLogAction(t *testing.T) {
	rules, err := Parse(`on stop -> log`)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	_, ok := rules[0].Actions[0].(LogAction)
	if !ok {
		t.Errorf("action type = %T", rules[0].Actions[0])
	}
}

func TestParseRequireAction(t *testing.T) {
	rules, err := Parse(`on stop -> require tests-pass`)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	req, ok := rules[0].Actions[0].(RequireAction)
	if !ok {
		t.Fatalf("action type = %T", rules[0].Actions[0])
	}
	if req.Check != "tests-pass" {
		t.Errorf("check = %q", req.Check)
	}
}

func TestParseIdleCondition(t *testing.T) {
	rules, err := Parse(`on notification -> notify ntfy if idle > 60s`)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	cond, ok := rules[0].Condition.(FocusCondition)
	if !ok {
		t.Fatalf("condition type = %T", rules[0].Condition)
	}
	if cond.State != "idle" {
		t.Errorf("state = %q", cond.State)
	}
	if cond.Op != ">" {
		t.Errorf("op = %q", cond.Op)
	}
	if cond.Duration != 60*time.Second {
		t.Errorf("duration = %v", cond.Duration)
	}
}

func TestParseFileMatches(t *testing.T) {
	rules, err := Parse(`on pre-edit -> deny if file matches ".env"`)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	cond, ok := rules[0].Condition.(MatchCondition)
	if !ok {
		t.Fatalf("condition type = %T", rules[0].Condition)
	}
	if cond.Kind != "file" {
		t.Errorf("kind = %q", cond.Kind)
	}
	if cond.Pattern != ".env" {
		t.Errorf("pattern = %q", cond.Pattern)
	}
}

func TestParseCommandMatches(t *testing.T) {
	rules, err := Parse(`on pre-bash -> deny if command matches "sudo"`)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	cond, ok := rules[0].Condition.(MatchCondition)
	if !ok {
		t.Fatalf("condition type = %T", rules[0].Condition)
	}
	if cond.Kind != "command" {
		t.Errorf("kind = %q", cond.Kind)
	}
}

func TestParseOrCondition(t *testing.T) {
	rules, err := Parse(`on stop -> notify ntfy if elapsed > 30s or away`)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	_, ok := rules[0].Condition.(OrCondition)
	if !ok {
		t.Fatalf("condition type = %T", rules[0].Condition)
	}
}

func TestParseDenyWithReason(t *testing.T) {
	rules, err := Parse(`on pre-bash -> deny "dangerous command"`)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	deny, ok := rules[0].Actions[0].(DenyAction)
	if !ok {
		t.Fatalf("action type = %T", rules[0].Actions[0])
	}
	if deny.Reason != "dangerous command" {
		t.Errorf("reason = %q", deny.Reason)
	}
}

func TestParseUnknownEvent(t *testing.T) {
	_, err := Parse(`on stpo -> deny`)
	if err == nil {
		t.Fatal("expected error for unknown event")
	}
	pe, ok := err.(*ParseError)
	if !ok {
		t.Fatalf("error type = %T", err)
	}
	if pe.Suggestion == "" {
		t.Error("expected suggestion for typo")
	}
}

func TestParseUnknownEventNoSuggestion(t *testing.T) {
	_, err := Parse(`on xyzzy -> deny`)
	if err == nil {
		t.Fatal("expected error for unknown event")
	}
	pe, ok := err.(*ParseError)
	if !ok {
		t.Fatalf("error type = %T", err)
	}
	// xyzzy is too far from any event — no suggestion expected
	if pe.Suggestion != "" {
		t.Errorf("unexpected suggestion: %q", pe.Suggestion)
	}
}

func TestParseMissingArrow(t *testing.T) {
	_, err := Parse(`on stop notify discord`)
	if err == nil {
		t.Fatal("expected error for missing arrow")
	}
}

func TestParseMissingDurationUnit(t *testing.T) {
	_, err := Parse(`on stop -> notify ntfy if elapsed > 30`)
	if err == nil {
		t.Fatal("expected error for missing duration unit")
	}
}

func TestParseRuleFunc(t *testing.T) {
	rule, err := ParseRule(`on stop -> notify ntfy`)
	if err != nil {
		t.Fatalf("ParseRule: %v", err)
	}
	if rule.Event.HookEvent != "Stop" {
		t.Errorf("hookEvent = %q", rule.Event.HookEvent)
	}
}

func TestParseRuleEmpty(t *testing.T) {
	_, err := ParseRule(``)
	if err == nil {
		t.Fatal("expected error for empty input")
	}
}

func TestParseComments(t *testing.T) {
	input := `
# This is a comment
on stop -> notify discord
# Another comment
on pre-bash -> deny
`
	rules, err := Parse(input)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(rules) != 2 {
		t.Errorf("got %d rules, want 2", len(rules))
	}
}

func TestParseDenyNoCondition(t *testing.T) {
	rules, err := Parse(`on pre-bash -> deny`)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if rules[0].Condition != nil {
		t.Errorf("condition should be nil")
	}
}
