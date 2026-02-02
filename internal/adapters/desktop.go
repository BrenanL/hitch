package adapters

import (
	"context"
	"fmt"

	"github.com/BrenanL/hitch/internal/platform"
)

// DesktopAdapter sends OS-native desktop notifications.
type DesktopAdapter struct{}

// NewDesktopAdapter creates a new desktop notification adapter.
func NewDesktopAdapter(config map[string]string) (Adapter, error) {
	return &DesktopAdapter{}, nil
}

func (a *DesktopAdapter) Name() string { return "desktop" }

func (a *DesktopAdapter) ValidateConfig() error {
	return nil
}

func (a *DesktopAdapter) Send(ctx context.Context, msg Message) SendResult {
	title := msg.Title
	if title == "" {
		title = "Hitch"
	}
	body := msg.Body
	if body == "" {
		body = title
	}

	urgency := "normal"
	switch msg.Level {
	case Warning:
		urgency = "normal"
	case Error:
		urgency = "critical"
	}

	if err := platform.Notify(title, body, urgency); err != nil {
		return SendResult{Error: fmt.Errorf("desktop: %w", err)}
	}
	return SendResult{Success: true}
}

func (a *DesktopAdapter) Test(ctx context.Context) SendResult {
	return a.Send(ctx, Message{
		Title: "Hitch Test",
		Body:  "If you see this, your desktop notifications are working.",
		Level: Info,
	})
}
