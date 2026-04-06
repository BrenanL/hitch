package sessions

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// testLogger captures warnings for assertions.
type testLogger struct {
	warnings []string
}

func (l *testLogger) Warnf(format string, args ...interface{}) {
	// Just record that a warning occurred
	l.warnings = append(l.warnings, format)
}

func testDataPath(name string) string {
	return filepath.Join("testdata", name)
}

// TestParseSimpleSession verifies that a minimal session is parsed correctly.
func TestParseSimpleSession(t *testing.T) {
	ps, err := ParseSession(testDataPath("simple.jsonl"), nil, nil)
	if err != nil {
		t.Fatalf("ParseSession: %v", err)
	}

	if ps.ID != "simple" {
		t.Errorf("ID = %q, want %q", ps.ID, "simple")
	}

	// Expect 2 assistant messages (stop_reason set) and 2 user messages
	assistantCount := 0
	userCount := 0
	for _, m := range ps.Messages {
		switch m.Role {
		case "assistant":
			assistantCount++
		case "user":
			userCount++
		}
	}
	if assistantCount != 2 {
		t.Errorf("assistant messages = %d, want 2", assistantCount)
	}
	if userCount != 2 {
		t.Errorf("user messages = %d, want 2", userCount)
	}

	// Verify model
	if ps.Model != "claude-sonnet-4-6" {
		t.Errorf("Model = %q, want %q", ps.Model, "claude-sonnet-4-6")
	}

	// Verify token accumulation: 100+200=300 input, 20+30=50 output
	if ps.TokenUsage.InputTokens != 300 {
		t.Errorf("InputTokens = %d, want 300", ps.TokenUsage.InputTokens)
	}
	if ps.TokenUsage.OutputTokens != 50 {
		t.Errorf("OutputTokens = %d, want 50", ps.TokenUsage.OutputTokens)
	}
	if ps.TokenUsage.CacheReadTokens != 50 {
		t.Errorf("CacheReadTokens = %d, want 50", ps.TokenUsage.CacheReadTokens)
	}
	if ps.TokenUsage.CacheCreationTokens != 10 {
		t.Errorf("CacheCreationTokens = %d, want 10", ps.TokenUsage.CacheCreationTokens)
	}

	// Verify tool call extracted
	if len(ps.ToolCalls) != 1 {
		t.Fatalf("ToolCalls = %d, want 1", len(ps.ToolCalls))
	}
	tc := ps.ToolCalls[0]
	if tc.ToolName != "Read" {
		t.Errorf("ToolCalls[0].ToolName = %q, want %q", tc.ToolName, "Read")
	}
	if tc.FilePath != "/home/user/dev/project/main.go" {
		t.Errorf("ToolCalls[0].FilePath = %q", tc.FilePath)
	}
	if tc.IsError {
		t.Error("ToolCalls[0].IsError = true, want false")
	}

	// FileReadCounts
	if ps.FileReadCounts["/home/user/dev/project/main.go"] != 1 {
		t.Errorf("FileReadCounts[main.go] = %d, want 1", ps.FileReadCounts["/home/user/dev/project/main.go"])
	}

	// Timing
	if ps.StartedAt.IsZero() {
		t.Error("StartedAt is zero")
	}
	if ps.EndedAt.IsZero() {
		t.Error("EndedAt is zero")
	}
}

// TestParseSessionWithCompaction verifies compaction detection.
func TestParseSessionWithCompaction(t *testing.T) {
	ps, err := ParseSession(testDataPath("compaction.jsonl"), nil, nil)
	if err != nil {
		t.Fatalf("ParseSession: %v", err)
	}

	if len(ps.Compactions) != 1 {
		t.Fatalf("Compactions = %d, want 1", len(ps.Compactions))
	}

	ce := ps.Compactions[0]
	if ce.MessagesBefore == 0 {
		t.Error("CompactionEvent.MessagesBefore = 0, want > 0")
	}
	if ce.MessagesAfter != 1 {
		t.Errorf("CompactionEvent.MessagesAfter = %d, want 1", ce.MessagesAfter)
	}
	if ce.TokensBefore == 0 {
		t.Error("CompactionEvent.TokensBefore = 0, want > 0")
	}
	if ce.Timestamp.IsZero() {
		t.Error("CompactionEvent.Timestamp is zero")
	}
}

