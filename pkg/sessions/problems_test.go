package sessions

import (
	"testing"
	"time"
)

var testCfg = DefaultProblemConfig()

// makeTime returns a time.Time at a fixed base plus the given offset minutes.
func makeTime(offsetMinutes int) time.Time {
	base := time.Date(2026, 4, 4, 12, 0, 0, 0, time.UTC)
	return base.Add(time.Duration(offsetMinutes) * time.Minute)
}

// --- Repeated reads ---

func TestDetectRepeatedReads_MainSession(t *testing.T) {
	s := &ParsedSession{
		ToolCalls: []ToolCall{
			{ToolName: "Read", FilePath: "foo.go"},
			{ToolName: "Read", FilePath: "foo.go"},
			{ToolName: "Read", FilePath: "foo.go"}, // 3rd read — triggers
		},
	}
	problems := DetectProblems(s, testCfg)
	if len(problems.RepeatedReads) != 1 {
		t.Fatalf("expected 1 repeated read problem, got %d", len(problems.RepeatedReads))
	}
	p := problems.RepeatedReads[0]
	if p.FilePath != "foo.go" {
		t.Errorf("FilePath = %q, want foo.go", p.FilePath)
	}
	if p.ReadCount != 3 {
		t.Errorf("ReadCount = %d, want 3", p.ReadCount)
	}
	if p.SubagentID != "" {
		t.Errorf("SubagentID = %q, want empty", p.SubagentID)
	}
}

func TestDetectRepeatedReads_BelowThreshold(t *testing.T) {
	s := &ParsedSession{
		ToolCalls: []ToolCall{
			{ToolName: "Read", FilePath: "foo.go"},
			{ToolName: "Read", FilePath: "foo.go"}, // only 2 — no trigger
		},
	}
	problems := DetectProblems(s, testCfg)
	if len(problems.RepeatedReads) != 0 {
		t.Errorf("expected 0 repeated read problems, got %d", len(problems.RepeatedReads))
	}
}

func TestDetectRepeatedReads_Subagent(t *testing.T) {
	s := &ParsedSession{
		Subagents: []SubagentInfo{
			{
				SessionID: "sub-1",
				ToolCalls: []ToolCall{
					{ToolName: "Read", FilePath: "bar.go"},
					{ToolName: "Read", FilePath: "bar.go"},
					{ToolName: "Read", FilePath: "bar.go"},
				},
			},
		},
	}
	problems := DetectProblems(s, testCfg)
	if len(problems.RepeatedReads) != 1 {
		t.Fatalf("expected 1 repeated read problem, got %d", len(problems.RepeatedReads))
	}
	p := problems.RepeatedReads[0]
	if p.SubagentID != "sub-1" {
		t.Errorf("SubagentID = %q, want sub-1", p.SubagentID)
	}
	if p.FilePath != "bar.go" {
		t.Errorf("FilePath = %q, want bar.go", p.FilePath)
	}
}

func TestDetectRepeatedReads_NonReadToolsIgnored(t *testing.T) {
	s := &ParsedSession{
		ToolCalls: []ToolCall{
			{ToolName: "Edit", FilePath: "foo.go"},
			{ToolName: "Edit", FilePath: "foo.go"},
			{ToolName: "Edit", FilePath: "foo.go"},
			{ToolName: "Bash", Command: "ls"},
		},
	}
	problems := DetectProblems(s, testCfg)
	if len(problems.RepeatedReads) != 0 {
		t.Errorf("expected 0 repeated read problems for non-Read tools, got %d", len(problems.RepeatedReads))
	}
}

// --- Compaction loops ---

func TestDetectCompactionLoop_Triggered(t *testing.T) {
	t0 := makeTime(0)
	compactionAt := makeTime(10)

	s := &ParsedSession{
		ToolCalls: []ToolCall{
			{ToolName: "Read", FilePath: "server.go", Timestamp: t0},
			{ToolName: "Read", FilePath: "parser.go", Timestamp: t0},
			// After compaction, within window
			{ToolName: "Read", FilePath: "server.go", Timestamp: makeTime(15)},
		},
		Compactions: []CompactionEvent{
			{Timestamp: compactionAt},
		},
	}
	problems := DetectProblems(s, testCfg)
	if len(problems.CompactionLoops) != 1 {
		t.Fatalf("expected 1 compaction loop problem, got %d", len(problems.CompactionLoops))
	}
	p := problems.CompactionLoops[0]
	if !p.CompactionAt.Equal(compactionAt) {
		t.Errorf("CompactionAt = %v, want %v", p.CompactionAt, compactionAt)
	}
	if len(p.RereadFiles) != 1 || p.RereadFiles[0] != "server.go" {
		t.Errorf("RereadFiles = %v, want [server.go]", p.RereadFiles)
	}
	if p.WindowMinutes != testCfg.CompactionLoopWindowMin {
		t.Errorf("WindowMinutes = %d, want %d", p.WindowMinutes, testCfg.CompactionLoopWindowMin)
	}
}

