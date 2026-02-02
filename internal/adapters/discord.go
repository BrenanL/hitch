package adapters

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// DiscordAdapter sends notifications via Discord webhook.
type DiscordAdapter struct {
	webhookURL string
	client     *http.Client
}

// NewDiscordAdapter creates a new Discord adapter from config.
// Required config: "webhook_url".
func NewDiscordAdapter(config map[string]string) (Adapter, error) {
	a := &DiscordAdapter{
		webhookURL: config["webhook_url"],
		client:     http.DefaultClient,
	}
	if err := a.ValidateConfig(); err != nil {
		return nil, err
	}
	return a, nil
}

func (a *DiscordAdapter) Name() string { return "discord" }

func (a *DiscordAdapter) ValidateConfig() error {
	if a.webhookURL == "" {
		return fmt.Errorf("discord: webhook_url is required")
	}
	return nil
}

func (a *DiscordAdapter) Send(ctx context.Context, msg Message) SendResult {
	// Build Discord embed
	color := 0x3498db // blue for info
	switch msg.Level {
	case Warning:
		color = 0xf39c12 // orange
	case Error:
		color = 0xe74c3c // red
	}

	embed := map[string]any{
		"title":       msg.Title,
		"description": msg.Body,
		"color":       color,
	}

	// Add fields
	if len(msg.Fields) > 0 {
		fields := make([]map[string]any, 0, len(msg.Fields))
		for k, v := range msg.Fields {
			fields = append(fields, map[string]any{
				"name":   k,
				"value":  v,
				"inline": true,
			})
		}
		embed["fields"] = fields
	}

	payload := map[string]any{
		"embeds": []any{embed},
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return SendResult{Error: fmt.Errorf("discord: marshaling: %w", err)}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.webhookURL, bytes.NewReader(data))
	if err != nil {
		return SendResult{Error: fmt.Errorf("discord: creating request: %w", err)}
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.client.Do(req)
	if err != nil {
		return SendResult{Error: fmt.Errorf("discord: sending: %w", err), Retryable: true}
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)

	if resp.StatusCode >= 400 {
		retryable := resp.StatusCode >= 500 || resp.StatusCode == 429
		return SendResult{Error: fmt.Errorf("discord: HTTP %d", resp.StatusCode), Retryable: retryable}
	}

	return SendResult{Success: true}
}

func (a *DiscordAdapter) Test(ctx context.Context) SendResult {
	return a.Send(ctx, Message{
		Title: "Hitch Test",
		Body:  "If you see this, your Discord channel is working.",
		Level: Info,
	})
}
