package adapters

import "context"

// Level represents the severity of a notification.
type Level int

const (
	Info Level = iota
	Warning
	Error
)

// String returns the level name.
func (l Level) String() string {
	switch l {
	case Info:
		return "info"
	case Warning:
		return "warning"
	case Error:
		return "error"
	default:
		return "info"
	}
}

// Message represents a notification to send.
type Message struct {
	Title   string
	Body    string
	Level   Level
	Fields  map[string]string
	Event   string
	Session string
}

// SendResult is the outcome of a Send or Test call.
type SendResult struct {
	Success   bool
	Error     error
	Retryable bool
}

// Adapter is the interface for notification channel implementations.
type Adapter interface {
	// Name returns the adapter identifier (e.g., "ntfy", "discord").
	Name() string

	// Send delivers a message through this channel.
	Send(ctx context.Context, msg Message) SendResult

	// Test sends a test message to verify configuration.
	Test(ctx context.Context) SendResult

	// ValidateConfig checks that configuration is complete and well-formed.
	ValidateConfig() error
}
