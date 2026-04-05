// Package sessions parses Claude Code session JSONL transcripts.
package sessions

import (
	"encoding/json"
	"time"
)

// CostEstimator is the callback pkg/sessions uses to compute estimated costs.
// The CLI layer passes in the shared internal/pricing function so all cost
// calculations use a single canonical implementation.
type CostEstimator func(model string, inputTokens, outputTokens, cacheReadTokens, cacheCreationTokens int) float64

// Logger is the minimal interface accepted by ParseSession for warning output.
type Logger interface {
	Warnf(format string, args ...interface{})
}

// TokenUsage holds the four token categories returned by the Anthropic API.
// EstimatedCost is computed at parse time using the CostEstimator callback
// passed to ParseSession. It is zero when ParseSession is called with a nil
// CostEstimator.
type TokenUsage struct {
	InputTokens         int
	OutputTokens        int
	CacheReadTokens     int
	CacheCreationTokens int
	EstimatedCost       float64
}

// RateLimit returns the subset of tokens that count toward the API rate limit:
// input + output + cache_creation. Cache reads are excluded per Anthropic docs.
func (t TokenUsage) RateLimit() int {
	return t.InputTokens + t.OutputTokens + t.CacheCreationTokens
}

// Total returns the sum of all four token categories.
func (t TokenUsage) Total() int {
	return t.InputTokens + t.OutputTokens + t.CacheReadTokens + t.CacheCreationTokens
}

// ContentBlock represents one element inside an assistant or user message.
type ContentBlock struct {
	Type       string           // "text", "tool_use", "tool_result", "thinking"
	Text       string           // populated for "text" and "thinking"
	ToolUse    *ToolUseBlock
	ToolResult *ToolResultBlock
}

// ToolUseBlock holds a single tool invocation found in an assistant message.
type ToolUseBlock struct {
	ID    string                 // tool_use_id from the API
	Name  string                 // e.g. "Read", "Bash", "Edit"
	Input map[string]interface{} // raw parsed input
}

// ToolResultBlock holds the result of a tool invocation found in a user message.
type ToolResultBlock struct {
	ToolUseID string
	Content   string // joined text content
	IsError   bool
	SizeBytes int // len(raw content before joining)
}

// Message represents one turn in the conversation log.
type Message struct {
	UUID        string
	Role        string    // "user", "assistant", "system"
	Timestamp   time.Time
	Model       string    // non-empty on assistant messages
	MessageID   string    // message.id from the API (used for deduplication)
	StopReason  string    // non-empty on final assistant chunks
	Content     []ContentBlock
	Usage       TokenUsage // zero on user messages
	IsCompacted bool       // true if this message was produced by compaction
}

// ToolCall is a denormalized record of a single tool invocation — extracted
// from Message.Content for easy aggregation without re-scanning.
type ToolCall struct {
	ToolName   string
	FilePath   string    // extracted from Read/Edit/Write/Glob/Grep inputs
	Command    string    // extracted from Bash input
	Pattern    string    // extracted from Glob/Grep inputs
	ResultSize int       // bytes of result returned
	Timestamp  time.Time
	MessageID  string    // links back to the parent Message
	IsError    bool
}

// CompactionEvent represents a detected context compaction.
type CompactionEvent struct {
	Timestamp      time.Time
	MessagesBefore int // message count before this compaction
	MessagesAfter  int // message count after (typically 1 summary message)
	TokensBefore   int // sum of InputTokens in the N messages before
	TokensAfter    int // InputTokens of the summary message
}

// SubagentInfo summarises a subagent's activity derived from its own JSONL.
type SubagentInfo struct {
	SessionID       string
	ParentID        string
	AgentName       string // optional: from SubagentStart hook event in hitch.db
	AgentType       string // optional: from SubagentStart hook event in hitch.db
	Model           string // most-used model in subagent transcript
	StartedAt       time.Time
	EndedAt         time.Time
	TokenUsage      TokenUsage
	ToolCalls       []ToolCall
	Compactions     []CompactionEvent
	FileReads       []string       // deduplicated sorted list of file paths read
	FileReadCounts  map[string]int // file path -> read count
}

