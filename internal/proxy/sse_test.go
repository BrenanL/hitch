package proxy

import "testing"

func TestParseSSEDataMessageStart(t *testing.T) {
	data := `{"type":"message_start","message":{"id":"msg_test123","model":"claude-opus-4-6","usage":{"input_tokens":10,"cache_read_input_tokens":500,"cache_creation_input_tokens":100}}}`
	rec := &RequestLog{}
	parseSSEData(data, rec)

	if rec.RequestID != "msg_test123" {
		t.Errorf("RequestID = %q, want %q", rec.RequestID, "msg_test123")
	}
	if rec.Model != "claude-opus-4-6" {
		t.Errorf("Model = %q, want %q", rec.Model, "claude-opus-4-6")
	}
	if rec.InputTokens != 10 {
		t.Errorf("InputTokens = %d, want 10", rec.InputTokens)
	}
	if rec.CacheReadTokens != 500 {
		t.Errorf("CacheReadTokens = %d, want 500", rec.CacheReadTokens)
	}
	if rec.CacheCreationTokens != 100 {
		t.Errorf("CacheCreationTokens = %d, want 100", rec.CacheCreationTokens)
	}
}

func TestParseSSEDataMessageDelta(t *testing.T) {
	data := `{"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":42}}`
	rec := &RequestLog{}
	parseSSEData(data, rec)

	if rec.OutputTokens != 42 {
		t.Errorf("OutputTokens = %d, want 42", rec.OutputTokens)
	}
	if rec.StopReason != "end_turn" {
		t.Errorf("StopReason = %q, want %q", rec.StopReason, "end_turn")
	}
}

func TestParseSSEDataUnknownType(t *testing.T) {
	data := `{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hello!"}}`
	rec := &RequestLog{}
	parseSSEData(data, rec)

	// Should not change any fields
	if rec.RequestID != "" || rec.Model != "" || rec.InputTokens != 0 || rec.OutputTokens != 0 {
		t.Error("unknown event type should not modify RequestLog fields")
	}
}

func TestParseSSEDataInvalidJSON(t *testing.T) {
	rec := &RequestLog{}
	parseSSEData("not valid json {{{", rec)

	// Should not panic or set fields
	if rec.RequestID != "" {
		t.Error("invalid JSON should not set RequestID")
	}
}

func TestParseNonStreamingResponse(t *testing.T) {
	body := []byte(`{
		"id": "msg_test456",
		"model": "claude-sonnet-4-6",
		"stop_reason": "tool_use",
		"usage": {
			"input_tokens": 15,
			"output_tokens": 8,
			"cache_read_input_tokens": 1000,
			"cache_creation_input_tokens": 200
		}
	}`)
	rec := &RequestLog{}
	parseNonStreamingResponse(body, rec)

	if rec.RequestID != "msg_test456" {
		t.Errorf("RequestID = %q, want %q", rec.RequestID, "msg_test456")
	}
	if rec.Model != "claude-sonnet-4-6" {
		t.Errorf("Model = %q, want %q", rec.Model, "claude-sonnet-4-6")
	}
	if rec.StopReason != "tool_use" {
		t.Errorf("StopReason = %q, want %q", rec.StopReason, "tool_use")
	}
	if rec.InputTokens != 15 {
		t.Errorf("InputTokens = %d, want 15", rec.InputTokens)
	}
	if rec.OutputTokens != 8 {
		t.Errorf("OutputTokens = %d, want 8", rec.OutputTokens)
	}
	if rec.CacheReadTokens != 1000 {
		t.Errorf("CacheReadTokens = %d, want 1000", rec.CacheReadTokens)
	}
	if rec.CacheCreationTokens != 200 {
		t.Errorf("CacheCreationTokens = %d, want 200", rec.CacheCreationTokens)
	}
}

func TestParseNonStreamingResponseInvalidJSON(t *testing.T) {
	rec := &RequestLog{}
	parseNonStreamingResponse([]byte("not json"), rec)
	if rec.RequestID != "" {
		t.Error("invalid JSON should not set RequestID")
	}
}

func TestParseNonStreamingResponseEmptyUsage(t *testing.T) {
	body := []byte(`{"id":"msg_empty","model":"claude-opus-4-6","stop_reason":"end_turn","usage":{}}`)
	rec := &RequestLog{}
	parseNonStreamingResponse(body, rec)

	if rec.RequestID != "msg_empty" {
		t.Errorf("RequestID = %q, want %q", rec.RequestID, "msg_empty")
	}
	if rec.InputTokens != 0 || rec.OutputTokens != 0 {
		t.Errorf("empty usage should yield zero tokens, got in=%d out=%d", rec.InputTokens, rec.OutputTokens)
	}
}
