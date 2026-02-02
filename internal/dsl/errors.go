package dsl

import "fmt"

// ParseError represents a DSL parsing error with location information.
type ParseError struct {
	File       string
	Line       int
	Col        int
	Message    string
	Suggestion string // optional "did you mean?" suggestion
}

func (e *ParseError) Error() string {
	loc := ""
	if e.File != "" {
		loc = e.File + ":"
	}
	if e.Line > 0 {
		loc += fmt.Sprintf("%d", e.Line)
		if e.Col > 0 {
			loc += fmt.Sprintf(":%d", e.Col)
		}
		loc += ": "
	}
	msg := loc + e.Message
	if e.Suggestion != "" {
		msg += " — " + e.Suggestion
	}
	return msg
}

// newParseError creates a new ParseError.
func newParseError(line, col int, msg string) *ParseError {
	return &ParseError{Line: line, Col: col, Message: msg}
}

// newParseErrorWithSuggestion creates a ParseError with a suggestion.
func newParseErrorWithSuggestion(line, col int, msg, suggestion string) *ParseError {
	return &ParseError{Line: line, Col: col, Message: msg, Suggestion: suggestion}
}

// ValidateError represents a semantic validation error.
type ValidateError struct {
	Line       int
	Message    string
	Suggestion string
	IsWarning  bool // warnings don't prevent execution
}

func (e *ValidateError) Error() string {
	prefix := "error"
	if e.IsWarning {
		prefix = "warning"
	}
	msg := fmt.Sprintf("line %d: %s: %s", e.Line, prefix, e.Message)
	if e.Suggestion != "" {
		msg += " — " + e.Suggestion
	}
	return msg
}
