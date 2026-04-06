package proxy

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTestReqJSON(t *testing.T, body map[string]any) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.req.json")

	bodyJSON, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshaling body: %v", err)
	}

	envelope := map[string]any{
		"method":  "POST",
		"url":     "/v1/messages",
		"headers": map[string]any{},
		"body":    json.RawMessage(bodyJSON),
	}
	data, err := json.Marshal(envelope)
	if err != nil {
		t.Fatalf("marshaling envelope: %v", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("writing test file: %v", err)
	}
	return path
}

func TestAnalyzeRequestBodyBasic(t *testing.T) {
	path := writeTestReqJSON(t, map[string]any{
		"model": "claude-opus-4-6",
		"system": []map[string]any{
			{"type": "text", "text": "You are helpful."},
		},
		"messages": []map[string]any{
			{"role": "user", "content": "Hello"},
			{"role": "assistant", "content": "Hi there!"},
		},
	})

	a, err := AnalyzeRequestBody(path)
	if err != nil {
		t.Fatalf("AnalyzeRequestBody: %v", err)
	}

	if a.Model != "claude-opus-4-6" {
		t.Errorf("Model = %q, want %q", a.Model, "claude-opus-4-6")
	}
	if a.System.BlockCount != 1 {
		t.Errorf("System.BlockCount = %d, want 1", a.System.BlockCount)
	}
	if a.Messages.Total != 2 {
		t.Errorf("Messages.Total = %d, want 2", a.Messages.Total)
	}
	if a.Messages.ConversationTurns != 1 {
		t.Errorf("Messages.ConversationTurns = %d, want 1", a.Messages.ConversationTurns)
	}
}

func TestAnalyzeRequestBodyToolUses(t *testing.T) {
	path := writeTestReqJSON(t, map[string]any{
		"model": "claude-opus-4-6",
		"messages": []map[string]any{
			{"role": "user", "content": "hello"},
			{
				"role": "assistant",
				"content": []map[string]any{
					{"type": "tool_use", "id": "tu_1", "name": "Bash", "input": map[string]any{"command": "ls"}},
					{"type": "tool_use", "id": "tu_2", "name": "Read", "input": map[string]any{"file_path": "/tmp/test.txt"}},
					{"type": "tool_use", "id": "tu_3", "name": "Bash", "input": map[string]any{"command": "pwd"}},
				},
			},
			{
				"role": "user",
				"content": []map[string]any{
					{"type": "tool_result", "tool_use_id": "tu_1", "content": "file1.go file2.go"},
					{"type": "tool_result", "tool_use_id": "tu_2", "content": "file contents here"},
					{"type": "tool_result", "tool_use_id": "tu_3", "content": "/home/user"},
				},
			},
		},
		"tools": []map[string]any{
			{"name": "Bash", "description": "Run bash"},
			{"name": "Read", "description": "Read file"},
		},
	})

	a, err := AnalyzeRequestBody(path)
	if err != nil {
		t.Fatalf("AnalyzeRequestBody: %v", err)
	}

	if a.ToolUses.Count != 3 {
		t.Errorf("ToolUses.Count = %d, want 3", a.ToolUses.Count)
	}
	if a.ToolUses.ByTool["Bash"] != 2 {
		t.Errorf("ToolUses.ByTool[Bash] = %d, want 2", a.ToolUses.ByTool["Bash"])
	}
	if a.ToolUses.ByTool["Read"] != 1 {
		t.Errorf("ToolUses.ByTool[Read] = %d, want 1", a.ToolUses.ByTool["Read"])
	}
	if a.ToolResults.Count != 3 {
		t.Errorf("ToolResults.Count = %d, want 3", a.ToolResults.Count)
	}
	if a.Tools.Count != 2 {
		t.Errorf("Tools.Count = %d, want 2", a.Tools.Count)
	}
}

func TestAnalyzeRequestBodyFileReads(t *testing.T) {
	path := writeTestReqJSON(t, map[string]any{
		"model": "claude-opus-4-6",
		"messages": []map[string]any{
			{"role": "user", "content": "read these files"},
			{
				"role": "assistant",
				"content": []map[string]any{
					{"type": "tool_use", "id": "r1", "name": "Read", "input": map[string]any{"file_path": "/home/user/main.go"}},
					{"type": "tool_use", "id": "r2", "name": "Read", "input": map[string]any{"file_path": "/home/user/go.mod"}},
				},
			},
		},
	})

	a, err := AnalyzeRequestBody(path)
	if err != nil {
		t.Fatalf("AnalyzeRequestBody: %v", err)
	}

	if len(a.FileReads) != 2 {
		t.Fatalf("FileReads = %d, want 2", len(a.FileReads))
	}
	if a.FileReads[0] != "/home/user/main.go" {
		t.Errorf("FileReads[0] = %q, want /home/user/main.go", a.FileReads[0])
	}
	if a.FileReads[1] != "/home/user/go.mod" {
		t.Errorf("FileReads[1] = %q, want /home/user/go.mod", a.FileReads[1])
	}
}

