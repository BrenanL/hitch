package dsl

import (
	"fmt"
	"strconv"
	"time"
)

// Parser is a recursive descent parser for the hitch DSL.
type Parser struct {
	tokens []Token
	pos    int
}

// Parse parses a DSL string and returns the parsed rules.
func Parse(input string) ([]Rule, error) {
	lexer := NewLexer(input)
	tokens, err := lexer.Tokenize()
	if err != nil {
		return nil, err
	}

	p := &Parser{tokens: tokens}
	return p.parseFile(input)
}

// ParseRule parses a single rule from a DSL string.
func ParseRule(input string) (*Rule, error) {
	rules, err := Parse(input)
	if err != nil {
		return nil, err
	}
	if len(rules) == 0 {
		return nil, fmt.Errorf("no rule found in input")
	}
	if len(rules) > 1 {
		return nil, fmt.Errorf("expected 1 rule, got %d", len(rules))
	}
	return &rules[0], nil
}

func (p *Parser) parseFile(raw string) ([]Rule, error) {
	var rules []Rule

	for !p.atEnd() {
		if p.check(TokenOn) {
			rule, err := p.parseRule()
			if err != nil {
				return nil, err
			}
			rule.Raw = raw
			rules = append(rules, *rule)
		} else if p.check(TokenEOF) {
			break
		} else {
			tok := p.peek()
			return nil, p.error(tok, "expected 'on' to start a rule")
		}
	}

	return rules, nil
}

func (p *Parser) parseRule() (*Rule, error) {
	onTok := p.peek()
	if !p.match(TokenOn) {
		return nil, p.error(onTok, "expected 'on'")
	}

	rule := &Rule{Line: onTok.Line}

	// Parse event
	event, err := p.parseEvent()
	if err != nil {
		return nil, err
	}
	rule.Event = *event

	// Expect ->
	if !p.match(TokenArrow) {
		tok := p.peek()
		return nil, p.error(tok, "expected '->' after event")
	}

	// Parse actions
	actions, err := p.parseActions()
	if err != nil {
		return nil, err
	}
	rule.Actions = actions

	// Optional condition
	if p.check(TokenIf) {
		cond, err := p.parseConditionClause()
		if err != nil {
			return nil, err
		}
		rule.Condition = cond
	}

	return rule, nil
}

func (p *Parser) parseEvent() (*Event, error) {
	tok := p.peek()
	if tok.Type != TokenIdent && !isEventKeyword(tok.Type) {
		return nil, p.error(tok, "expected event name")
	}
	p.advance()

	eventName := tok.Value

	// Handle hyphenated event names (pre-tool, post-edit, etc.)
	for p.pos < len(p.tokens) && p.tokens[p.pos].Type == TokenIdent &&
		p.pos > 0 && isHyphenatedEvent(eventName+"-"+p.tokens[p.pos].Value) {
		// This is handled by the lexer since identifiers can contain hyphens
		break
	}

	event := &Event{Name: eventName}

	// Optional matcher after colon
	if p.match(TokenColon) {
		matcherTok := p.peek()
		if matcherTok.Type == TokenIdent || matcherTok.Type == TokenString {
			event.Matcher = matcherTok.Value
			p.advance()
		} else {
			return nil, p.error(matcherTok, "expected matcher value after ':'")
		}
	}

	// Resolve event mapping
	if err := resolveEvent(event); err != nil {
		if pe, ok := err.(*ParseError); ok {
			pe.Line = tok.Line
			pe.Col = tok.Col
			return nil, pe
		}
		return nil, &ParseError{Line: tok.Line, Col: tok.Col, Message: err.Error()}
	}

	return event, nil
}

func (p *Parser) parseActions() ([]Action, error) {
	first, err := p.parseAction()
	if err != nil {
		return nil, err
	}
	actions := []Action{first}

	// Chain with ->
	for p.match(TokenArrow) {
		next, err := p.parseAction()
		if err != nil {
			return nil, err
		}
		actions = append(actions, next)
	}

	return actions, nil
}

func (p *Parser) parseAction() (Action, error) {
	tok := p.peek()

	switch tok.Type {
	case TokenNotify:
		return p.parseNotifyAction()
	case TokenRun:
		return p.parseRunAction()
	case TokenDeny:
		return p.parseDenyAction()
	case TokenRequire:
		return p.parseRequireAction()
	case TokenSummarize:
		return p.parseSummarizeAction()
	case TokenLog:
		return p.parseLogAction()
	default:
		return nil, p.error(tok, fmt.Sprintf("expected action (notify, run, deny, require, summarize, log), got %q", tok.Value))
	}
}