// TestMalformedLineHandling verifies that bad JSON lines are skipped, not fatal.
func TestMalformedLineHandling(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a1b2c3d4-e5f6-7890-abcd-ef1234567891.jsonl")

	content := `{"type":"user","uuid":"u1","sessionId":"a1b2c3d4-e5f6-7890-abcd-ef1234567891","timestamp":"2026-04-04T10:00:00.000Z","isSidechain":false,"message":{"role":"user","content":"hello"}}
this is not valid json at all }{}{
{"type":"assistant","uuid":"a1","sessionId":"a1b2c3d4-e5f6-7890-abcd-ef1234567891","timestamp":"2026-04-04T10:00:01.000Z","isSidechain":false,"message":{"id":"msg_x01","role":"assistant","model":"claude-sonnet-4-6","stop_reason":"end_turn","content":[{"type":"text","text":"hello back"}],"usage":{"input_tokens":10,"output_tokens":5,"cache_read_input_tokens":0,"cache_creation_input_tokens":0}}}
`
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	log := &testLogger{}
	ps, err := ParseSession(path, log, nil)
	if err != nil {
		t.Fatalf("ParseSession should not fail on malformed lines: %v", err)
	}

	// Malformed line should have generated a warning
	if len(log.warnings) == 0 {
		t.Error("expected at least one warning for malformed line, got none")
	}

	// Valid messages should still be parsed
	assistantCount := 0
	for _, m := range ps.Messages {
		if m.Role == "assistant" {
			assistantCount++
		}
	}
	if assistantCount != 1 {
		t.Errorf("assistant messages = %d, want 1", assistantCount)
	}
}

// TestTimestampFormatHandling verifies both ISO 8601 and epoch millisecond timestamps.
func TestTimestampFormatHandling(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "c1b2c3d4-e5f6-7890-abcd-ef1234567890.jsonl")

	// One message with ISO 8601, one with epoch ms
	content := `{"type":"user","uuid":"u1","sessionId":"c1b2c3d4-e5f6-7890-abcd-ef1234567890","timestamp":"2026-04-04T10:00:00.000Z","isSidechain":false,"message":{"role":"user","content":"hello"}}
{"type":"assistant","uuid":"a1","sessionId":"c1b2c3d4-e5f6-7890-abcd-ef1234567890","timestamp":1743760861000,"isSidechain":false,"message":{"id":"msg_ts01","role":"assistant","model":"claude-sonnet-4-6","stop_reason":"end_turn","content":[{"type":"text","text":"hi"}],"usage":{"input_tokens":5,"output_tokens":3,"cache_read_input_tokens":0,"cache_creation_input_tokens":0}}}
`
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	ps, err := ParseSession(path, nil, nil)
	if err != nil {
		t.Fatalf("ParseSession: %v", err)
	}

	if ps.StartedAt.IsZero() {
		t.Error("StartedAt is zero")
	}
	if ps.EndedAt.IsZero() {
		t.Error("EndedAt is zero")
	}

	// The epoch timestamp 1743760861000 = 2025-04-04T10:01:01Z
	epochExpected := time.Unix(1743760861, 0).UTC()
	// ISO timestamp = 2026-04-04T10:00:00Z (later than epoch)
	isoExpected := time.Date(2026, 4, 4, 10, 0, 0, 0, time.UTC)

	// StartedAt should be the earlier of the two: epoch (2025)
	if !ps.StartedAt.Equal(epochExpected) {
		t.Errorf("StartedAt = %v, want %v", ps.StartedAt, epochExpected)
	}
	// EndedAt should be the later of the two: ISO (2026)
	if !ps.EndedAt.Equal(isoExpected) {
		t.Errorf("EndedAt = %v, want %v", ps.EndedAt, isoExpected)
	}
}