// ParsedSession is the fully-parsed representation of one Claude Code session.
type ParsedSession struct {
	ID             string
	ProjectDir     string    // decoded from ~/.claude/projects/<encoded>
	TranscriptPath string    // absolute path to the JSONL file
	StartedAt      time.Time
	EndedAt        time.Time
	Model          string    // most-used model
	Messages       []Message
	TokenUsage     TokenUsage
	ToolCalls      []ToolCall
	Compactions    []CompactionEvent
	Subagents      []SubagentInfo
	FileReadCounts map[string]int // file path -> read count across main session
}

// SessionSummary is a lightweight description built without full parsing.
type SessionSummary struct {
	ID             string
	ProjectDir     string
	TranscriptPath string
	FileSize       int64
	StartedAt      time.Time
	LastModified   time.Time
	IsActive       bool // mtime within last 5 minutes = file still being written
	MessageCount   int  // approximate: count of JSONL lines
}

// ProjectInfo describes one encoded project directory under ~/.claude/projects/.
type ProjectInfo struct {
	EncodedName  string // the directory name as stored on disk
	OriginalPath string // decoded project path; from sessions-index.json if present
	DirPath      string // absolute path to the project directory
	SessionCount int
	LastActivity time.Time
}

// Problems is the result of running problem-detection heuristics on a ParsedSession.
type Problems struct {
	RepeatedReads          []RepeatedReadProblem
	CompactionLoops        []CompactionLoopProblem
	ModelMismatches        []ModelMismatchProblem
	ExcessiveSubagents     *ExcessiveSubagentsProblem // nil if not triggered
	ContextFillNoProgress  []ContextFillProblem
}

// RepeatedReadProblem is raised when a file is read 3 or more times.
type RepeatedReadProblem struct {
	FilePath   string
	ReadCount  int
	SubagentID string // empty if from the main session
}

// CompactionLoopProblem is raised when a compaction is followed by re-reads
// of the same large files within a short window.
type CompactionLoopProblem struct {
	CompactionAt  time.Time
	RereadFiles   []string // files read after the compaction that were read before
	WindowMinutes int
}

// ModelMismatchProblem is raised when a subagent uses Opus while the parent
// session also uses Opus.
type ModelMismatchProblem struct {
	SubagentID    string
	SubagentModel string
	ParentModel   string
}

// ExcessiveSubagentsProblem is raised when a session spawns more subagents
// than the configured threshold.
type ExcessiveSubagentsProblem struct {
	SubagentCount int
	Threshold     int
}

// ContextFillProblem is raised when a session has high input tokens but
// very low output tokens.
type ContextFillProblem struct {
	SubagentID   string // empty if main session
	InputTokens  int
	OutputTokens int
	OutputRatio  float64 // output / (input + output)
}

// ProblemConfig holds thresholds for the problem-detection heuristics.
type ProblemConfig struct {
	RepeatedReadThreshold   int
	CompactionLoopWindowMin int
	ExcessiveSubagentCount  int
	ContextFillOutputRatio  float64
}

// DefaultProblemConfig returns the default thresholds.
func DefaultProblemConfig() ProblemConfig {
	return ProblemConfig{
		RepeatedReadThreshold:   3,
		CompactionLoopWindowMin: 10,
		ExcessiveSubagentCount:  10,
		ContextFillOutputRatio:  0.05,
	}
}

// rawLine is an intermediate representation of a parsed JSONL line.
type rawLine struct {
	Type        string              `json:"type"`
	UUID        string              `json:"uuid"`
	SessionID   string              `json:"sessionId"`
	Timestamp   json.RawMessage     `json:"timestamp"`
	IsSidechain bool                `json:"isSidechain"`
	Message     *rawMessage         `json:"message"`
	// for progress lines
	ToolUseID       string `json:"toolUseID"`
	ParentToolUseID string `json:"parentToolUseID"`
}

type rawMessage struct {
	ID         string              `json:"id"`
	Role       string              `json:"role"`
	Model      string              `json:"model"`
	StopReason *string             `json:"stop_reason"`
	Usage      *rawUsage           `json:"usage"`
	Content    json.RawMessage     `json:"content"`
}

type rawUsage struct {
	InputTokens              int `json:"input_tokens"`
	OutputTokens             int `json:"output_tokens"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
}

type rawContentBlock struct {
	Type      string              `json:"type"`
	Text      string              `json:"text"`
	ID        string              `json:"id"`
	Name      string              `json:"name"`
	Input     json.RawMessage     `json:"input"`
	ToolUseID string              `json:"tool_use_id"`
	Content   json.RawMessage     `json:"content"`
	IsError   bool                `json:"is_error"`
}