func (p *Parser) parseNotifyAction() (Action, error) {
	p.advance() // consume "notify"

	// Channel name
	chanTok := p.peek()
	if chanTok.Type != TokenIdent {
		return nil, p.error(chanTok, "expected channel name after 'notify'")
	}
	p.advance()

	action := NotifyAction{Channel: chanTok.Value}

	// Optional message
	if p.check(TokenString) {
		action.Message = p.peek().Value
		p.advance()
	}

	return action, nil
}

func (p *Parser) parseRunAction() (Action, error) {
	p.advance() // consume "run"

	cmdTok := p.peek()
	if cmdTok.Type != TokenString {
		return nil, p.error(cmdTok, "expected quoted command after 'run'")
	}
	p.advance()

	action := RunAction{Command: cmdTok.Value}

	// Optional "async"
	if p.check(TokenAsync) {
		action.Async = true
		p.advance()
	}

	return action, nil
}

func (p *Parser) parseDenyAction() (Action, error) {
	p.advance() // consume "deny"

	action := DenyAction{}

	// Optional reason
	if p.check(TokenString) {
		action.Reason = p.peek().Value
		p.advance()
	}

	return action, nil
}

func (p *Parser) parseRequireAction() (Action, error) {
	p.advance() // consume "require"

	checkTok := p.peek()
	if checkTok.Type != TokenIdent {
		return nil, p.error(checkTok, "expected check name after 'require'")
	}
	p.advance()

	name := checkTok.Value
	// Support hyphenated names like "tests-pass"
	// Already handled by the lexer since identifiers can contain hyphens

	return RequireAction{Check: name}, nil
}

func (p *Parser) parseSummarizeAction() (Action, error) {
	p.advance() // consume "summarize"
	return SummarizeAction{}, nil
}

func (p *Parser) parseLogAction() (Action, error) {
	p.advance() // consume "log"

	action := LogAction{}

	// Optional target
	if p.check(TokenIdent) && !isConditionStart(p.peek().Type) {
		action.Target = p.peek().Value
		p.advance()
	}

	return action, nil
}

func (p *Parser) parseConditionClause() (Condition, error) {
	if !p.match(TokenIf) {
		return nil, p.error(p.peek(), "expected 'if'")
	}
	return p.parseExpr()
}

func (p *Parser) parseExpr() (Condition, error) {
	left, err := p.parseSimpleExpr()
	if err != nil {
		return nil, err
	}

	for p.check(TokenAnd) || p.check(TokenOr) {
		op := p.peek().Type
		p.advance()

		right, err := p.parseSimpleExpr()
		if err != nil {
			return nil, err
		}

		if op == TokenAnd {
			left = AndCondition{Left: left, Right: right}
		} else {
			left = OrCondition{Left: left, Right: right}
		}
	}

	return left, nil
}

func (p *Parser) parseSimpleExpr() (Condition, error) {
	tok := p.peek()

	switch tok.Type {
	case TokenNot:
		p.advance()
		inner, err := p.parseSimpleExpr()
		if err != nil {
			return nil, err
		}
		return NotCondition{Cond: inner}, nil

	case TokenElapsed:
		return p.parseElapsedExpr()

	case TokenAway:
		p.advance()
		return FocusCondition{State: "away"}, nil

	case TokenFocused:
		p.advance()
		return FocusCondition{State: "focused"}, nil

	case TokenIdle:
		return p.parseIdleExpr()

	case TokenMatches:
		return p.parseMatchExpr("")

	case TokenFile:
		p.advance()
		if !p.match(TokenMatches) {
			return nil, p.error(p.peek(), "expected 'matches' after 'file'")
		}
		return p.parseMatchPattern("file")

	case TokenCommand:
		p.advance()
		if !p.match(TokenMatches) {
			return nil, p.error(p.peek(), "expected 'matches' after 'command'")
		}
		return p.parseMatchPattern("command")

	default:
		return nil, p.error(tok, fmt.Sprintf("expected condition (elapsed, away, focused, idle, matches, file, command, not), got %q", tok.Value))
	}
}