// TestToolCallMatching verifies that tool_use blocks are matched with tool_result.
func TestToolCallMatching(t *testing.T) {
	ps, err := ParseSession(testDataPath("simple.jsonl"), nil, nil)
	if err != nil {
		t.Fatalf("ParseSession: %v", err)
	}

	if len(ps.ToolCalls) != 1 {
		t.Fatalf("ToolCalls = %d, want 1", len(ps.ToolCalls))
	}

	tc := ps.ToolCalls[0]
	if tc.ToolName != "Read" {
		t.Errorf("ToolName = %q, want Read", tc.ToolName)
	}
	if tc.FilePath != "/home/user/dev/project/main.go" {
		t.Errorf("FilePath = %q", tc.FilePath)
	}
	// Result was matched (content was present, not an error)
	if tc.IsError {
		t.Error("IsError = true, want false")
	}
	// Result size should be positive (content was provided)
	if tc.ResultSize == 0 {
		t.Error("ResultSize = 0, want > 0")
	}
}

// TestCostEstimationCallback verifies the cost estimator is called and its result stored.
func TestCostEstimationCallback(t *testing.T) {
	called := false
	estimator := func(model string, inputTokens, outputTokens, cacheReadTokens, cacheCreationTokens int) float64 {
		called = true
		if model != "claude-sonnet-4-6" {
			t.Errorf("estimator called with model=%q, want claude-sonnet-4-6", model)
		}
		return 0.042
	}

	ps, err := ParseSession(testDataPath("simple.jsonl"), nil, estimator)
	if err != nil {
		t.Fatalf("ParseSession: %v", err)
	}

	if !called {
		t.Error("cost estimator was not called")
	}
	if ps.TokenUsage.EstimatedCost != 0.042 {
		t.Errorf("EstimatedCost = %v, want 0.042", ps.TokenUsage.EstimatedCost)
	}
}

// TestNilCostEstimator verifies that a nil cost estimator leaves EstimatedCost as zero.
func TestNilCostEstimator(t *testing.T) {
	ps, err := ParseSession(testDataPath("simple.jsonl"), nil, nil)
	if err != nil {
		t.Fatalf("ParseSession: %v", err)
	}
	if ps.TokenUsage.EstimatedCost != 0 {
		t.Errorf("EstimatedCost = %v, want 0", ps.TokenUsage.EstimatedCost)
	}
}

// TestStreamingDeduplication verifies that duplicate messages (same message.id) are counted once.
func TestStreamingDeduplication(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "d1b2c3d4-e5f6-7890-abcd-ef1234567890.jsonl")

	// Same message ID appearing twice (streaming duplicate) — second one with stop_reason
	content := `{"type":"assistant","uuid":"a1","sessionId":"d1b2c3d4-e5f6-7890-abcd-ef1234567890","timestamp":"2026-04-04T10:00:00.000Z","isSidechain":false,"message":{"id":"msg_dup01","role":"assistant","model":"claude-sonnet-4-6","stop_reason":"end_turn","content":[{"type":"text","text":"hello"}],"usage":{"input_tokens":10,"output_tokens":5,"cache_read_input_tokens":0,"cache_creation_input_tokens":0}}}
{"type":"assistant","uuid":"a2","sessionId":"d1b2c3d4-e5f6-7890-abcd-ef1234567890","timestamp":"2026-04-04T10:00:01.000Z","isSidechain":false,"message":{"id":"msg_dup01","role":"assistant","model":"claude-sonnet-4-6","stop_reason":"end_turn","content":[{"type":"text","text":"hello again"}],"usage":{"input_tokens":10,"output_tokens":5,"cache_read_input_tokens":0,"cache_creation_input_tokens":0}}}
`
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	ps, err := ParseSession(path, nil, nil)
	if err != nil {
		t.Fatalf("ParseSession: %v", err)
	}

	count := 0
	for _, m := range ps.Messages {
		if m.Role == "assistant" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("assistant messages after dedup = %d, want 1", count)
	}

	// Token usage should only be counted once
	if ps.TokenUsage.InputTokens != 10 {
		t.Errorf("InputTokens = %d, want 10 (no double counting)", ps.TokenUsage.InputTokens)
	}
}