func TestAnalyzeRequestBodySystemString(t *testing.T) {
	path := writeTestReqJSON(t, map[string]any{
		"model":    "claude-opus-4-6",
		"system":   "You are a helpful assistant.",
		"messages": []map[string]any{},
	})

	a, err := AnalyzeRequestBody(path)
	if err != nil {
		t.Fatalf("AnalyzeRequestBody: %v", err)
	}

	if a.System.BlockCount != 1 {
		t.Errorf("System.BlockCount = %d, want 1", a.System.BlockCount)
	}
	if a.System.TotalSizeBytes != 28 { // len("You are a helpful assistant.")
		t.Errorf("System.TotalSizeBytes = %d, want 28", a.System.TotalSizeBytes)
	}
	if a.System.Types["text"] != 1 {
		t.Errorf("System.Types[text] = %d, want 1", a.System.Types["text"])
	}
}

func TestAnalyzeRequestBodyNoTools(t *testing.T) {
	path := writeTestReqJSON(t, map[string]any{
		"model": "claude-opus-4-6",
		"messages": []map[string]any{
			{"role": "user", "content": "Hello"},
		},
	})

	a, err := AnalyzeRequestBody(path)
	if err != nil {
		t.Fatalf("AnalyzeRequestBody: %v", err)
	}

	if a.Tools.Count != 0 {
		t.Errorf("Tools.Count = %d, want 0", a.Tools.Count)
	}
	if a.ToolUses.Count != 0 {
		t.Errorf("ToolUses.Count = %d, want 0", a.ToolUses.Count)
	}
	if a.ToolResults.Count != 0 {
		t.Errorf("ToolResults.Count = %d, want 0", a.ToolResults.Count)
	}
	if len(a.FileReads) != 0 {
		t.Errorf("FileReads = %d, want 0", len(a.FileReads))
	}
}

func TestAnalyzeRequestBodyComposition(t *testing.T) {
	path := writeTestReqJSON(t, map[string]any{
		"model": "claude-opus-4-6",
		"system": []map[string]any{
			{"type": "text", "text": "system prompt"},
		},
		"messages": []map[string]any{
			{"role": "user", "content": "hello"},
			{"role": "assistant", "content": "world"},
		},
		"tools": []map[string]any{
			{"name": "Bash", "description": "run commands"},
		},
	})

	a, err := AnalyzeRequestBody(path)
	if err != nil {
		t.Fatalf("AnalyzeRequestBody: %v", err)
	}

	total := a.Composition.SystemPercent + a.Composition.ConversationPercent +
		a.Composition.ToolResultPercent + a.Composition.ToolDefPercent
	if total < 99.0 || total > 101.0 {
		t.Errorf("Composition percentages sum to %.1f%%, expected ~100%%", total)
	}
	if a.Composition.SystemPercent <= 0 {
		t.Error("SystemPercent should be > 0")
	}
	if a.Composition.ConversationPercent <= 0 {
		t.Error("ConversationPercent should be > 0")
	}
}

// ---------------------------------------------------------------------------
// RequestBreakdown tests (SPEC-03)
// ---------------------------------------------------------------------------