func TestDetectCompactionLoop_OutsideWindow(t *testing.T) {
	t0 := makeTime(0)
	compactionAt := makeTime(10)

	s := &ParsedSession{
		ToolCalls: []ToolCall{
			{ToolName: "Read", FilePath: "server.go", Timestamp: t0},
			// Re-read occurs 11 minutes after compaction — outside 10-min window
			{ToolName: "Read", FilePath: "server.go", Timestamp: makeTime(21)},
		},
		Compactions: []CompactionEvent{
			{Timestamp: compactionAt},
		},
	}
	problems := DetectProblems(s, testCfg)
	if len(problems.CompactionLoops) != 0 {
		t.Errorf("expected 0 compaction loop problems (outside window), got %d", len(problems.CompactionLoops))
	}
}

func TestDetectCompactionLoop_NoRereadOfPreviousFiles(t *testing.T) {
	compactionAt := makeTime(10)

	s := &ParsedSession{
		ToolCalls: []ToolCall{
			// Only new file read after compaction
			{ToolName: "Read", FilePath: "new.go", Timestamp: makeTime(15)},
		},
		Compactions: []CompactionEvent{
			{Timestamp: compactionAt},
		},
	}
	problems := DetectProblems(s, testCfg)
	if len(problems.CompactionLoops) != 0 {
		t.Errorf("expected 0 compaction loop problems (new file only), got %d", len(problems.CompactionLoops))
	}
}

func TestDetectCompactionLoop_NoCompactions(t *testing.T) {
	s := &ParsedSession{
		ToolCalls: []ToolCall{
			{ToolName: "Read", FilePath: "server.go", Timestamp: makeTime(0)},
			{ToolName: "Read", FilePath: "server.go", Timestamp: makeTime(5)},
		},
	}
	problems := DetectProblems(s, testCfg)
	if len(problems.CompactionLoops) != 0 {
		t.Errorf("expected 0 compaction loop problems (no compactions), got %d", len(problems.CompactionLoops))
	}
}

// --- Model mismatches ---

func TestDetectModelMismatch_BothOpus(t *testing.T) {
	s := &ParsedSession{
		Model: "claude-opus-4-6",
		Subagents: []SubagentInfo{
			{SessionID: "sub-1", Model: "claude-opus-4-6"},
		},
	}
	problems := DetectProblems(s, testCfg)
	if len(problems.ModelMismatches) != 1 {
		t.Fatalf("expected 1 model mismatch problem, got %d", len(problems.ModelMismatches))
	}
	p := problems.ModelMismatches[0]
	if p.SubagentID != "sub-1" {
		t.Errorf("SubagentID = %q, want sub-1", p.SubagentID)
	}
	if p.SubagentModel != "claude-opus-4-6" {
		t.Errorf("SubagentModel = %q, want claude-opus-4-6", p.SubagentModel)
	}
	if p.ParentModel != "claude-opus-4-6" {
		t.Errorf("ParentModel = %q, want claude-opus-4-6", p.ParentModel)
	}
}

func TestDetectModelMismatch_SubagentSonnet(t *testing.T) {
	s := &ParsedSession{
		Model: "claude-opus-4-6",
		Subagents: []SubagentInfo{
			{SessionID: "sub-1", Model: "claude-sonnet-4-6"},
		},
	}
	problems := DetectProblems(s, testCfg)
	if len(problems.ModelMismatches) != 0 {
		t.Errorf("expected 0 model mismatch problems (subagent is sonnet), got %d", len(problems.ModelMismatches))
	}
}

func TestDetectModelMismatch_ParentNotOpus(t *testing.T) {
	s := &ParsedSession{
		Model: "claude-sonnet-4-6",
		Subagents: []SubagentInfo{
			{SessionID: "sub-1", Model: "claude-opus-4-6"},
		},
	}
	problems := DetectProblems(s, testCfg)
	if len(problems.ModelMismatches) != 0 {
		t.Errorf("expected 0 model mismatch problems (parent is sonnet), got %d", len(problems.ModelMismatches))
	}
}

