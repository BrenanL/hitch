package hookio

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
)

// ReadInput reads and parses hook input JSON from the given reader.
func ReadInput(r io.Reader) (*HookInput, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("reading input: %w", err)
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("empty input")
	}

	var input HookInput
	if err := json.Unmarshal(data, &input); err != nil {
		return nil, fmt.Errorf("parsing input JSON: %w", err)
	}

	if input.HookEventName == "" {
		return nil, fmt.Errorf("missing hook_event_name in input")
	}

	return &input, nil
}

// ReadStdin reads and parses hook input JSON from stdin.
func ReadStdin() (*HookInput, error) {
	return ReadInput(os.Stdin)
}

// ParseToolInputBash parses the tool_input field as a Bash command.
func (h *HookInput) ParseToolInputBash() (*ToolInputBash, error) {
	if h.ToolInput == nil {
		return nil, fmt.Errorf("no tool_input")
	}
	var ti ToolInputBash
	if err := json.Unmarshal(h.ToolInput, &ti); err != nil {
		return nil, fmt.Errorf("parsing bash tool_input: %w", err)
	}
	return &ti, nil
}

// ParseToolInputEdit parses the tool_input field as an Edit command.
func (h *HookInput) ParseToolInputEdit() (*ToolInputEdit, error) {
	if h.ToolInput == nil {
		return nil, fmt.Errorf("no tool_input")
	}
	var ti ToolInputEdit
	if err := json.Unmarshal(h.ToolInput, &ti); err != nil {
		return nil, fmt.Errorf("parsing edit tool_input: %w", err)
	}
	return &ti, nil
}

// ParseToolInputWrite parses the tool_input field as a Write command.
func (h *HookInput) ParseToolInputWrite() (*ToolInputWrite, error) {
	if h.ToolInput == nil {
		return nil, fmt.Errorf("no tool_input")
	}
	var ti ToolInputWrite
	if err := json.Unmarshal(h.ToolInput, &ti); err != nil {
		return nil, fmt.Errorf("parsing write tool_input: %w", err)
	}
	return &ti, nil
}

// Command extracts the command string from a Bash tool_input.
// Returns empty string if not a Bash tool or if parsing fails.
func (h *HookInput) Command() string {
	ti, err := h.ParseToolInputBash()
	if err != nil {
		return ""
	}
	return ti.Command
}

// FilePath extracts the file_path from an Edit or Write tool_input.
// Returns empty string if not applicable or if parsing fails.
func (h *HookInput) FilePath() string {
	if h.ToolInput == nil {
		return ""
	}
	var fp struct {
		FilePath string `json:"file_path"`
	}
	if err := json.Unmarshal(h.ToolInput, &fp); err != nil {
		return ""
	}
	return fp.FilePath
}

// StopFailure returns typed fields from a StopFailure event.
// Returns nil if event type is wrong.
func (h *HookInput) StopFailure() *StopFailureInput {
	if h.HookEventName != EventStopFailure {
		return nil
	}
	return &StopFailureInput{
		ErrorType:    h.ErrorType,
		ErrorMessage: h.ErrorMessage,
	}
}

// Task returns typed fields from a TaskCreated or TaskCompleted event.
// Returns nil if event type is wrong.
func (h *HookInput) Task() *TaskInput {
	if h.HookEventName != EventTaskCreated && h.HookEventName != EventTaskCompleted {
		return nil
	}
	return &TaskInput{
		TaskID:          h.TaskID,
		TaskSubject:     h.TaskSubject,
		TaskDescription: h.TaskDescription,
		TeammateNameHook: h.TeammateNameHook,
		TeamName:        h.TeamName,
	}
}

// ConfigChange returns typed fields from a ConfigChange event.
// Returns nil if event type is wrong.
func (h *HookInput) ConfigChange() *ConfigChangeInput {
	if h.HookEventName != EventConfigChange {
		return nil
	}
	return &ConfigChangeInput{
		ConfigSource: h.ConfigSource,
	}
}

// Worktree returns typed fields from a WorktreeCreate or WorktreeRemove event.
// Returns nil if event type is wrong.
func (h *HookInput) Worktree() *WorktreeInput {
	if h.HookEventName != EventWorktreeCreate && h.HookEventName != EventWorktreeRemove {
		return nil
	}
	return &WorktreeInput{
		WorktreePath: h.WorktreePath,
		TargetBranch: h.TargetBranch,
		SourceBranch: h.SourceBranch,
	}
}

// Elicitation returns typed fields from an Elicitation or ElicitationResult event.
// Returns nil if event type is wrong.
func (h *HookInput) Elicitation() *ElicitationInput {
	if h.HookEventName != EventElicitation && h.HookEventName != EventElicitationResult {
		return nil
	}
	return &ElicitationInput{
		ServerName:        h.ServerName,
		ToolName:          h.ToolName,
		ElicitationSchema: h.ElicitationSchema,
		UserAction:        h.UserAction,
		UserContent:       h.UserContent,
	}
}
