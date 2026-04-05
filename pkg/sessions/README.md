# pkg/sessions

Package `sessions` parses Claude Code session JSONL transcripts and analyzes them for problems.

## Package Purpose

Claude Code writes one JSONL file per conversation to `~/.claude/projects/<encoded-path>/<uuid>.jsonl`. This package provides:

- Full parsing of those transcripts into structured Go types
- Discovery of projects and sessions on disk
- Loading of subagent transcripts linked to a parent session
- Heuristic problem detection (repeated reads, compaction loops, excessive subagents, etc.)

The package is intentionally pure — it has no dependency on the Hitch database or CLI. Cost estimation is injected as a callback so callers can wire in `internal/pricing` without creating a circular import.

## Key Types

### ParsedSession

The fully-parsed representation of one Claude Code session:

```go
type ParsedSession struct {
    ID             string
    ProjectDir     string         // decoded from ~/.claude/projects/<encoded>
    TranscriptPath string
    StartedAt      time.Time
    EndedAt        time.Time
    Model          string
    Messages       []Message
    TokenUsage     TokenUsage
    ToolCalls      []ToolCall     // denormalized for easy aggregation
    Compactions    []CompactionEvent
    Subagents      []SubagentInfo
    FileReadCounts map[string]int // path -> read count across the main session
}
```

### Message

One turn in the conversation:

```go
type Message struct {
    UUID        string
    Role        string         // "user" or "assistant"
    Timestamp   time.Time
    Model       string
    MessageID   string         // from the API; used for deduplication
    StopReason  string
    Content     []ContentBlock
    Usage       TokenUsage
    IsCompacted bool           // true if produced by context compaction
}
```

### ToolCall

A denormalized record extracted from `Message.Content` for easy aggregation. Fields are populated based on which tool was called:

```go
type ToolCall struct {
    ToolName   string
    FilePath   string    // Read, Edit, Write, Glob, Grep
    Command    string    // Bash
    Pattern    string    // Glob, Grep
    ResultSize int       // bytes returned in the tool_result
    Timestamp  time.Time
    MessageID  string
    IsError    bool
}
```

### SubagentInfo

Summary of a subagent's activity, derived from its own JSONL in `<session-dir>/subagents/`:

```go
type SubagentInfo struct {
    SessionID      string
    ParentID       string
    AgentName      string         // from agent-<id>.meta.json (optional)
    AgentType      string         // from agent-<id>.meta.json (optional)
    Model          string
    StartedAt      time.Time
    EndedAt        time.Time
    TokenUsage     TokenUsage
    ToolCalls      []ToolCall
    Compactions    []CompactionEvent
    FileReads      []string       // deduplicated, sorted
    FileReadCounts map[string]int
}
```

### TokenUsage

Token counts from the Anthropic API, with helpers:

```go
type TokenUsage struct {
    InputTokens         int
    OutputTokens        int
    CacheReadTokens     int
    CacheCreationTokens int
    EstimatedCost       float64  // zero if ParseSession called with nil CostEstimator
}

func (t TokenUsage) RateLimit() int  // input + output + cache_creation (excludes cache reads)
func (t TokenUsage) Total() int      // sum of all four categories
```

### SessionSummary

Lightweight description built without full parsing (reads only the first 20 lines):

```go
type SessionSummary struct {
    ID             string
    ProjectDir     string
    TranscriptPath string
    FileSize       int64
    StartedAt      time.Time
    LastModified   time.Time
    IsActive       bool    // mtime within last 5 minutes
    MessageCount   int
}
```

### ProjectInfo

One entry under `~/.claude/projects/`:

```go
type ProjectInfo struct {
    EncodedName  string    // directory name on disk
    OriginalPath string    // decoded project path
    DirPath      string    // absolute path to project directory
    SessionCount int
    LastActivity time.Time
}
```

## Functions

### ParseSession

```go
func ParseSession(transcriptPath string, log Logger, cost CostEstimator) (*ParsedSession, error)
```