func TestClassifySystemBlock(t *testing.T) {
	cases := []struct {
		name     string
		block    rawContentBlock
		index    int
		wantLbl  string
		wantCat  string
	}{
		{
			name:    "index 0 is always base",
			block:   rawContentBlock{Type: "text", Text: "some system instructions"},
			index:   0,
			wantLbl: "System Prompt (base)",
			wantCat: "system",
		},
		{
			name:    "CLAUDE.md project — Contents of path",
			block:   rawContentBlock{Type: "text", Text: "Contents of /home/user/dev/myproject/CLAUDE.md:\nsome content"},
			index:   1,
			wantLbl: "CLAUDE.md (project)",
			wantCat: "system",
		},
		{
			name:    "CLAUDE.md user — ~/.claude/ path",
			block:   rawContentBlock{Type: "text", Text: "Contents of ~/.claude/CLAUDE.md:\nsome content"},
			index:   1,
			wantLbl: "CLAUDE.md (user)",
			wantCat: "system",
		},
		{
			name:    "CLAUDE.md user — /.claude/CLAUDE.md path",
			block:   rawContentBlock{Type: "text", Text: "Contents of /home/user/.claude/CLAUDE.md:\nsome content"},
			index:   2,
			wantLbl: "CLAUDE.md (user)",
			wantCat: "system",
		},
		{
			name:    "Auto-memory — frontmatter with # Memory",
			block:   rawContentBlock{Type: "text", Text: "---\n# Memory\nsome content\n---"},
			index:   2,
			wantLbl: "Auto-memory",
			wantCat: "memory",
		},
		{
			name:    "Auto-memory — MEMORY.md reference",
			block:   rawContentBlock{Type: "text", Text: "---\nMEMORY.md is available\n"},
			index:   3,
			wantLbl: "Auto-memory",
			wantCat: "memory",
		},
		{
			name:    "Auto-memory — [Use ] bullet format",
			block:   rawContentBlock{Type: "text", Text: "some preamble\n[Use this for guidance]\n"},
			index:   2,
			wantLbl: "Auto-memory",
			wantCat: "memory",
		},
		{
			name:    "Rules — ## Rules heading",
			block:   rawContentBlock{Type: "text", Text: "## Rules\n- always use Go"},
			index:   1,
			wantLbl: "Rules",
			wantCat: "system",
		},
		{
			name:    "Rules — ## Constraints heading",
			block:   rawContentBlock{Type: "text", Text: "## Constraints\n- never modify"},
			index:   1,
			wantLbl: "Rules",
			wantCat: "system",
		},
		{
			name:    "Rules — .claude/rules/ path",
			block:   rawContentBlock{Type: "text", Text: "Contents of .claude/rules/myfile.md"},
			index:   1,
			wantLbl: "Rules",
			wantCat: "system",
		},
		{
			name:    "Hook context — additionalContext keyword",
			block:   rawContentBlock{Type: "text", Text: "additionalContext: something"},
			index:   1,
			wantLbl: "Hook context",
			wantCat: "system",
		},
		{
			name:    "Hook context — hook output keyword",
			block:   rawContentBlock{Type: "text", Text: "hook output: some data"},
			index:   1,
			wantLbl: "Hook context",
			wantCat: "system",
		},
		{
			name:    "Output style — ## Output Style",
			block:   rawContentBlock{Type: "text", Text: "## Output Style\nuse markdown"},
			index:   1,
			wantLbl: "Output style",
			wantCat: "system",
		},
		{
			name:    "Output style — outputStyle keyword",
			block:   rawContentBlock{Type: "text", Text: "outputStyle: concise"},
			index:   1,
			wantLbl: "Output style",
			wantCat: "system",
		},
		{
			name:    "Unknown — short text",
			block:   rawContentBlock{Type: "text", Text: "You are an expert Go developer"},
			index:   1,
			wantLbl: `System (unknown): "You are an expert Go developer"`,
			wantCat: "system",
		},
		{
			name:    "Unknown — text truncated at 60 chars",
			block:   rawContentBlock{Type: "text", Text: "1234567890123456789012345678901234567890123456789012345678901234567890"},
			index:   2,
			wantLbl: `System (unknown): "123456789012345678901234567890123456789012345678901234567890"`,
			wantCat: "system",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			lbl, cat := classifySystemBlock(tc.block, tc.index)
			if lbl != tc.wantLbl {
				t.Errorf("label = %q, want %q", lbl, tc.wantLbl)
			}
			if cat != tc.wantCat {
				t.Errorf("category = %q, want %q", cat, tc.wantCat)
			}
		})
	}
}

func TestEstimateTokens(t *testing.T) {
	cases := []struct {
		chars int
		want  int
	}{
		{0, 0},
		{4, 1},
		{8, 2},
		{7, 2},  // 7%4=3 >= 2, rounds up
		{6, 2},  // 6%4=2 >= 2, rounds up
		{5, 1},  // 5%4=1 < 2, truncates
		{100, 25},
		{101, 25}, // 101%4=1 < 2
		{102, 26}, // 102%4=2 >= 2, rounds up
		{1000, 250},
	}
	for _, tc := range cases {
		got := estimateTokens(tc.chars)
		if got != tc.want {
			t.Errorf("estimateTokens(%d) = %d, want %d", tc.chars, got, tc.want)
		}
	}
}

func TestBuildToolUseIndex(t *testing.T) {
	messages := []rawMessage{
		{
			Role: "assistant",
			Content: mustMarshalRaw([]map[string]any{
				{"type": "tool_use", "id": "tu_1", "name": "Read", "input": map[string]any{"file_path": "/path/to/file.go"}},
				{"type": "tool_use", "id": "tu_2", "name": "Bash", "input": map[string]any{"command": "ls"}},
			}),
		},
		{
			Role: "user",
			Content: mustMarshalRaw([]map[string]any{
				{"type": "tool_result", "tool_use_id": "tu_1", "content": "file content"},
			}),
		},
		{
			Role: "assistant",
			Content: mustMarshalRaw([]map[string]any{
				{"type": "tool_use", "id": "tu_3", "name": "Grep", "input": map[string]any{"pattern": "foo"}},
			}),
		},
	}

	idx := buildToolUseIndex(messages)

	if len(idx) != 3 {
		t.Fatalf("index len = %d, want 3", len(idx))
	}
	if idx["tu_1"].Name != "Read" {
		t.Errorf("tu_1 name = %q, want Read", idx["tu_1"].Name)
	}
	if idx["tu_2"].Name != "Bash" {
		t.Errorf("tu_2 name = %q, want Bash", idx["tu_2"].Name)
	}
	if idx["tu_3"].Name != "Grep" {
		t.Errorf("tu_3 name = %q, want Grep", idx["tu_3"].Name)
	}

	// tool_result should not be indexed
	if _, ok := idx[""]; ok {
		t.Error("empty key should not be present")
	}
}