func TestDetectModelMismatch_CaseInsensitive(t *testing.T) {
	s := &ParsedSession{
		Model: "Claude-Opus-4-6",
		Subagents: []SubagentInfo{
			{SessionID: "sub-1", Model: "OPUS-4"},
		},
	}
	problems := DetectProblems(s, testCfg)
	if len(problems.ModelMismatches) != 1 {
		t.Errorf("expected 1 model mismatch (case-insensitive), got %d", len(problems.ModelMismatches))
	}
}

// --- Excessive subagents ---

func TestDetectExcessiveSubagents_AtThreshold(t *testing.T) {
	subagents := make([]SubagentInfo, 10) // exactly 10 — at threshold
	s := &ParsedSession{Subagents: subagents}
	problems := DetectProblems(s, testCfg)
	if problems.ExcessiveSubagents == nil {
		t.Fatal("expected ExcessiveSubagents problem at threshold (10)")
	}
	if problems.ExcessiveSubagents.SubagentCount != 10 {
		t.Errorf("SubagentCount = %d, want 10", problems.ExcessiveSubagents.SubagentCount)
	}
	if problems.ExcessiveSubagents.Threshold != 10 {
		t.Errorf("Threshold = %d, want 10", problems.ExcessiveSubagents.Threshold)
	}
}

func TestDetectExcessiveSubagents_AboveThreshold(t *testing.T) {
	subagents := make([]SubagentInfo, 14)
	s := &ParsedSession{Subagents: subagents}
	problems := DetectProblems(s, testCfg)
	if problems.ExcessiveSubagents == nil {
		t.Fatal("expected ExcessiveSubagents problem")
	}
	if problems.ExcessiveSubagents.SubagentCount != 14 {
		t.Errorf("SubagentCount = %d, want 14", problems.ExcessiveSubagents.SubagentCount)
	}
}

func TestDetectExcessiveSubagents_BelowThreshold(t *testing.T) {
	subagents := make([]SubagentInfo, 9) // 9 < 10
	s := &ParsedSession{Subagents: subagents}
	problems := DetectProblems(s, testCfg)
	if problems.ExcessiveSubagents != nil {
		t.Errorf("expected no ExcessiveSubagents problem (9 < 10), got one")
	}
}

func TestDetectExcessiveSubagents_NoSubagents(t *testing.T) {
	s := &ParsedSession{}
	problems := DetectProblems(s, testCfg)
	if problems.ExcessiveSubagents != nil {
		t.Errorf("expected no ExcessiveSubagents problem (no subagents)")
	}
}

// --- Context fill without progress ---

func TestDetectContextFillNoProgress_MainSession(t *testing.T) {
	s := &ParsedSession{
		TokenUsage: TokenUsage{
			InputTokens:  340_000,
			OutputTokens: 2_000, // ratio = 2000/342000 ≈ 0.0058 < 0.05
		},
	}
	problems := DetectProblems(s, testCfg)
	if len(problems.ContextFillNoProgress) != 1 {
		t.Fatalf("expected 1 context fill problem, got %d", len(problems.ContextFillNoProgress))
	}
	p := problems.ContextFillNoProgress[0]
	if p.SubagentID != "" {
		t.Errorf("SubagentID = %q, want empty (main session)", p.SubagentID)
	}
	if p.InputTokens != 340_000 {
		t.Errorf("InputTokens = %d, want 340000", p.InputTokens)
	}
	if p.OutputTokens != 2_000 {
		t.Errorf("OutputTokens = %d, want 2000", p.OutputTokens)
	}
	if p.OutputRatio >= 0.05 {
		t.Errorf("OutputRatio = %f, expected < 0.05", p.OutputRatio)
	}
}

func TestDetectContextFillNoProgress_BelowInputThreshold(t *testing.T) {
	s := &ParsedSession{
		TokenUsage: TokenUsage{
			InputTokens:  50_000, // not > 50_000
			OutputTokens: 100,
		},
	}
	problems := DetectProblems(s, testCfg)
	if len(problems.ContextFillNoProgress) != 0 {
		t.Errorf("expected 0 context fill problems (input <= 50000), got %d", len(problems.ContextFillNoProgress))
	}
}

