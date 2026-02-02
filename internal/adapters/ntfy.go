package adapters

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
)

const defaultNtfyServer = "https://ntfy.sh"

// NtfyAdapter sends notifications via ntfy.sh or a custom ntfy server.
type NtfyAdapter struct {
	topic  string
	server string
	client *http.Client
}

// NewNtfyAdapter creates a new ntfy adapter from config.
// Required config: "topic". Optional: "server" (defaults to ntfy.sh).
func NewNtfyAdapter(config map[string]string) (Adapter, error) {
	a := &NtfyAdapter{
		topic:  config["topic"],
		server: config["server"],
		client: http.DefaultClient,
	}
	if a.server == "" {
		a.server = defaultNtfyServer
	}
	if err := a.ValidateConfig(); err != nil {
		return nil, err
	}
	return a, nil
}

func (a *NtfyAdapter) Name() string { return "ntfy" }

func (a *NtfyAdapter) ValidateConfig() error {
	if a.topic == "" {
		return fmt.Errorf("ntfy: topic is required")
	}
	return nil
}

func (a *NtfyAdapter) Send(ctx context.Context, msg Message) SendResult {
	url := fmt.Sprintf("%s/%s", a.server, a.topic)

	body := msg.Body
	if body == "" {
		body = msg.Title
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBufferString(body))
	if err != nil {
		return SendResult{Error: fmt.Errorf("ntfy: creating request: %w", err)}
	}

	if msg.Title != "" {
		req.Header.Set("Title", msg.Title)
	}

	// Map level to ntfy priority
	switch msg.Level {
	case Error:
		req.Header.Set("Priority", "urgent")
	case Warning:
		req.Header.Set("Priority", "high")
	default:
		req.Header.Set("Priority", "default")
	}

	// Add tags based on event
	if msg.Event != "" {
		req.Header.Set("Tags", msg.Event)
	}

	resp, err := a.client.Do(req)
	if err != nil {
		return SendResult{Error: fmt.Errorf("ntfy: sending: %w", err), Retryable: true}
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)

	if resp.StatusCode >= 400 {
		retryable := resp.StatusCode >= 500
		return SendResult{Error: fmt.Errorf("ntfy: HTTP %d", resp.StatusCode), Retryable: retryable}
	}

	return SendResult{Success: true}
}

func (a *NtfyAdapter) Test(ctx context.Context) SendResult {
	return a.Send(ctx, Message{
		Title: "Hitch Test",
		Body:  "If you see this, your ntfy channel is working.",
		Level: Info,
	})
}