// TestTokenUsageMethods verifies RateLimit and Total helper methods.
func TestTokenUsageMethods(t *testing.T) {
	u := TokenUsage{
		InputTokens:         100,
		OutputTokens:        50,
		CacheReadTokens:     200,
		CacheCreationTokens: 30,
	}

	if got := u.RateLimit(); got != 180 {
		t.Errorf("RateLimit() = %d, want 180", got)
	}
	if got := u.Total(); got != 380 {
		t.Errorf("Total() = %d, want 380", got)
	}
}

// TestMultipleToolTypes verifies extraction for Bash, Edit, Glob, Grep tools.
func TestMultipleToolTypes(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "e1b2c3d4-e5f6-7890-abcd-ef1234567890.jsonl")

	content := `{"type":"assistant","uuid":"a1","sessionId":"e1b2c3d4-e5f6-7890-abcd-ef1234567890","timestamp":"2026-04-04T10:00:00.000Z","isSidechain":false,"message":{"id":"msg_tools01","role":"assistant","model":"claude-sonnet-4-6","stop_reason":"tool_use","content":[{"type":"tool_use","id":"toolu_bash","name":"Bash","input":{"command":"ls -la"}},{"type":"tool_use","id":"toolu_edit","name":"Edit","input":{"file_path":"/tmp/foo.go","old_string":"x","new_string":"y"}},{"type":"tool_use","id":"toolu_glob","name":"Glob","input":{"pattern":"*.go","path":"/home/user"}},{"type":"tool_use","id":"toolu_grep","name":"Grep","input":{"pattern":"func main","path":"/home/user/dev"}}],"usage":{"input_tokens":50,"output_tokens":80,"cache_read_input_tokens":0,"cache_creation_input_tokens":0}}}
{"type":"user","uuid":"u1","sessionId":"e1b2c3d4-e5f6-7890-abcd-ef1234567890","timestamp":"2026-04-04T10:00:01.000Z","isSidechain":false,"message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"toolu_bash","content":"file1.go\nfile2.go","is_error":false},{"type":"tool_result","tool_use_id":"toolu_edit","content":"edited","is_error":false},{"type":"tool_result","tool_use_id":"toolu_glob","content":"match1.go","is_error":false},{"type":"tool_result","tool_use_id":"toolu_grep","content":"main.go:1:func main()","is_error":false}]}}
`
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	ps, err := ParseSession(path, nil, nil)
	if err != nil {
		t.Fatalf("ParseSession: %v", err)
	}

	if len(ps.ToolCalls) != 4 {
		t.Fatalf("ToolCalls = %d, want 4", len(ps.ToolCalls))
	}

	byName := make(map[string]ToolCall)
	for _, tc := range ps.ToolCalls {
		byName[tc.ToolName] = tc
	}

	if tc, ok := byName["Bash"]; !ok {
		t.Error("no Bash tool call found")
	} else if tc.Command != "ls -la" {
		t.Errorf("Bash.Command = %q, want %q", tc.Command, "ls -la")
	}

	if tc, ok := byName["Edit"]; !ok {
		t.Error("no Edit tool call found")
	} else if tc.FilePath != "/tmp/foo.go" {
		t.Errorf("Edit.FilePath = %q, want /tmp/foo.go", tc.FilePath)
	}

	if tc, ok := byName["Glob"]; !ok {
		t.Error("no Glob tool call found")
	} else {
		if tc.Pattern != "*.go" {
			t.Errorf("Glob.Pattern = %q, want *.go", tc.Pattern)
		}
		if tc.FilePath != "/home/user" {
			t.Errorf("Glob.FilePath = %q, want /home/user", tc.FilePath)
		}
	}

	if tc, ok := byName["Grep"]; !ok {
		t.Error("no Grep tool call found")
	} else {
		if tc.Pattern != "func main" {
			t.Errorf("Grep.Pattern = %q, want 'func main'", tc.Pattern)
		}
		if tc.FilePath != "/home/user/dev" {
			t.Errorf("Grep.FilePath = %q, want /home/user/dev", tc.FilePath)
		}
	}
}

