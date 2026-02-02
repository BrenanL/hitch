package adapters

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNtfyValidateConfig(t *testing.T) {
	// Missing topic
	_, err := NewNtfyAdapter(map[string]string{})
	if err == nil {
		t.Error("expected error for missing topic")
	}

	// Valid config
	_, err = NewNtfyAdapter(map[string]string{"topic": "test"})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestNtfySend(t *testing.T) {
	var gotTitle, gotPriority, gotBody string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotTitle = r.Header.Get("Title")
		gotPriority = r.Header.Get("Priority")
		buf := make([]byte, 1024)
		n, _ := r.Body.Read(buf)
		gotBody = string(buf[:n])
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	a, err := NewNtfyAdapter(map[string]string{
		"topic":  "test",
		"server": server.URL,
	})
	if err != nil {
		t.Fatalf("NewNtfyAdapter: %v", err)
	}

	result := a.Send(context.Background(), Message{
		Title: "Test Title",
		Body:  "Test Body",
		Level: Warning,
		Event: "Stop",
	})

	if !result.Success {
		t.Errorf("Send failed: %v", result.Error)
	}
	if gotTitle != "Test Title" {
		t.Errorf("title = %q, want %q", gotTitle, "Test Title")
	}
	if gotPriority != "high" {
		t.Errorf("priority = %q, want %q", gotPriority, "high")
	}
	if gotBody != "Test Body" {
		t.Errorf("body = %q, want %q", gotBody, "Test Body")
	}
}

func TestNtfySendError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	a, _ := NewNtfyAdapter(map[string]string{
		"topic":  "test",
		"server": server.URL,
	})

	result := a.Send(context.Background(), Message{Title: "Test"})
	if result.Success {
		t.Error("expected failure")
	}
	if !result.Retryable {
		t.Error("5xx should be retryable")
	}
}

func TestNtfyName(t *testing.T) {
	a, _ := NewNtfyAdapter(map[string]string{"topic": "test"})
	if a.Name() != "ntfy" {
		t.Errorf("Name() = %q, want %q", a.Name(), "ntfy")
	}
}
