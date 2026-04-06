package hookio

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
)

// Allow returns a HookOutput that allows the action to proceed.
func Allow() *HookOutput {
	return &HookOutput{}
}

// Deny returns a HookOutput that blocks the action with a reason.
func Deny(reason string) *HookOutput {
	return &HookOutput{
		Decision: "deny",
		Reason:   reason,
	}
}

// Block is an alias for Deny.
func Block(reason string) *HookOutput {
	return Deny(reason)
}

// Ask returns a HookOutput that asks for user confirmation.
func Ask(reason string) *HookOutput {
	return &HookOutput{
		Decision: "ask",
		Reason:   reason,
	}
}

// InjectContext returns a HookOutput that adds context to the conversation.
func InjectContext(text string) *HookOutput {
	return &HookOutput{
		AdditionalContext: text,
	}
}

// ContinueWorking returns a HookOutput for Stop hooks that tells Claude to keep working.
func ContinueWorking(reason string) *HookOutput {
	t := true
	return &HookOutput{
		Continue:   &t,
		StopReason: reason,
	}
}

// StopWorking returns a HookOutput for Stop hooks that allows Claude to stop.
func StopWorking() *HookOutput {
	f := false
	return &HookOutput{
		Continue: &f,
	}
}

// AllowPermission returns a HookOutput that auto-approves a permission request.
func AllowPermission() *HookOutput {
	return &HookOutput{
		PermissionDecision: "allow",
	}
}

// DenyPermission returns a HookOutput that denies a permission request.
func DenyPermission(reason string) *HookOutput {
	return &HookOutput{
		PermissionDecision: "deny",
		Reason:             reason,
	}
}

// SuppressNotification returns a HookOutput that suppresses a notification.
func SuppressNotification() *HookOutput {
	t := true
	return &HookOutput{
		SuppressNotification: &t,
	}
}

// WriteOutput writes the hook output as JSON to the given writer.
func WriteOutput(w io.Writer, out *HookOutput) error {
	data, err := json.Marshal(out)
	if err != nil {
		return fmt.Errorf("marshaling output: %w", err)
	}
	_, err = w.Write(data)
	return err
}

// WriteStdout writes the hook output as JSON to stdout.
func WriteStdout(out *HookOutput) error {
	return WriteOutput(os.Stdout, out)
}

// JSON returns the hook output as a JSON byte slice.
func (h *HookOutput) JSON() ([]byte, error) {
	return json.Marshal(h)
}

// StopTeammate returns a HookOutput for TeammateIdle/TaskCreated/TaskCompleted that stops the teammate.
func StopTeammate(reason string) *HookOutput {
	f := false
	return &HookOutput{
		Continue:   &f,
		StopReason: reason,
	}
}

// WorktreePassthrough returns a HookOutput for WorktreeCreate hooks.
// The worktreePath is returned via hookSpecificOutput.
func WorktreePassthrough(worktreePath string) *HookOutput {
	return &HookOutput{
		HookSpecificOutput: map[string]any{
			"worktree_path": worktreePath,
		},
	}
}
