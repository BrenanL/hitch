package proxy

import (
	"encoding/json"
	"os"
	"path/filepath"
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
