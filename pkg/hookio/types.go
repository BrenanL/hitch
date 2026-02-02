// Package hookio provides types and utilities for parsing Claude Code hook
// JSON input and building JSON output responses. This is the public library
// face of hitch — other tools can import it to interact with Claude Code hooks.
package hookio

import "encoding/json"

// HookInput represents the JSON input piped to a hook via stdin.
type HookInput struct {
	SessionID      string          `json:"session_id"`
	TranscriptPath string          `json:"transcript_path"`
	Cwd            string          `json:"cwd"`
	PermissionMode string          `json:"permission_mode"`
	HookEventName  string          `json:"hook_event_name"`
	ToolName       string          `json:"tool_name,omitempty"`
	ToolInput      json.RawMessage `json:"tool_input,omitempty"`
	ToolResult     json.RawMessage `json:"tool_result,omitempty"`
	StopHookActive bool            `json:"stop_hook_active,omitempty"`
	// Notification-specific
	NotificationType string `json:"notification_type,omitempty"`
	NotificationBody string `json:"notification_body,omitempty"`
	// SubagentStart/Stop-specific
	AgentName string `json:"agent_name,omitempty"`
	AgentType string `json:"agent_type,omitempty"`
	// SessionStart-specific
	SessionType string `json:"session_type,omitempty"`
	// UserPromptSubmit-specific
	UserPrompt string `json:"user_prompt,omitempty"`
	// Stop-specific
	StopReason string `json:"stop_reason,omitempty"`
}

// ToolInputBash represents the tool_input for a Bash tool call.
type ToolInputBash struct {
	Command string `json:"command"`
	Timeout int    `json:"timeout,omitempty"`
}

// ToolInputEdit represents the tool_input for an Edit tool call.
type ToolInputEdit struct {
	FilePath  string `json:"file_path"`
	OldString string `json:"old_string,omitempty"`
	NewString string `json:"new_string,omitempty"`
}

// ToolInputWrite represents the tool_input for a Write tool call.
type ToolInputWrite struct {
	FilePath string `json:"file_path"`
	Content  string `json:"content"`
}

// ToolInputRead represents the tool_input for a Read tool call.
type ToolInputRead struct {
	FilePath string `json:"file_path"`
}

// ToolInputGlob represents the tool_input for a Glob tool call.
type ToolInputGlob struct {
	Pattern string `json:"pattern"`
	Path    string `json:"path,omitempty"`
}

// ToolInputGrep represents the tool_input for a Grep tool call.
type ToolInputGrep struct {
	Pattern string `json:"pattern"`
	Path    string `json:"path,omitempty"`
}

// HookOutput represents the JSON output written to stdout by a hook.
type HookOutput struct {
	// For PreToolUse: "allow", "deny", or "ask"
	Decision string `json:"decision,omitempty"`
	// Reason for the decision
	Reason string `json:"reason,omitempty"`
	// Additional context to inject into conversation
	AdditionalContext string `json:"additionalContext,omitempty"`
	// For Stop hooks: false = stop (allow), true = continue working
	Continue *bool `json:"continue,omitempty"`
	// For Stop hooks: reason to continue
	StopReason string `json:"stopReason,omitempty"`
	// For Notification hooks
	SuppressNotification *bool `json:"suppressNotification,omitempty"`
	// For PermissionRequest hooks: "allow" or "deny"
	PermissionDecision string `json:"permissionDecision,omitempty"`
}

// Event name constants matching Claude Code's hook event names.
const (
	EventSessionStart       = "SessionStart"
	EventSessionEnd         = "SessionEnd"
	EventUserPromptSubmit   = "UserPromptSubmit"
	EventPreToolUse         = "PreToolUse"
	EventPostToolUse        = "PostToolUse"
	EventPostToolUseFailure = "PostToolUseFailure"
	EventPermissionRequest  = "PermissionRequest"
	EventNotification       = "Notification"
	EventSubagentStart      = "SubagentStart"
	EventSubagentStop       = "SubagentStop"
	EventStop               = "Stop"
	EventPreCompact         = "PreCompact"
)