func TestDetectContextFillNoProgress_HighOutputRatio(t *testing.T) {
	s := &ParsedSession{
		TokenUsage: TokenUsage{
			InputTokens:  100_000,
			OutputTokens: 20_000, // ratio = 0.167 > 0.05
		},
	}
	problems := DetectProblems(s, testCfg)
	if len(problems.ContextFillNoProgress) != 0 {
		t.Errorf("expected 0 context fill problems (high output ratio), got %d", len(problems.ContextFillNoProgress))
	}
}

func TestDetectContextFillNoProgress_Subagent(t *testing.T) {
	s := &ParsedSession{
		Subagents: []SubagentInfo{
			{
				SessionID: "sub-1",
				TokenUsage: TokenUsage{
					InputTokens:  200_000,
					OutputTokens: 500, // very low output ratio
				},
			},
		},
	}
	problems := DetectProblems(s, testCfg)
	if len(problems.ContextFillNoProgress) != 1 {
		t.Fatalf("expected 1 context fill problem (subagent), got %d", len(problems.ContextFillNoProgress))
	}
	if problems.ContextFillNoProgress[0].SubagentID != "sub-1" {
		t.Errorf("SubagentID = %q, want sub-1", problems.ContextFillNoProgress[0].SubagentID)
	}
}

// --- Clean session ---

func TestDetectProblems_CleanSession(t *testing.T) {
	s := &ParsedSession{
		Model: "claude-sonnet-4-6",
		ToolCalls: []ToolCall{
			{ToolName: "Read", FilePath: "main.go", Timestamp: makeTime(0)},
			{ToolName: "Edit", FilePath: "main.go", Timestamp: makeTime(1)},
		},
		TokenUsage: TokenUsage{
			InputTokens:  10_000,
			OutputTokens: 5_000,
		},
		Subagents: []SubagentInfo{
			{SessionID: "sub-1", Model: "claude-sonnet-4-6"},
		},
	}
	problems := DetectProblems(s, testCfg)
	if len(problems.RepeatedReads) != 0 {
		t.Errorf("RepeatedReads: expected 0, got %d", len(problems.RepeatedReads))
	}
	if len(problems.CompactionLoops) != 0 {
		t.Errorf("CompactionLoops: expected 0, got %d", len(problems.CompactionLoops))
	}
	if len(problems.ModelMismatches) != 0 {
		t.Errorf("ModelMismatches: expected 0, got %d", len(problems.ModelMismatches))
	}
	if problems.ExcessiveSubagents != nil {
		t.Errorf("ExcessiveSubagents: expected nil, got %+v", problems.ExcessiveSubagents)
	}
	if len(problems.ContextFillNoProgress) != 0 {
		t.Errorf("ContextFillNoProgress: expected 0, got %d", len(problems.ContextFillNoProgress))
	}
}

// --- Multiple problems simultaneously ---

func TestDetectProblems_MultipleProblems(t *testing.T) {
	t0 := makeTime(0)
	compactionAt := makeTime(10)

	s := &ParsedSession{
		Model: "claude-opus-4-6",
		ToolCalls: []ToolCall{
			// Repeated reads (3x)
			{ToolName: "Read", FilePath: "big.go", Timestamp: t0},
			{ToolName: "Read", FilePath: "big.go", Timestamp: makeTime(1)},
			{ToolName: "Read", FilePath: "big.go", Timestamp: makeTime(2)},
			// Compaction loop re-read within window
			{ToolName: "Read", FilePath: "server.go", Timestamp: t0},
			{ToolName: "Read", FilePath: "server.go", Timestamp: makeTime(15)},
		},
		Compactions: []CompactionEvent{
			{Timestamp: compactionAt},
		},
		TokenUsage: TokenUsage{
			InputTokens:  300_000,
			OutputTokens: 1_000, // context fill
		},
		Subagents: func() []SubagentInfo {
			sas := make([]SubagentInfo, 10)
			for i := range sas {
				sas[i] = SubagentInfo{
					SessionID: "sub",
					Model:     "claude-opus-4-6", // model mismatch
				}
			}
			return sas
		}(),
	}

	problems := DetectProblems(s, testCfg)

	if len(problems.RepeatedReads) == 0 {
		t.Error("expected RepeatedReads problem")
	}
	if len(problems.CompactionLoops) == 0 {
		t.Error("expected CompactionLoops problem")
	}
	if len(problems.ModelMismatches) == 0 {
		t.Error("expected ModelMismatches problem")
	}
	if problems.ExcessiveSubagents == nil {
		t.Error("expected ExcessiveSubagents problem")
	}
	if len(problems.ContextFillNoProgress) == 0 {
		t.Error("expected ContextFillNoProgress problem")
	}
}