func mustMarshalRaw(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return json.RawMessage(b)
}

func TestExtractToolResultText(t *testing.T) {
	// String content
	strContent, _ := json.Marshal("file contents here")
	got := extractToolResultText(json.RawMessage(strContent))
	if got != "file contents here" {
		t.Errorf("string case: got %q, want %q", got, "file contents here")
	}

	// Array of text blocks
	arrContent, _ := json.Marshal([]map[string]any{
		{"type": "text", "text": "part one "},
		{"type": "text", "text": "part two"},
	})
	got = extractToolResultText(json.RawMessage(arrContent))
	if got != "part one part two" {
		t.Errorf("array case: got %q, want %q", got, "part one part two")
	}

	// Empty content
	got = extractToolResultText(nil)
	if got != "" {
		t.Errorf("nil case: got %q, want empty", got)
	}

	// Nested object fallback — returns raw JSON string
	objContent := json.RawMessage(`{"key":"value"}`)
	got = extractToolResultText(objContent)
	if got == "" {
		t.Error("fallback case: got empty, want non-empty raw bytes")
	}
}

func TestParseMessageBodyAllComponents(t *testing.T) {
	body := map[string]any{
		"model":      "claude-opus-4-6",
		"max_tokens": 16384,
		"system": []map[string]any{
			{"type": "text", "text": "You are a coding assistant."},
			{"type": "text", "text": "Contents of /home/user/dev/project/CLAUDE.md:\nproject instructions", "cache_control": map[string]any{"type": "ephemeral"}},
			{"type": "text", "text": "---\n# Memory\nsome memory data"},
		},
		"messages": []map[string]any{
			{"role": "user", "content": "please read my file"},
			{
				"role": "assistant",
				"content": []map[string]any{
					{"type": "text", "text": "Sure, reading it now."},
					{"type": "tool_use", "id": "tu_1", "name": "Read", "input": map[string]any{"file_path": "/home/user/main.go"}},
				},
			},
			{
				"role": "user",
				"content": []map[string]any{
					{"type": "tool_result", "tool_use_id": "tu_1", "content": "package main\nfunc main() {}"},
				},
			},
		},
		"tools": []map[string]any{
			{"name": "Read", "description": "Read a file"},
		},
		"thinking": map[string]any{
			"type":          "enabled",
			"budget_tokens": 5000,
		},
	}

	bodyJSON, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	bd, err := parseMessageBody(bodyJSON)
	if err != nil {
		t.Fatalf("parseMessageBody: %v", err)
	}

	// Model and max_tokens
	if bd.Model != "claude-opus-4-6" {
		t.Errorf("Model = %q, want claude-opus-4-6", bd.Model)
	}
	if bd.MaxTokens != 16384 {
		t.Errorf("MaxTokens = %d, want 16384", bd.MaxTokens)
	}

	// Thinking budget
	if bd.ThinkingBudgetTokens != 5000 {
		t.Errorf("ThinkingBudgetTokens = %d, want 5000", bd.ThinkingBudgetTokens)
	}

	// System components: 3 blocks
	if len(bd.SystemComponents) != 3 {
		t.Fatalf("SystemComponents len = %d, want 3", len(bd.SystemComponents))
	}
	if bd.SystemComponents[0].Label != "System Prompt (base)" {
		t.Errorf("SystemComponents[0].Label = %q, want System Prompt (base)", bd.SystemComponents[0].Label)
	}
	if bd.SystemComponents[1].Label != "CLAUDE.md (project)" {
		t.Errorf("SystemComponents[1].Label = %q, want CLAUDE.md (project)", bd.SystemComponents[1].Label)
	}
	if bd.SystemComponents[1].CacheStatus != "cache_candidate" {
		t.Errorf("SystemComponents[1].CacheStatus = %q, want cache_candidate", bd.SystemComponents[1].CacheStatus)
	}
	if bd.SystemComponents[2].Label != "Auto-memory" {
		t.Errorf("SystemComponents[2].Label = %q, want Auto-memory", bd.SystemComponents[2].Label)
	}

	// Tools component
	if bd.Tools.Category != "tool_defs" {
		t.Errorf("Tools.Category = %q, want tool_defs", bd.Tools.Category)
	}
	if !strings.Contains(bd.Tools.Label, "1 tools") {
		t.Errorf("Tools.Label = %q, want to contain '1 tools'", bd.Tools.Label)
	}

	// Messages: 3 messages
	if len(bd.Messages) != 3 {
		t.Fatalf("Messages len = %d, want 3", len(bd.Messages))
	}

	// Tool calls: 1 Read call
	if len(bd.ToolCalls) != 1 {
		t.Fatalf("ToolCalls len = %d, want 1", len(bd.ToolCalls))
	}
	if bd.ToolCalls[0].ToolName != "Read" {
		t.Errorf("ToolCalls[0].ToolName = %q, want Read", bd.ToolCalls[0].ToolName)
	}
	if bd.ToolCalls[0].ResultChars == 0 {
		t.Error("ToolCalls[0].ResultChars should be > 0 after matching")
	}

	// File reads: 1 Read result
	if len(bd.FileReads) != 1 {
		t.Fatalf("FileReads len = %d, want 1", len(bd.FileReads))
	}
	if bd.FileReads[0].FilePath != "/home/user/main.go" {
		t.Errorf("FileReads[0].FilePath = %q, want /home/user/main.go", bd.FileReads[0].FilePath)
	}
	if bd.FileReads[0].EstimatedTokens == 0 {
		t.Error("FileReads[0].EstimatedTokens should be > 0")
	}

	// TotalEstimatedTokens > 0
	if bd.TotalEstimatedTokens == 0 {
		t.Error("TotalEstimatedTokens should be > 0")
	}

	// LargestComponents: up to 5
	if len(bd.LargestComponents) == 0 {
		t.Error("LargestComponents should not be empty")
	}

	// Percentages should sum to ~100
	total := 0.0
	for _, c := range bd.SystemComponents {
		total += c.Percentage
	}
	total += bd.Tools.Percentage
	for _, m := range bd.Messages {
		for _, p := range m.Parts {
			total += p.Percentage
		}
	}
	if total < 99.0 || total > 101.0 {
		t.Errorf("percentages sum to %.2f, want ~100", total)
	}
}

