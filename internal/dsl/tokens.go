package dsl

// TokenType represents the type of a lexer token.
type TokenType int

const (
	// Special tokens
	TokenEOF TokenType = iota
	TokenError

	// Literals
	TokenIdent      // identifier
	TokenString     // "quoted string"
	TokenNumber     // 123
	TokenDuration   // 30s, 5m, 2h (number + unit combined during parsing)

	// Keywords
	TokenOn
	TokenIf
	TokenAnd
	TokenOr
	TokenNot
	TokenNotify
	TokenRun
	TokenDeny
	TokenRequire
	TokenSummarize
	TokenLog
	TokenElapsed
	TokenAway
	TokenFocused
	TokenIdle
	TokenMatches
	TokenFile
	TokenCommand
	TokenAsync

	// Punctuation
	TokenArrow    // ->
	TokenColon    // :
	TokenGT       // >
	TokenLT       // <
	TokenGTE      // >=
	TokenLTE      // <=
	TokenEQ       // =

	// Comment
	TokenComment
)

// Token is a lexer token with position information.
type Token struct {
	Type  TokenType
	Value string
	Line  int
	Col   int
}

var keywords = map[string]TokenType{
	"on":        TokenOn,
	"if":        TokenIf,
	"and":       TokenAnd,
	"or":        TokenOr,
	"not":       TokenNot,
	"notify":    TokenNotify,
	"run":       TokenRun,
	"deny":      TokenDeny,
	"require":   TokenRequire,
	"summarize": TokenSummarize,
	"log":       TokenLog,
	"elapsed":   TokenElapsed,
	"away":      TokenAway,
	"focused":   TokenFocused,
	"idle":      TokenIdle,
	"matches":   TokenMatches,
	"file":      TokenFile,
	"command":   TokenCommand,
	"async":     TokenAsync,
}

// String returns the token type name.
func (t TokenType) String() string {
	switch t {
	case TokenEOF:
		return "EOF"
	case TokenError:
		return "error"
	case TokenIdent:
		return "identifier"
	case TokenString:
		return "string"
	case TokenNumber:
		return "number"
	case TokenDuration:
		return "duration"
	case TokenOn:
		return "on"
	case TokenIf:
		return "if"
	case TokenAnd:
		return "and"
	case TokenOr:
		return "or"
	case TokenNot:
		return "not"
	case TokenNotify:
		return "notify"
	case TokenRun:
		return "run"
	case TokenDeny:
		return "deny"
	case TokenRequire:
		return "require"
	case TokenSummarize:
		return "summarize"
	case TokenLog:
		return "log"
	case TokenElapsed:
		return "elapsed"
	case TokenAway:
		return "away"
	case TokenFocused:
		return "focused"
	case TokenIdle:
		return "idle"
	case TokenMatches:
		return "matches"
	case TokenFile:
		return "file"
	case TokenCommand:
		return "command"
	case TokenAsync:
		return "async"
	case TokenArrow:
		return "->"
	case TokenColon:
		return ":"
	case TokenGT:
		return ">"
	case TokenLT:
		return "<"
	case TokenGTE:
		return ">="
	case TokenLTE:
		return "<="
	case TokenEQ:
		return "="
	case TokenComment:
		return "comment"
	default:
		return "unknown"
	}
}