func (p *Parser) parseElapsedExpr() (Condition, error) {
	p.advance() // consume "elapsed"

	op, err := p.parseComparison()
	if err != nil {
		return nil, err
	}

	dur, err := p.parseDuration()
	if err != nil {
		return nil, err
	}

	return ElapsedCondition{Op: op, Duration: dur}, nil
}

func (p *Parser) parseIdleExpr() (Condition, error) {
	p.advance() // consume "idle"

	cond := FocusCondition{State: "idle"}

	// Optional comparison and duration
	if isComparison(p.peek().Type) {
		op, err := p.parseComparison()
		if err != nil {
			return nil, err
		}
		dur, err := p.parseDuration()
		if err != nil {
			return nil, err
		}
		cond.Op = op
		cond.Duration = dur
	}

	return cond, nil
}

func (p *Parser) parseMatchExpr(kind string) (Condition, error) {
	p.advance() // consume "matches"
	return p.parseMatchPattern(kind)
}

func (p *Parser) parseMatchPattern(kind string) (Condition, error) {
	tok := p.peek()

	// Check for deny-list: pattern
	if tok.Type == TokenIdent && tok.Value == "deny-list" {
		p.advance()
		if !p.match(TokenColon) {
			return nil, p.error(p.peek(), "expected ':' after 'deny-list'")
		}
		nameTok := p.peek()
		if nameTok.Type != TokenIdent {
			return nil, p.error(nameTok, "expected deny list name")
		}
		p.advance()
		return MatchCondition{Kind: kind, Pattern: nameTok.Value, IsDenyList: true}, nil
	}

	// Regular pattern
	if tok.Type != TokenString {
		return nil, p.error(tok, "expected pattern string or 'deny-list:name'")
	}
	p.advance()
	return MatchCondition{Kind: kind, Pattern: tok.Value}, nil
}

func (p *Parser) parseComparison() (string, error) {
	tok := p.peek()
	switch tok.Type {
	case TokenGT:
		p.advance()
		return ">", nil
	case TokenLT:
		p.advance()
		return "<", nil
	case TokenGTE:
		p.advance()
		return ">=", nil
	case TokenLTE:
		p.advance()
		return "<=", nil
	case TokenEQ:
		p.advance()
		return "=", nil
	default:
		return "", p.error(tok, "expected comparison operator (>, <, >=, <=, =)")
	}
}

func (p *Parser) parseDuration() (time.Duration, error) {
	tok := p.peek()

	if tok.Type == TokenDuration {
		p.advance()
		return parseDurationString(tok.Value, tok.Line, tok.Col)
	}

	if tok.Type == TokenNumber {
		p.advance()
		// Check for unit suffix
		nextTok := p.peek()
		if nextTok.Type == TokenIdent && (nextTok.Value == "s" || nextTok.Value == "m" || nextTok.Value == "h") {
			p.advance()
			return parseDurationString(tok.Value+nextTok.Value, tok.Line, tok.Col)
		}
		return 0, &ParseError{
			Line:       tok.Line,
			Col:        tok.Col,
			Message:    fmt.Sprintf("duration %q missing time unit", tok.Value),
			Suggestion: "use s, m, or h (e.g., 30s, 5m, 2h)",
		}
	}

	return 0, p.error(tok, "expected duration (e.g., 30s, 5m, 2h)")
}

func parseDurationString(s string, line, col int) (time.Duration, error) {
	if len(s) < 2 {
		return 0, &ParseError{Line: line, Col: col, Message: "invalid duration: " + s}
	}

	unit := s[len(s)-1]
	numStr := s[:len(s)-1]
	n, err := strconv.Atoi(numStr)
	if err != nil {
		return 0, &ParseError{Line: line, Col: col, Message: "invalid duration number: " + numStr}
	}

	switch unit {
	case 's':
		return time.Duration(n) * time.Second, nil
	case 'm':
		return time.Duration(n) * time.Minute, nil
	case 'h':
		return time.Duration(n) * time.Hour, nil
	default:
		return 0, &ParseError{
			Line:       line,
			Col:        col,
			Message:    fmt.Sprintf("unknown time unit %q in %q", string(unit), s),
			Suggestion: "use s, m, or h",
		}
	}
}

// Helper methods

func (p *Parser) peek() Token {
	if p.pos >= len(p.tokens) {
		return Token{Type: TokenEOF}
	}
	return p.tokens[p.pos]
}

func (p *Parser) advance() Token {
	tok := p.peek()
	if p.pos < len(p.tokens) {
		p.pos++
	}
	return tok
}