Fully parses a JSONL transcript. Malformed lines are skipped with a warning written to `log` (pass `nil` to suppress). `cost` is an optional callback to compute `EstimatedCost`; pass `nil` to leave it zero.

Streaming duplicates (same `message.id` appearing twice) are deduplicated. Streaming partial chunks (messages with no `stop_reason`) are skipped. Subagents are loaded recursively up to depth 5.

### ParseSessionSummary

```go
func ParseSessionSummary(transcriptPath string) (*SessionSummary, error)
```

Fast summary path that reads only the first 20 lines. Suitable for listing hundreds of sessions without full parsing.

### DiscoverProjects

```go
func DiscoverProjects(claudeDir string) ([]ProjectInfo, error)
```

Scans `claudeDir/projects/` and returns one `ProjectInfo` per subdirectory, sorted by last activity descending. `claudeDir` is typically `~/.claude`. Returns an error if the `projects/` directory does not exist.

Project original paths are resolved from `sessions-index.json` when present, falling back to heuristic decoding of the encoded directory name (e.g. `-home-user-dev-hitch` → `/home/user/dev/hitch`).

### DiscoverSessions

```go
func DiscoverSessions(projectDir string) ([]SessionSummary, error)
```

Returns lightweight summaries for all UUID-named JSONL files in `projectDir`, sorted by last-modified descending. Files whose names are not valid UUIDs (e.g. legacy `agent-*.jsonl`) are filtered out.

### LoadSubagents

```go
func LoadSubagents(sessionDir string) ([]SubagentInfo, error)
```

Finds and parses subagent JSONL files in `<sessionDir>/subagents/`. `sessionDir` is the transcript path with `.jsonl` stripped. Subagents that spawned their own subagents are loaded recursively up to depth 5. Returns `nil, nil` (not an error) when the `subagents/` directory does not exist.

Optional metadata is loaded from `agent-<id>.meta.json` alongside each JSONL file.

### DetectProblems

```go
func DetectProblems(s *ParsedSession, cfg ProblemConfig) Problems
```

Runs all heuristics against a fully-parsed session. Returns a `Problems` struct with slices for each category (empty slice = no problems of that type). `DefaultProblemConfig()` returns the standard thresholds.

### ParseProjectDir

```go
func ParseProjectDir(transcriptPath string) string
```

Extracts the decoded project directory path from a transcript file path. Reads `sessions-index.json` from the parent directory when present; otherwise heuristically decodes the encoded directory name.

## Problem Heuristics

All thresholds come from `ProblemConfig` (defaults from `DefaultProblemConfig()`).

| Problem | Trigger condition | Default threshold |
|---|---|---|
| **RepeatedReads** | Same file read >= N times in main session or a subagent | 3 reads |
| **CompactionLoops** | File read before a compaction is re-read within N minutes after it | 10-minute window |
| **ModelMismatches** | Subagent uses Opus while the parent session also uses Opus | N/A (always flagged) |
| **ExcessiveSubagents** | Session spawned >= N subagents | 10 subagents |
| **ContextFillNoProgress** | input > 50,000 tokens and output/(input+output) < ratio threshold | 5% output ratio |

`RepeatedReads` and `ContextFillNoProgress` are checked independently for the main session and each subagent. `CompactionLoops` and `ModelMismatches` are checked only at the session level.

## CostEstimator Interface

```go
type CostEstimator func(model string, inputTokens, outputTokens, cacheReadTokens, cacheCreationTokens int) float64
```

Pass `pricing.LoadPricing().EstimateCost` from `internal/pricing` to get accurate cost estimates. The `pkg/sessions` package itself does not import `internal/pricing` to avoid a dependency cycle.

## Logger Interface

```go
type Logger interface {
    Warnf(format string, args ...interface{})
}
```

Any type with a `Warnf` method satisfies this interface. Pass `nil` to suppress all warnings.
