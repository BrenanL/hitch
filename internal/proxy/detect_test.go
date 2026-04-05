package proxy

import (
	"encoding/json"
	"testing"
)

func TestDetectMicrocompactNone(t *testing.T) {
	body := []byte(`{"messages":[{"role":"user","content":"hello"}]}`)
	got := detectMicrocompact(body)
	if got != 0 {
		t.Errorf("detectMicrocompact = %d, want 0", got)
	}
}

func TestDetectMicrocompactMultiple(t *testing.T) {
	body := []byte(`{"messages":[
		{"role":"user","content":[
			{"type":"tool_result","content":"[Old tool result content cleared]"},
			{"type":"tool_result","content":"[Old tool result content cleared]"},
			{"type":"tool_result","content":"[Old tool result content cleared]"}
		]}
	]}`)
	got := detectMicrocompact(body)
	if got != 3 {
		t.Errorf("detectMicrocompact = %d, want 3", got)
	}
}

func TestDetectMicrocompactInvalidJSON(t *testing.T) {
	got := detectMicrocompact([]byte("not json at all"))
	if got != 0 {
		t.Errorf("detectMicrocompact on non-JSON = %d, want 0", got)
	}
}

func TestDetectBudgetTruncationNone(t *testing.T) {
	body := buildRequestBody(t, []toolResult{
		{Content: "this is a normal length tool result that should not be detected"},
	})
	truncated, _ := detectBudgetTruncation(body)
	if truncated != 0 {
		t.Errorf("truncated = %d, want 0", truncated)
	}
}

func TestDetectBudgetTruncationShortResults(t *testing.T) {
	body := buildRequestBody(t, []toolResult{
		{Content: "short"}, // 5 chars, within 1-41 range
		{Content: "x"},     // 1 char
	})
	truncated, _ := detectBudgetTruncation(body)
	if truncated != 2 {
		t.Errorf("truncated = %d, want 2", truncated)
	}
}

func TestDetectBudgetTruncationSubBlocks(t *testing.T) {
	// Content as array of sub-blocks
	body := []byte(`{"messages":[{"role":"user","content":[
		{"type":"tool_result","content":[{"type":"text","text":"hi"}]}
	]}]}`)
	truncated, total := detectBudgetTruncation(body)
	if truncated != 1 {
		t.Errorf("truncated = %d, want 1", truncated)
	}
	if total != 2 {
		t.Errorf("totalSize = %d, want 2", total)
	}
}

func TestDetectBudgetTruncationSkipsNonUser(t *testing.T) {
	// Assistant messages should not be scanned
	body := []byte(`{"messages":[{"role":"assistant","content":[
		{"type":"tool_result","content":"x"}
	]}]}`)
	truncated, _ := detectBudgetTruncation(body)
	if truncated != 0 {
		t.Errorf("truncated = %d, want 0 (assistant messages should be skipped)", truncated)
	}
}

func TestDetectBudgetTruncationInvalidJSON(t *testing.T) {
	truncated, total := detectBudgetTruncation([]byte("not json"))
	if truncated != 0 || total != 0 {
		t.Errorf("invalid JSON: truncated=%d, total=%d, want 0,0", truncated, total)
	}
}

func TestDetectBudgetTruncationTotalSize(t *testing.T) {
	body := buildRequestBody(t, []toolResult{
		{Content: "aaaaabbbbb"},     // 10 bytes
		{Content: "cccccdddddeeee"}, // 14 bytes
	})
	_, total := detectBudgetTruncation(body)
	if total != 24 {
		t.Errorf("totalSize = %d, want 24", total)
	}
}

// --- helpers ---

type toolResult struct {
	Content string
}

func buildRequestBody(t *testing.T, results []toolResult) []byte {
	t.Helper()
	var blocks []map[string]any
	for _, r := range results {
		blocks = append(blocks, map[string]any{
			"type":    "tool_result",
			"content": r.Content,
		})
	}
	req := map[string]any{
		"messages": []map[string]any{
			{"role": "user", "content": blocks},
		},
	}
	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshaling request body: %v", err)
	}
	return data
}
