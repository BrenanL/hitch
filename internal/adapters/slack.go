package adapters

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// SlackAdapter sends notifications via Slack incoming webhook.
type SlackAdapter struct {
	webhookURL string
	client     *http.Client
}

// NewSlackAdapter creates a new Slack adapter from config.
// Required config: "webhook_url".
func NewSlackAdapter(config map[string]string) (Adapter, error) {
	a := &SlackAdapter{
		webhookURL: config["webhook_url"],
		client:     http.DefaultClient,
	}
	if err := a.ValidateConfig(); err != nil {
		return nil, err
	}
	return a, nil
}

func (a *SlackAdapter) Name() string { return "slack" }

func (a *SlackAdapter) ValidateConfig() error {
	if a.webhookURL == "" {
		return fmt.Errorf("slack: webhook_url is required")
	}
	return nil
}

func (a *SlackAdapter) Send(ctx context.Context, msg Message) SendResult {
	// Build Slack Block Kit message
	blocks := []any{}

	// Header
	if msg.Title != "" {
		blocks = append(blocks, map[string]any{
			"type": "header",
			"text": map[string]any{
				"type": "plain_text",
				"text": msg.Title,
			},
		})
	}

	// Body
	if msg.Body != "" {
		// Prefix with emoji based on level
		prefix := ""
		switch msg.Level {
		case Warning:
			prefix = ":warning: "
		case Error:
			prefix = ":x: "
		}
		blocks = append(blocks, map[string]any{
			"type": "section",
			"text": map[string]any{
				"type": "mrkdwn",
				"text": prefix + msg.Body,
			},
		})
	}

	// Fields as context block
	if len(msg.Fields) > 0 {
		elements := make([]any, 0, len(msg.Fields))
		for k, v := range msg.Fields {
			elements = append(elements, map[string]any{
				"type": "mrkdwn",
				"text": fmt.Sprintf("*%s:* %s", k, v),
			})
		}
		blocks = append(blocks, map[string]any{
			"type":     "context",
			"elements": elements,
		})
	}

	payload := map[string]any{
		"blocks": blocks,
	}

	// Also include plain text fallback
	if msg.Body != "" {
		payload["text"] = msg.Body
	} else {
		payload["text"] = msg.Title
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return SendResult{Error: fmt.Errorf("slack: marshaling: %w", err)}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.webhookURL, bytes.NewReader(data))
	if err != nil {
		return SendResult{Error: fmt.Errorf("slack: creating request: %w", err)}
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.client.Do(req)
	if err != nil {
		return SendResult{Error: fmt.Errorf("slack: sending: %w", err), Retryable: true}
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)

	if resp.StatusCode >= 400 {
		retryable := resp.StatusCode >= 500 || resp.StatusCode == 429
		return SendResult{Error: fmt.Errorf("slack: HTTP %d", resp.StatusCode), Retryable: retryable}
	}

	return SendResult{Success: true}
}

func (a *SlackAdapter) Test(ctx context.Context) SendResult {
	return a.Send(ctx, Message{
		Title: "Hitch Test",
		Body:  "If you see this, your Slack channel is working.",
		Level: Info,
	})
}
