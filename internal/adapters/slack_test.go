package adapters

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSlackValidateConfig(t *testing.T) {
	_, err := NewSlackAdapter(map[string]string{})
	if err == nil {
		t.Error("expected error for missing webhook_url")
	}

	_, err = NewSlackAdapter(map[string]string{"webhook_url": "https://hooks.slack.com/services/T/B/xxx"})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestSlackSend(t *testing.T) {
	var payload map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &payload)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))
	defer server.Close()

	a, _ := NewSlackAdapter(map[string]string{"webhook_url": server.URL})

	result := a.Send(context.Background(), Message{
		Title: "Hitch Alert",
		Body:  "Tests passed!",
		Level: Info,
	})

	if !result.Success {
		t.Errorf("Send failed: %v", result.Error)
	}

	blocks, ok := payload["blocks"].([]any)
	if !ok {
		t.Fatal("expected blocks")
	}
	if len(blocks) < 2 {
		t.Fatalf("expected at least 2 blocks, got %d", len(blocks))
	}

	// First block should be header
	header := blocks[0].(map[string]any)
	if header["type"] != "header" {
		t.Errorf("first block type = %v, want header", header["type"])
	}

	// Second block should be section with body
	section := blocks[1].(map[string]any)
	if section["type"] != "section" {
		t.Errorf("second block type = %v, want section", section["type"])
	}

	// Check text fallback
	if payload["text"] != "Tests passed!" {
		t.Errorf("fallback text = %v", payload["text"])
	}
}

func TestSlackSendWithFields(t *testing.T) {
	var payload map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &payload)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	a, _ := NewSlackAdapter(map[string]string{"webhook_url": server.URL})

	result := a.Send(context.Background(), Message{
		Title: "Alert",
		Body:  "Something happened",
		Fields: map[string]string{
			"Event": "Stop",
		},
	})

	if !result.Success {
		t.Errorf("Send failed: %v", result.Error)
	}

	blocks := payload["blocks"].([]any)
	// Should have header, section, and context blocks
	if len(blocks) != 3 {
		t.Errorf("expected 3 blocks, got %d", len(blocks))
	}
}