// TestParseSessionSummary verifies the fast summary path.
func TestParseSessionSummary(t *testing.T) {
	s, err := ParseSessionSummary(testDataPath("simple.jsonl"))
	if err != nil {
		t.Fatalf("ParseSessionSummary: %v", err)
	}

	if s.ID != "simple" {
		t.Errorf("ID = %q, want %q", s.ID, "simple")
	}
	if s.FileSize == 0 {
		t.Error("FileSize = 0")
	}
	if s.LastModified.IsZero() {
		t.Error("LastModified is zero")
	}
}

// TestIsValidUUID verifies UUID validation.
func TestIsValidUUID(t *testing.T) {
	valid := []string{
		"a1b2c3d4-e5f6-7890-abcd-ef1234567890",
		"00000000-0000-0000-0000-000000000000",
		"ffffffff-ffff-ffff-ffff-ffffffffffff",
	}
	invalid := []string{
		"not-a-uuid",
		"a1b2c3d4-e5f6-7890-abcd-ef123456789",  // too short
		"a1b2c3d4-e5f6-7890-abcd-ef12345678901", // too long
		"a1b2c3d4e5f678901234ef1234567890",       // no dashes
		"simple",
	}

	for _, s := range valid {
		if !isValidUUID(s) {
			t.Errorf("isValidUUID(%q) = false, want true", s)
		}
	}
	for _, s := range invalid {
		if isValidUUID(s) {
			t.Errorf("isValidUUID(%q) = true, want false", s)
		}
	}
}

// TestDefaultProblemConfig verifies the default thresholds.
func TestDefaultProblemConfig(t *testing.T) {
	cfg := DefaultProblemConfig()
	if cfg.RepeatedReadThreshold != 3 {
		t.Errorf("RepeatedReadThreshold = %d, want 3", cfg.RepeatedReadThreshold)
	}
	if cfg.CompactionLoopWindowMin != 10 {
		t.Errorf("CompactionLoopWindowMin = %d, want 10", cfg.CompactionLoopWindowMin)
	}
	if cfg.ExcessiveSubagentCount != 10 {
		t.Errorf("ExcessiveSubagentCount = %d, want 10", cfg.ExcessiveSubagentCount)
	}
	if cfg.ContextFillOutputRatio != 0.05 {
		t.Errorf("ContextFillOutputRatio = %v, want 0.05", cfg.ContextFillOutputRatio)
	}
}

// TestParseSessionFileNotFound verifies that ParseSession returns an error for
// a missing file (not a panic or empty session).
func TestParseSessionFileNotFound(t *testing.T) {
	_, err := ParseSession("/nonexistent/path/no-such-session.jsonl", nil, nil)
	if err == nil {
		t.Error("expected error for missing file, got nil")
	}
}

// TestParseSessionWithCostWrapper verifies that ParseSessionWithCost delegates
// to ParseSession with the provided estimator.
func TestParseSessionWithCostWrapper(t *testing.T) {
	var called bool
	estimator := func(model string, in, out, cacheRead, cacheCreate int) float64 {
		called = true
		return 1.234
	}
	ps, err := ParseSessionWithCost(testDataPath("simple.jsonl"), estimator)
	if err != nil {
		t.Fatalf("ParseSessionWithCost: %v", err)
	}
	if !called {
		t.Error("cost estimator was not called via ParseSessionWithCost")
	}
	if ps.TokenUsage.EstimatedCost != 1.234 {
		t.Errorf("EstimatedCost = %v, want 1.234", ps.TokenUsage.EstimatedCost)
	}
}

