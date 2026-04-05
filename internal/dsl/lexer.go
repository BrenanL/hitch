package dsl

import (
	"fmt"
	"unicode"
)

// Lexer tokenizes DSL input.
type Lexer struct {
	input  string
	pos    int
	line   int
	col    int
	tokens []Token
}

// NewLexer creates a new lexer for the given input.
func NewLexer(input string) *Lexer {
	return &Lexer{
		input: input,
		line:  1,
		col:   1,
	}
}

// Tokenize scans the entire input and returns all tokens.
func (l *Lexer) Tokenize() ([]Token, error) {
	for {
		tok, err := l.nextToken()
		if err != nil {
			return nil, err
		}
		if tok.Type == TokenComment {
			continue // skip comments
		}
		l.tokens = append(l.tokens, tok)
		if tok.Type == TokenEOF {
			break
		}
	}
	return l.tokens, nil
}

func (l *Lexer) nextToken() (Token, error) {
	l.skipWhitespace()

	if l.pos >= len(l.input) {
		return Token{Type: TokenEOF, Line: l.line, Col: l.col}, nil
	}

	ch := l.input[l.pos]
	startLine, startCol := l.line, l.col

	// Comment
	if ch == '#' {
		return l.scanComment(), nil
	}

	// String
	if ch == '"' {
		return l.scanString()
	}

	// Number
	if ch >= '0' && ch <= '9' {
		return l.scanNumber(), nil
	}

	// Arrow ->
	if ch == '-' && l.pos+1 < len(l.input) && l.input[l.pos+1] == '>' {
		l.advance()
		l.advance()
		return Token{Type: TokenArrow, Value: "->", Line: startLine, Col: startCol}, nil
	}

	// Operators
	switch ch {
	case ':':
		l.advance()
		return Token{Type: TokenColon, Value: ":", Line: startLine, Col: startCol}, nil
	case '>':
		l.advance()
		if l.pos < len(l.input) && l.input[l.pos] == '=' {
			l.advance()
			return Token{Type: TokenGTE, Value: ">=", Line: startLine, Col: startCol}, nil
		}
		return Token{Type: TokenGT, Value: ">", Line: startLine, Col: startCol}, nil
	case '<':
		l.advance()
		if l.pos < len(l.input) && l.input[l.pos] == '=' {
			l.advance()
			return Token{Type: TokenLTE, Value: "<=", Line: startLine, Col: startCol}, nil
		}
		return Token{Type: TokenLT, Value: "<", Line: startLine, Col: startCol}, nil
	case '=':
		l.advance()
		if l.pos < len(l.input) && l.input[l.pos] == '=' {
			l.advance()
			return Token{Type: TokenEQEQ, Value: "==", Line: startLine, Col: startCol}, nil
		}
		return Token{Type: TokenEQ, Value: "=", Line: startLine, Col: startCol}, nil
	}

	// Identifier or keyword
	if isIdentStart(ch) {
		return l.scanIdent(), nil
	}

	return Token{}, fmt.Errorf("line %d, col %d: unexpected character %q", l.line, l.col, ch)
}

func (l *Lexer) scanComment() Token {
	start := l.pos
	startLine, startCol := l.line, l.col
	for l.pos < len(l.input) && l.input[l.pos] != '\n' {
		l.advance()
	}
	return Token{Type: TokenComment, Value: l.input[start:l.pos], Line: startLine, Col: startCol}
}

func (l *Lexer) scanString() (Token, error) {
	startLine, startCol := l.line, l.col
	l.advance() // skip opening quote
	start := l.pos
	var result []byte

	for l.pos < len(l.input) {
		ch := l.input[l.pos]
		if ch == '\\' && l.pos+1 < len(l.input) && l.input[l.pos+1] == '"' {
			result = append(result, l.input[start:l.pos]...)
			result = append(result, '"')
			l.advance() // skip backslash
			l.advance() // skip quote
			start = l.pos
			continue
		}
		if ch == '"' {
			if result != nil {
				result = append(result, l.input[start:l.pos]...)
				l.advance() // skip closing quote
				return Token{Type: TokenString, Value: string(result), Line: startLine, Col: startCol}, nil
			}
			val := l.input[start:l.pos]
			l.advance() // skip closing quote
			return Token{Type: TokenString, Value: val, Line: startLine, Col: startCol}, nil
		}
		if ch == '\n' {
			return Token{}, fmt.Errorf("line %d, col %d: unterminated string", startLine, startCol)
		}
		l.advance()
	}

	return Token{}, fmt.Errorf("line %d, col %d: unterminated string", startLine, startCol)
}

func (l *Lexer) scanNumber() Token {
	start := l.pos
	startLine, startCol := l.line, l.col
	for l.pos < len(l.input) && l.input[l.pos] >= '0' && l.input[l.pos] <= '9' {
		l.advance()
	}
	// Check for decimal point (float literal)
	if l.pos < len(l.input) && l.input[l.pos] == '.' {
		l.advance()
		for l.pos < len(l.input) && l.input[l.pos] >= '0' && l.input[l.pos] <= '9' {
			l.advance()
		}
		return Token{Type: TokenNumber, Value: l.input[start:l.pos], Line: startLine, Col: startCol}
	}
	// Check for duration suffix (s, m, h)
	if l.pos < len(l.input) && (l.input[l.pos] == 's' || l.input[l.pos] == 'm' || l.input[l.pos] == 'h') {
		l.advance()
		return Token{Type: TokenDuration, Value: l.input[start:l.pos], Line: startLine, Col: startCol}
	}
	return Token{Type: TokenNumber, Value: l.input[start:l.pos], Line: startLine, Col: startCol}
}

func (l *Lexer) scanIdent() Token {
	start := l.pos
	startLine, startCol := l.line, l.col
	for l.pos < len(l.input) && isIdentPart(l.input[l.pos]) {
		l.advance()
	}
	value := l.input[start:l.pos]

	// Check if it's a keyword
	if tokType, ok := keywords[value]; ok {
		return Token{Type: tokType, Value: value, Line: startLine, Col: startCol}
	}

	return Token{Type: TokenIdent, Value: value, Line: startLine, Col: startCol}
}

func (l *Lexer) skipWhitespace() {
	for l.pos < len(l.input) {
		ch := l.input[l.pos]
		if ch == '\n' {
			l.line++
			l.col = 0
			l.pos++
			l.col = 1
		} else if ch == ' ' || ch == '\t' || ch == '\r' {
			l.advance()
		} else {
			break
		}
	}
}

func (l *Lexer) advance() {
	if l.pos < len(l.input) {
		l.pos++
		l.col++
	}
}

func isIdentStart(ch byte) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || ch == '_'
}

func isIdentPart(ch byte) bool {
	r := rune(ch)
	return unicode.IsLetter(r) || unicode.IsDigit(r) || ch == '-' || ch == '_'
}