func TestParseMessageBodyStringSystem(t *testing.T) {
	body := map[string]any{
		"model":    "claude-opus-4-6",
		"system":   "You are a helpful assistant.",
		"messages": []map[string]any{},
	}
	bodyJSON, _ := json.Marshal(body)

	bd, err := parseMessageBody(bodyJSON)
	if err != nil {
		t.Fatalf("parseMessageBody: %v", err)
	}

	if len(bd.SystemComponents) != 1 {
		t.Fatalf("SystemComponents len = %d, want 1", len(bd.SystemComponents))
	}
	if bd.SystemComponents[0].Label != "System Prompt (base)" {
		t.Errorf("label = %q, want System Prompt (base)", bd.SystemComponents[0].Label)
	}
	if bd.SystemComponents[0].CharCount != len("You are a helpful assistant.") {
		t.Errorf("CharCount = %d, want %d", bd.SystemComponents[0].CharCount, len("You are a helpful assistant."))
	}
}

func TestParseMessageBodyStringContent(t *testing.T) {
	body := map[string]any{
		"model": "claude-opus-4-6",
		"messages": []map[string]any{
			{"role": "user", "content": "hello world"},
		},
	}
	bodyJSON, _ := json.Marshal(body)

	bd, err := parseMessageBody(bodyJSON)
	if err != nil {
		t.Fatalf("parseMessageBody: %v", err)
	}

	if len(bd.Messages) != 1 {
		t.Fatalf("Messages len = %d, want 1", len(bd.Messages))
	}
	if len(bd.Messages[0].Parts) != 1 {
		t.Fatalf("Parts len = %d, want 1", len(bd.Messages[0].Parts))
	}
	if bd.Messages[0].Parts[0].Category != "user_text" {
		t.Errorf("Category = %q, want user_text", bd.Messages[0].Parts[0].Category)
	}
	if bd.Messages[0].Parts[0].CharCount != len("hello world") {
		t.Errorf("CharCount = %d, want %d", bd.Messages[0].Parts[0].CharCount, len("hello world"))
	}
}

func TestParseMessageBodyCorruptJSON(t *testing.T) {
	_, err := parseMessageBody([]byte(`{not valid json`))
	if err == nil {
		t.Error("expected error for corrupt JSON, got nil")
	}
}

func TestParseMessageBodyNoSystem(t *testing.T) {
	body := map[string]any{
		"model": "claude-opus-4-6",
		"messages": []map[string]any{
			{"role": "user", "content": "hi"},
		},
	}
	bodyJSON, _ := json.Marshal(body)

	bd, err := parseMessageBody(bodyJSON)
	if err != nil {
		t.Fatalf("parseMessageBody: %v", err)
	}
	if len(bd.SystemComponents) != 0 {
		t.Errorf("SystemComponents len = %d, want 0", len(bd.SystemComponents))
	}
}