func (p *Parser) check(t TokenType) bool {
	return p.peek().Type == t
}

func (p *Parser) match(t TokenType) bool {
	if p.check(t) {
		p.advance()
		return true
	}
	return false
}

func (p *Parser) atEnd() bool {
	return p.peek().Type == TokenEOF
}

func (p *Parser) error(tok Token, msg string) *ParseError {
	return &ParseError{Line: tok.Line, Col: tok.Col, Message: msg}
}

func isComparison(t TokenType) bool {
	return t == TokenGT || t == TokenLT || t == TokenGTE || t == TokenLTE || t == TokenEQ
}

func isConditionStart(t TokenType) bool {
	return t == TokenIf || t == TokenElapsed || t == TokenAway || t == TokenFocused ||
		t == TokenIdle || t == TokenMatches || t == TokenFile || t == TokenCommand || t == TokenNot
}

func isEventKeyword(t TokenType) bool {
	return t == TokenIdent || t == TokenCommand || t == TokenFile || t == TokenLog
}

// isHyphenatedEvent checks if a string matches a known DSL event name.
func isHyphenatedEvent(s string) bool {
	_, ok := eventMap[s]
	return ok
}

// Event name → Claude Code event mapping.
var eventMap = map[string]struct {
	hookEvent      string
	defaultMatcher string
}{
	"stop":           {hookEvent: "Stop"},
	"pre-tool":       {hookEvent: "PreToolUse", defaultMatcher: "*"},
	"post-tool":      {hookEvent: "PostToolUse", defaultMatcher: "*"},
	"tool-failure":   {hookEvent: "PostToolUseFailure", defaultMatcher: "*"},
	"pre-bash":       {hookEvent: "PreToolUse", defaultMatcher: "Bash"},
	"post-bash":      {hookEvent: "PostToolUse", defaultMatcher: "Bash"},
	"pre-edit":       {hookEvent: "PreToolUse", defaultMatcher: "Edit|Write"},
	"post-edit":      {hookEvent: "PostToolUse", defaultMatcher: "Edit|Write"},
	"notification":   {hookEvent: "Notification", defaultMatcher: "*"},
	"permission":     {hookEvent: "PermissionRequest", defaultMatcher: "*"},
	"session-start":  {hookEvent: "SessionStart", defaultMatcher: "*"},
	"session-end":    {hookEvent: "SessionEnd", defaultMatcher: "*"},
	"pre-compact":    {hookEvent: "PreCompact", defaultMatcher: "*"},
	"subagent-start": {hookEvent: "SubagentStart", defaultMatcher: "*"},
	"subagent-stop":  {hookEvent: "SubagentStop", defaultMatcher: "*"},
}

func resolveEvent(event *Event) error {
	mapping, ok := eventMap[event.Name]
	if !ok {
		suggestion := suggestEvent(event.Name)
		msg := fmt.Sprintf("unknown event %q", event.Name)
		if suggestion != "" {
			return &ParseError{Message: msg, Suggestion: fmt.Sprintf("did you mean %q?", suggestion)}
		}
		return &ParseError{Message: msg}
	}

	event.HookEvent = mapping.hookEvent
	event.DefaultMatcher = mapping.defaultMatcher

	// If user provided a matcher, use it. Otherwise use default.
	if event.Matcher == "" {
		event.Matcher = mapping.defaultMatcher
	}

	return nil
}

// suggestEvent returns a suggestion for a misspelled event name.
func suggestEvent(name string) string {
	best := ""
	bestDist := 999

	for candidate := range eventMap {
		d := levenshtein(name, candidate)
		if d < bestDist && d <= 3 {
			best = candidate
			bestDist = d
		}
	}
	return best
}

// levenshtein computes the edit distance between two strings.
func levenshtein(a, b string) int {
	la, lb := len(a), len(b)
	if la == 0 {
		return lb
	}
	if lb == 0 {
		return la
	}

	prev := make([]int, lb+1)
	curr := make([]int, lb+1)

	for j := 0; j <= lb; j++ {
		prev[j] = j
	}

	for i := 1; i <= la; i++ {
		curr[0] = i
		for j := 1; j <= lb; j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			curr[j] = min(curr[j-1]+1, min(prev[j]+1, prev[j-1]+cost))
		}
		prev, curr = curr, prev
	}

	return prev[lb]
}
