package adapters

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDiscordValidateConfig(t *testing.T) {
	_, err := NewDiscordAdapter(map[string]string{})
	if err == nil {
		t.Error("expected error for missing webhook_url")
	}

	_, err = NewDiscordAdapter(map[string]string{"webhook_url": "https://discord.com/api/webhooks/123/abc"})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestDiscordSend(t *testing.T) {
	var payload map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &payload)
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Content-Type = %q", r.Header.Get("Content-Type"))
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	a, _ := NewDiscordAdapter(map[string]string{"webhook_url": server.URL})

	result := a.Send(context.Background(), Message{
		Title: "Test",
		Body:  "Hello Discord",
		Level: Error,
		Fields: map[string]string{
			"Session": "abc123",
		},
	})

	if !result.Success {
		t.Errorf("Send failed: %v", result.Error)
	}

	embeds, ok := payload["embeds"].([]any)
	if !ok || len(embeds) == 0 {
		t.Fatal("expected embeds")
	}
	embed := embeds[0].(map[string]any)
	if embed["title"] != "Test" {
		t.Errorf("title = %v", embed["title"])
	}
	// Red color for Error level
	if embed["color"] != float64(0xe74c3c) {
		t.Errorf("color = %v, want %v", embed["color"], 0xe74c3c)
	}
}

func TestDiscordSendRateLimit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()

	a, _ := NewDiscordAdapter(map[string]string{"webhook_url": server.URL})
	result := a.Send(context.Background(), Message{Title: "Test"})
	if result.Success {
		t.Error("expected failure on 429")
	}
	if !result.Retryable {
		t.Error("429 should be retryable")
	}
}