func TestParseMessageBodyToolResultArrayContent(t *testing.T) {
	body := map[string]any{
		"model": "claude-opus-4-6",
		"messages": []map[string]any{
			{
				"role": "assistant",
				"content": []map[string]any{
					{"type": "tool_use", "id": "tu_99", "name": "Bash", "input": map[string]any{"command": "ls"}},
				},
			},
			{
				"role": "user",
				"content": []map[string]any{
					{
						"type":        "tool_result",
						"tool_use_id": "tu_99",
						"content": []map[string]any{
							{"type": "text", "text": "file1.go "},
							{"type": "text", "text": "file2.go"},
						},
					},
				},
			},
		},
	}
	bodyJSON, _ := json.Marshal(body)

	bd, err := parseMessageBody(bodyJSON)
	if err != nil {
		t.Fatalf("parseMessageBody: %v", err)
	}

	// Find tool_result part
	var trPart *ComponentBreakdown
	for _, m := range bd.Messages {
		for i, p := range m.Parts {
			if p.Category == "tool_result" {
				trPart = &m.Parts[i]
			}
		}
	}
	if trPart == nil {
		t.Fatal("no tool_result part found")
	}
	expectedText := "file1.go file2.go"
	expectedChars := len(expectedText)
	if trPart.CharCount != expectedChars {
		t.Errorf("tool_result CharCount = %d, want %d", trPart.CharCount, expectedChars)
	}
	if trPart.Label != "Bash output" {
		t.Errorf("tool_result Label = %q, want Bash output", trPart.Label)
	}
}

func TestParseMessageBodyUnmatchedToolResult(t *testing.T) {
	body := map[string]any{
		"model": "claude-opus-4-6",
		"messages": []map[string]any{
			{
				"role": "user",
				"content": []map[string]any{
					{"type": "tool_result", "tool_use_id": "nonexistent_id", "content": "some output"},
				},
			},
		},
	}
	bodyJSON, _ := json.Marshal(body)

	bd, err := parseMessageBody(bodyJSON)
	if err != nil {
		t.Fatalf("parseMessageBody: %v", err)
	}

	var trPart *ComponentBreakdown
	for _, m := range bd.Messages {
		for i, p := range m.Parts {
			if p.Category == "tool_result" {
				trPart = &m.Parts[i]
			}
		}
	}
	if trPart == nil {
		t.Fatal("no tool_result part found")
	}
	if trPart.Label != "Tool result (unmatched)" {
		t.Errorf("label = %q, want Tool result (unmatched)", trPart.Label)
	}
}

func TestParseMessageBodyVeryLargeToolResult(t *testing.T) {
	// Build a 2MB tool result string
	large := strings.Repeat("x", 2*1024*1024)
	body := map[string]any{
		"model": "claude-opus-4-6",
		"messages": []map[string]any{
			{
				"role": "assistant",
				"content": []map[string]any{
					{"type": "tool_use", "id": "big_1", "name": "Read", "input": map[string]any{"file_path": "/bigfile"}},
				},
			},
			{
				"role": "user",
				"content": []map[string]any{
					{"type": "tool_result", "tool_use_id": "big_1", "content": large},
				},
			},
		},
	}
	bodyJSON, _ := json.Marshal(body)

	bd, err := parseMessageBody(bodyJSON)
	if err != nil {
		t.Fatalf("parseMessageBody: %v", err)
	}

	expectedTokens := estimateTokens(len(large))
	if len(bd.FileReads) != 1 {
		t.Fatalf("FileReads len = %d, want 1", len(bd.FileReads))
	}
	if bd.FileReads[0].EstimatedTokens != expectedTokens {
		t.Errorf("EstimatedTokens = %d, want %d", bd.FileReads[0].EstimatedTokens, expectedTokens)
	}
}

func TestParseRequestFile(t *testing.T) {
	path := writeTestReqJSON(t, map[string]any{
		"model":      "claude-opus-4-6",
		"max_tokens": 8192,
		"system": []map[string]any{
			{"type": "text", "text": "base system"},
		},
		"messages": []map[string]any{
			{"role": "user", "content": "hello"},
		},
	})

	bd, err := ParseRequestFile(path, 1000, 600, 100)
	if err != nil {
		t.Fatalf("ParseRequestFile: %v", err)
	}

	if bd.Model != "claude-opus-4-6" {
		t.Errorf("Model = %q, want claude-opus-4-6", bd.Model)
	}
	if bd.MaxTokens != 8192 {
		t.Errorf("MaxTokens = %d, want 8192", bd.MaxTokens)
	}
	if bd.ActualInputTokens != 1000 {
		t.Errorf("ActualInputTokens = %d, want 1000", bd.ActualInputTokens)
	}
	if bd.CacheReadTokens != 600 {
		t.Errorf("CacheReadTokens = %d, want 600", bd.CacheReadTokens)
	}
	if bd.CacheCreationTokens != 100 {
		t.Errorf("CacheCreationTokens = %d, want 100", bd.CacheCreationTokens)
	}
	if bd.UncachedTokens != 300 {
		t.Errorf("UncachedTokens = %d, want 300", bd.UncachedTokens)
	}
	wantCacheReadPct := 60.0
	if bd.CacheReadPct != wantCacheReadPct {
		t.Errorf("CacheReadPct = %.2f, want %.2f", bd.CacheReadPct, wantCacheReadPct)
	}
}