// TestStreamingPartialChunksSkipped verifies that assistant messages with no
// stop_reason (streaming partial chunks) are not added to Messages.
func TestStreamingPartialChunksSkipped(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "f1b2c3d4-e5f6-7890-abcd-ef1234567890.jsonl")

	// First line: no stop_reason (streaming partial) — should be skipped.
	// Second line: same message ID with stop_reason set — should be kept.
	content := `{"type":"assistant","uuid":"a1","sessionId":"f1b2c3d4-e5f6-7890-abcd-ef1234567890","timestamp":"2026-04-04T10:00:00.000Z","isSidechain":false,"message":{"id":"msg_stream01","role":"assistant","model":"claude-sonnet-4-6","stop_reason":null,"content":[{"type":"text","text":"partial..."}],"usage":{"input_tokens":5,"output_tokens":2,"cache_read_input_tokens":0,"cache_creation_input_tokens":0}}}
{"type":"assistant","uuid":"a2","sessionId":"f1b2c3d4-e5f6-7890-abcd-ef1234567890","timestamp":"2026-04-04T10:00:01.000Z","isSidechain":false,"message":{"id":"msg_stream01","role":"assistant","model":"claude-sonnet-4-6","stop_reason":"end_turn","content":[{"type":"text","text":"partial...final"}],"usage":{"input_tokens":5,"output_tokens":3,"cache_read_input_tokens":0,"cache_creation_input_tokens":0}}}
`
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	ps, err := ParseSession(path, nil, nil)
	if err != nil {
		t.Fatalf("ParseSession: %v", err)
	}

	assistantCount := 0
	for _, m := range ps.Messages {
		if m.Role == "assistant" {
			assistantCount++
		}
	}
	if assistantCount != 1 {
		t.Errorf("assistant messages = %d, want 1 (partial chunk must be skipped)", assistantCount)
	}
}

// TestFileReadCountsAllFileTools verifies that FileReadCounts accumulates paths
// from all file-touching tools (Read, Edit, Write, Glob, Grep), not only Read.
func TestFileReadCountsAllFileTools(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "g1b2c3d4-e5f6-7890-abcd-ef1234567890.jsonl")

	// One assistant message with Read + Edit + Write on the same file.
	content := `{"type":"assistant","uuid":"a1","sessionId":"g1b2c3d4-e5f6-7890-abcd-ef1234567890","timestamp":"2026-04-04T10:00:00.000Z","isSidechain":false,"message":{"id":"msg_fc01","role":"assistant","model":"claude-sonnet-4-6","stop_reason":"tool_use","content":[{"type":"tool_use","id":"tu_r","name":"Read","input":{"file_path":"/home/user/foo.go"}},{"type":"tool_use","id":"tu_e","name":"Edit","input":{"file_path":"/home/user/bar.go","old_string":"x","new_string":"y"}},{"type":"tool_use","id":"tu_w","name":"Write","input":{"file_path":"/home/user/baz.go"}}],"usage":{"input_tokens":10,"output_tokens":5,"cache_read_input_tokens":0,"cache_creation_input_tokens":0}}}
{"type":"user","uuid":"u1","sessionId":"g1b2c3d4-e5f6-7890-abcd-ef1234567890","timestamp":"2026-04-04T10:00:01.000Z","isSidechain":false,"message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"tu_r","content":"content","is_error":false},{"type":"tool_result","tool_use_id":"tu_e","content":"ok","is_error":false},{"type":"tool_result","tool_use_id":"tu_w","content":"ok","is_error":false}]}}
`
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	ps, err := ParseSession(path, nil, nil)
	if err != nil {
		t.Fatalf("ParseSession: %v", err)
	}

	// All three file paths should be in FileReadCounts.
	for _, fp := range []string{"/home/user/foo.go", "/home/user/bar.go", "/home/user/baz.go"} {
		if ps.FileReadCounts[fp] == 0 {
			t.Errorf("FileReadCounts[%q] = 0, want > 0", fp)
		}
	}
}

// TestParseTimestamp verifies both timestamp formats.
func TestParseTimestamp(t *testing.T) {
	// ISO 8601
	isoRaw := []byte(`"2026-04-04T10:00:00.000Z"`)
	ts, err := parseTimestamp(isoRaw)
	if err != nil {
		t.Fatalf("parseTimestamp ISO: %v", err)
	}
	expected := time.Date(2026, 4, 4, 10, 0, 0, 0, time.UTC)
	if !ts.Equal(expected) {
		t.Errorf("ISO timestamp = %v, want %v", ts, expected)
	}

	// Epoch milliseconds
	epochRaw := []byte(`1743760861000`)
	ts2, err := parseTimestamp(epochRaw)
	if err != nil {
		t.Fatalf("parseTimestamp epoch: %v", err)
	}
	expected2 := time.Unix(1743760861, 0).UTC()
	if !ts2.Equal(expected2) {
		t.Errorf("epoch timestamp = %v, want %v", ts2, expected2)
	}
}
