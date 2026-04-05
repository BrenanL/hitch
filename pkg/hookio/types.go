// Package hookio provides types and utilities for parsing Claude Code hook
// JSON input and building JSON output responses. This is the public library
// face of hitch — other tools can import it to interact with Claude Code hooks.
package hookio

import "encoding/json"

// StopFailureInput contains the structured fields for a StopFailure event.
type StopFailureInput struct {
	ErrorType    string `json:"error_type"`
	ErrorMessage string `json:"error_message"`
}

// TaskInput contains the structured fields for TaskCreated and TaskCompleted events.
type TaskInput struct {
	TaskID          string `json:"task_id"`
	TaskSubject     string `json:"task_subject"`
	TaskDescription string `json:"task_description,omitempty"`
	TeammateNameHook string `json:"teammate_name,omitempty"`
	TeamName        string `json:"team_name,omitempty"`
}

// ConfigChangeInput contains the structured fields for a ConfigChange event.
type ConfigChangeInput struct {
	ConfigSource string `json:"config_source"`
}

// PostCompactInput contains the structured fields for PostCompact (and PreCompact).
type PostCompactInput struct {
	CompactTrigger string `json:"compact_trigger"`
}

// WorktreeInput contains the structured fields for WorktreeCreate / WorktreeRemove.
type WorktreeInput struct {
	WorktreePath string `json:"worktree_path"`
	TargetBranch string `json:"target_branch,omitempty"`
	SourceBranch string `json:"source_branch,omitempty"`
}

// ElicitationInput contains the structured fields for Elicitation / ElicitationResult.
type ElicitationInput struct {
	ServerName        string          `json:"server_name"`
	ToolName          string          `json:"tool_name"`
	ElicitationSchema json.RawMessage `json:"elicitation_schema,omitempty"`
	UserAction        string          `json:"user_action,omitempty"`
	UserContent       json.RawMessage `json:"user_content,omitempty"`
}

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
	// PermissionDenied-specific
	PermissionDeniedReason string `json:"reason,omitempty"`
	// TaskCreated / TaskCompleted-specific
	TaskID           string `json:"task_id,omitempty"`
	TaskSubject      string `json:"task_subject,omitempty"`
	TaskDescription  string `json:"task_description,omitempty"`
	TeammateNameHook string `json:"teammate_name,omitempty"`
	TeamName         string `json:"team_name,omitempty"`
	TaskStatus       string `json:"task_status,omitempty"`
	// StopFailure-specific
	ErrorType    string `json:"error_type,omitempty"`
	ErrorMessage string `json:"error_message,omitempty"`
	// TeammateIdle-specific
	PendingTaskCount int `json:"pending_task_count,omitempty"`
	// InstructionsLoaded-specific (also used by FileChanged via the same JSON tag)
	EventFilePath string `json:"file_path,omitempty"`
	MemoryType    string `json:"memory_type,omitempty"`
	LoadReason    string `json:"load_reason,omitempty"`
	// ConfigChange-specific
	ConfigSource string `json:"config_source,omitempty"`
	// CwdChanged-specific
	OldCwd string `json:"old_cwd,omitempty"`
	NewCwd string `json:"new_cwd,omitempty"`
	// FileChanged-specific
	ChangeType string `json:"change_type,omitempty"`
	// WorktreeCreate / WorktreeRemove-specific
	WorktreePath string `json:"worktree_path,omitempty"`
	TargetBranch string `json:"target_branch,omitempty"`
	SourceBranch string `json:"source_branch,omitempty"`
	// PostCompact-specific
	CompactTrigger string `json:"compact_trigger,omitempty"`
	// Elicitation / ElicitationResult-specific
	ServerName        string          `json:"server_name,omitempty"`
	ElicitationSchema json.RawMessage `json:"elicitation_schema,omitempty"`
	UserAction        string          `json:"user_action,omitempty"`
	UserContent       json.RawMessage `json:"user_content,omitempty"`
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
	// For WorktreeCreate and other hooks that return structured data
	HookSpecificOutput map[string]any `json:"hookSpecificOutput,omitempty"`
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
	EventPermissionDenied   = "PermissionDenied"
	EventStopFailure        = "StopFailure"
	EventTaskCreated        = "TaskCreated"
	EventTaskCompleted      = "TaskCompleted"
	EventTeammateIdle       = "TeammateIdle"
	EventInstructionsLoaded = "InstructionsLoaded"
	EventConfigChange       = "ConfigChange"
	EventCwdChanged         = "CwdChanged"
	EventFileChanged        = "FileChanged"
	EventWorktreeCreate     = "WorktreeCreate"
	EventWorktreeRemove     = "WorktreeRemove"
	EventPostCompact        = "PostCompact"
	EventElicitation        = "Elicitation"
	EventElicitationResult  = "ElicitationResult"
)