func TestParseRequestFileMissingFile(t *testing.T) {
	_, err := ParseRequestFile("/nonexistent/path/file.req.json", 0, 0, 0)
	if err == nil {
		t.Error("expected error for missing file, got nil")
	}
}

func TestGenerateWhySummary(t *testing.T) {
	// Build a breakdown dominated by tool results
	bd := &RequestBreakdown{
		RequestID: "42",
		Model:     "claude-opus-4-6",
		Messages: []MessageBreakdown{
			{
				Role:  "user",
				Index: 0,
				Parts: []ComponentBreakdown{
					{Label: "File: /big.go", Category: "tool_result", CharCount: 80000, EstimatedTokens: 20000, Percentage: 80.0},
				},
				EstimatedTokens: 20000,
			},
			{
				Role:  "assistant",
				Index: 1,
				Parts: []ComponentBreakdown{
					{Label: "Assistant response", Category: "assistant_text", CharCount: 400, EstimatedTokens: 100, Percentage: 0.4},
				},
				EstimatedTokens: 100,
			},
		},
		TotalEstimatedTokens: 25000,
		FileReads: []FileReadInfo{
			{FilePath: "/big.go", CharCount: 80000, EstimatedTokens: 20000},
		},
		ActualInputTokens:   30000,
		CacheReadTokens:     20000,
		CacheCreationTokens: 5000,
		CacheReadPct:        66.7,
	}

	summary := GenerateWhySummary(bd, 0.50)

	if !strings.Contains(summary, "Request 42") {
		t.Errorf("summary missing request ID: %q", summary)
	}
	if !strings.Contains(summary, "Tool results dominated") {
		t.Errorf("summary should mention tool results dominant: %q", summary)
	}
	if !strings.Contains(summary, "large file reads") {
		t.Errorf("summary should mention large file reads: %q", summary)
	}
	if !strings.Contains(summary, "Cache hit rate was healthy") {
		t.Errorf("summary should mention healthy cache: %q", summary)
	}
}

func TestGenerateWhySummaryLowCache(t *testing.T) {
	bd := &RequestBreakdown{
		RequestID: "7",
		Messages: []MessageBreakdown{
			{
				Parts: []ComponentBreakdown{
					{Category: "user_text", EstimatedTokens: 1000},
				},
				EstimatedTokens: 1000,
			},
		},
		TotalEstimatedTokens: 1000,
		ActualInputTokens:    1000,
		CacheReadTokens:      300,
		CacheReadPct:         30.0,
	}

	summary := GenerateWhySummary(bd, 0.01)
	if !strings.Contains(summary, "Cache hit rate was low") {
		t.Errorf("summary should mention low cache: %q", summary)
	}
}

func TestGenerateWhySummaryNoCacheNoFiles(t *testing.T) {
	bd := &RequestBreakdown{
		RequestID: "1",
		Messages: []MessageBreakdown{
			{
				Parts: []ComponentBreakdown{
					{Category: "user_text", EstimatedTokens: 500},
				},
				EstimatedTokens: 500,
			},
		},
		TotalEstimatedTokens: 500,
	}

	summary := GenerateWhySummary(bd, 0.0)
	if !strings.Contains(summary, "Request 1") {
		t.Errorf("summary missing request ID: %q", summary)
	}
	// No cache section
	if strings.Contains(summary, "Cache hit rate") {
		t.Errorf("should not mention cache when CacheReadTokens=0: %q", summary)
	}
}

func TestGenerateWhySummaryThinkingBudget(t *testing.T) {
	bd := &RequestBreakdown{
		RequestID: "5",
		Messages: []MessageBreakdown{
			{
				Parts: []ComponentBreakdown{
					{Category: "user_text", EstimatedTokens: 100},
				},
				EstimatedTokens: 100,
			},
		},
		TotalEstimatedTokens: 100,
		ThinkingBudgetTokens: 8000,
	}

	summary := GenerateWhySummary(bd, 0.01)
	if !strings.Contains(summary, "Thinking budget was 8000") {
		t.Errorf("summary should mention thinking budget: %q", summary)
	}
}

func TestGenerateWhySummaryLongConversation(t *testing.T) {
	bd := &RequestBreakdown{
		RequestID: "9",
		Messages: []MessageBreakdown{
			{
				Parts: []ComponentBreakdown{
					{Category: "user_text", EstimatedTokens: 20000},
					{Category: "assistant_text", EstimatedTokens: 15000},
				},
				EstimatedTokens: 35000,
			},
		},
		TotalEstimatedTokens: 35000,
	}

	summary := GenerateWhySummary(bd, 0.10)
	if !strings.Contains(summary, "Conversation history is long") {
		t.Errorf("summary should mention long conversation: %q", summary)
	}
	if !strings.Contains(summary, "/compact") {
		t.Errorf("summary should suggest /compact: %q", summary)
	}
}

