package dsl

import (
	"testing"
)

func TestLexerBasicRule(t *testing.T) {
	input := `on stop -> notify discord if elapsed > 30s`
	tokens, err := NewLexer(input).Tokenize()
	if err != nil {
		t.Fatalf("Tokenize: %v", err)
	}

	expected := []TokenType{
		TokenOn, TokenIdent, TokenArrow, TokenNotify, TokenIdent,
		TokenIf, TokenElapsed, TokenGT, TokenDuration, TokenEOF,
	}

	if len(tokens) != len(expected) {
		t.Fatalf("token count = %d, want %d", len(tokens), len(expected))
	}
	for i, exp := range expected {
		if tokens[i].Type != exp {
			t.Errorf("token[%d] = %v (%q), want %v", i, tokens[i].Type, tokens[i].Value, exp)
		}
	}
}

func TestLexerStringToken(t *testing.T) {
	input := `run "npm test" async`
	tokens, err := NewLexer(input).Tokenize()
	if err != nil {
		t.Fatalf("Tokenize: %v", err)
	}

	if tokens[1].Type != TokenString {
		t.Errorf("token[1] type = %v, want string", tokens[1].Type)
	}
	if tokens[1].Value != "npm test" {
		t.Errorf("token[1] value = %q, want %q", tokens[1].Value, "npm test")
	}
	if tokens[2].Type != TokenAsync {
		t.Errorf("token[2] type = %v, want async", tokens[2].Type)
	}
}

func TestLexerEscapedString(t *testing.T) {
	input := `"hello \"world\""`
	tokens, err := NewLexer(input).Tokenize()
	if err != nil {
		t.Fatalf("Tokenize: %v", err)
	}
	if tokens[0].Value != `hello "world"` {
		t.Errorf("value = %q, want %q", tokens[0].Value, `hello "world"`)
	}
}

func TestLexerHyphenatedIdent(t *testing.T) {
	input := `on pre-bash -> deny`
	tokens, err := NewLexer(input).Tokenize()
	if err != nil {
		t.Fatalf("Tokenize: %v", err)
	}

	if tokens[1].Value != "pre-bash" {
		t.Errorf("event = %q, want %q", tokens[1].Value, "pre-bash")
	}
}

func TestLexerComments(t *testing.T) {
	input := "# This is a comment\non stop -> deny"
	tokens, err := NewLexer(input).Tokenize()
	if err != nil {
		t.Fatalf("Tokenize: %v", err)
	}

	// Comments should be filtered out
	if tokens[0].Type != TokenOn {
		t.Errorf("first token = %v, want 'on' (comments should be skipped)", tokens[0].Type)
	}
}

func TestLexerOperators(t *testing.T) {
	tests := []struct {
		input string
		want  TokenType
	}{
		{">", TokenGT},
		{"<", TokenLT},
		{">=", TokenGTE},
		{"<=", TokenLTE},
		{"=", TokenEQ},
		{"->", TokenArrow},
		{":", TokenColon},
	}
	for _, tt := range tests {
		tokens, err := NewLexer(tt.input).Tokenize()
		if err != nil {
			t.Errorf("Tokenize(%q): %v", tt.input, err)
			continue
		}
		if tokens[0].Type != tt.want {
			t.Errorf("Tokenize(%q) = %v, want %v", tt.input, tokens[0].Type, tt.want)
		}
	}
}

func TestLexerDuration(t *testing.T) {
	input := "30s 5m 2h"
	tokens, err := NewLexer(input).Tokenize()
	if err != nil {
		t.Fatalf("Tokenize: %v", err)
	}

	for i := 0; i < 3; i++ {
		if tokens[i].Type != TokenDuration {
			t.Errorf("token[%d] = %v (%q), want duration", i, tokens[i].Type, tokens[i].Value)
		}
	}
	if tokens[0].Value != "30s" {
		t.Errorf("token[0] = %q", tokens[0].Value)
	}
	if tokens[1].Value != "5m" {
		t.Errorf("token[1] = %q", tokens[1].Value)
	}
	if tokens[2].Value != "2h" {
		t.Errorf("token[2] = %q", tokens[2].Value)
	}
}

func TestLexerUnterminatedString(t *testing.T) {
	input := `"unterminated`
	_, err := NewLexer(input).Tokenize()
	if err == nil {
		t.Error("expected error for unterminated string")
	}
}

func TestLexerLineNumbers(t *testing.T) {
	input := "on stop\n-> notify discord"
	tokens, err := NewLexer(input).Tokenize()
	if err != nil {
		t.Fatalf("Tokenize: %v", err)
	}

	if tokens[0].Line != 1 {
		t.Errorf("token[0] line = %d, want 1", tokens[0].Line)
	}
	// Arrow should be on line 2
	if tokens[2].Line != 2 {
		t.Errorf("arrow line = %d, want 2", tokens[2].Line)
	}
}