func TestGenerateWhySummaryNil(t *testing.T) {
	got := GenerateWhySummary(nil, 0)
	if got != "" {
		t.Errorf("nil breakdown should return empty string, got %q", got)
	}
}

func TestComputePercentages(t *testing.T) {
	components := []ComponentBreakdown{
		{EstimatedTokens: 100},
		{EstimatedTokens: 200},
		{EstimatedTokens: 700},
	}
	result := computePercentages(components, 1000)
	if result[0].Percentage != 10.0 {
		t.Errorf("pct[0] = %.2f, want 10.0", result[0].Percentage)
	}
	if result[1].Percentage != 20.0 {
		t.Errorf("pct[1] = %.2f, want 20.0", result[1].Percentage)
	}
	if result[2].Percentage != 70.0 {
		t.Errorf("pct[2] = %.2f, want 70.0", result[2].Percentage)
	}
}

func TestComputePercentagesZeroTotal(t *testing.T) {
	components := []ComponentBreakdown{
		{EstimatedTokens: 0},
	}
	result := computePercentages(components, 0)
	if result[0].Percentage != 0 {
		t.Errorf("zero total should result in 0%% percentage, got %.2f", result[0].Percentage)
	}
}

func TestTopN(t *testing.T) {
	components := []ComponentBreakdown{
		{Label: "A", EstimatedTokens: 10},
		{Label: "B", EstimatedTokens: 50},
		{Label: "C", EstimatedTokens: 30},
		{Label: "D", EstimatedTokens: 5},
		{Label: "E", EstimatedTokens: 80},
		{Label: "F", EstimatedTokens: 20},
	}

	top3 := topN(components, 3)
	if len(top3) != 3 {
		t.Fatalf("topN(3) len = %d, want 3", len(top3))
	}
	if top3[0].Label != "E" {
		t.Errorf("top3[0] = %q, want E", top3[0].Label)
	}
	if top3[1].Label != "B" {
		t.Errorf("top3[1] = %q, want B", top3[1].Label)
	}
	if top3[2].Label != "C" {
		t.Errorf("top3[2] = %q, want C", top3[2].Label)
	}

	// Request more than available
	topAll := topN(components, 100)
	if len(topAll) != len(components) {
		t.Errorf("topN(100) with 6 items len = %d, want 6", len(topAll))
	}
}

func TestLabelToolResultVariants(t *testing.T) {
	readBlock := rawContentBlock{
		Type:  "tool_use",
		ID:    "r1",
		Name:  "Read",
		Input: mustMarshalRaw(map[string]any{"file_path": "/path/to/file.go"}),
	}
	bashBlock := rawContentBlock{Type: "tool_use", ID: "b1", Name: "Bash", Input: mustMarshalRaw(map[string]any{"command": "ls"})}
	globBlock := rawContentBlock{Type: "tool_use", ID: "g1", Name: "Glob", Input: mustMarshalRaw(map[string]any{"pattern": "*.go"})}
	grepBlock := rawContentBlock{Type: "tool_use", ID: "gr1", Name: "Grep", Input: mustMarshalRaw(map[string]any{"pattern": "foo"})}
	editBlock := rawContentBlock{Type: "tool_use", ID: "e1", Name: "Edit", Input: mustMarshalRaw(map[string]any{"file_path": "/some/file.go"})}
	writeBlock := rawContentBlock{Type: "tool_use", ID: "w1", Name: "Write", Input: mustMarshalRaw(map[string]any{"file_path": "/out.txt"})}
	unknownBlock := rawContentBlock{Type: "tool_use", ID: "u1", Name: "CustomTool", Input: mustMarshalRaw(map[string]any{})}

	idx := map[string]rawContentBlock{
		"r1":  readBlock,
		"b1":  bashBlock,
		"g1":  globBlock,
		"gr1": grepBlock,
		"e1":  editBlock,
		"w1":  writeBlock,
		"u1":  unknownBlock,
	}

	cases := []struct{ id, want string }{
		{"r1", "File: /path/to/file.go"},
		{"b1", "Bash output"},
		{"g1", "Glob result"},
		{"gr1", "Grep result"},
		{"e1", "Edit result: /some/file.go"},
		{"w1", "Edit result: /out.txt"},
		{"u1", "Tool result: CustomTool"},
		{"missing", "Tool result (unmatched)"},
	}

	for _, tc := range cases {
		got := labelToolResult(tc.id, idx)
		if got != tc.want {
			t.Errorf("labelToolResult(%q) = %q, want %q", tc.id, got, tc.want)
		}
	}
}
