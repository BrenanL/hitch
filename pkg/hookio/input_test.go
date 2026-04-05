package hookio

import (
	"strings"
	"testing"
)

func TestReadInputPreToolUseBash(t *testing.T) {
	input := `{
		"session_id": "abc123",
		"transcript_path": "/tmp/transcript.jsonl",
		"cwd": "/home/user/project",
		"permission_mode": "default",
		"hook_event_name": "PreToolUse",
		"tool_name": "Bash",
		"tool_input": {"command": "npm test"}
	}`

	hi, err := ReadInput(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ReadInput: %v", err)
	}

	if hi.SessionID != "abc123" {
		t.Errorf("SessionID = %q, want %q", hi.SessionID, "abc123")
	}
	if hi.HookEventName != "PreToolUse" {
		t.Errorf("HookEventName = %q, want %q", hi.HookEventName, "PreToolUse")
	}
	if hi.ToolName != "Bash" {
		t.Errorf("ToolName = %q, want %q", hi.ToolName, "Bash")
	}
	if hi.Command() != "npm test" {
		t.Errorf("Command() = %q, want %q", hi.Command(), "npm test")
	}
}

func TestReadInputPreToolUseEdit(t *testing.T) {
	input := `{
		"session_id": "abc123",
		"cwd": "/home/user/project",
		"hook_event_name": "PreToolUse",
		"tool_name": "Edit",
		"tool_input": {"file_path": "/src/main.go", "old_string": "foo", "new_string": "bar"}
	}`

	hi, err := ReadInput(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ReadInput: %v", err)
	}

	if hi.FilePath() != "/src/main.go" {
		t.Errorf("FilePath() = %q, want %q", hi.FilePath(), "/src/main.go")
	}

	ti, err := hi.ParseToolInputEdit()
	if err != nil {
		t.Fatalf("ParseToolInputEdit: %v", err)
	}
	if ti.OldString != "foo" {
		t.Errorf("OldString = %q, want %q", ti.OldString, "foo")
	}
}

func TestReadInputPostToolUse(t *testing.T) {
	input := `{
		"session_id": "s1",
		"cwd": "/proj",
		"hook_event_name": "PostToolUse",
		"tool_name": "Bash",
		"tool_input": {"command": "ls"},
		"tool_result": {"stdout": "file.txt\n"}
	}`

	hi, err := ReadInput(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ReadInput: %v", err)
	}
	if hi.HookEventName != "PostToolUse" {
		t.Errorf("HookEventName = %q, want %q", hi.HookEventName, "PostToolUse")
	}
	if hi.ToolResult == nil {
		t.Error("ToolResult should not be nil")
	}
}

func TestReadInputPostToolUseFailure(t *testing.T) {
	input := `{
		"session_id": "s1",
		"cwd": "/proj",
		"hook_event_name": "PostToolUseFailure",
		"tool_name": "Bash",
		"tool_input": {"command": "false"}
	}`

	hi, err := ReadInput(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ReadInput: %v", err)
	}
	if hi.HookEventName != "PostToolUseFailure" {
		t.Errorf("HookEventName = %q, want %q", hi.HookEventName, "PostToolUseFailure")
	}
}

func TestReadInputStop(t *testing.T) {
	input := `{
		"session_id": "s1",
		"cwd": "/proj",
		"hook_event_name": "Stop",
		"stop_hook_active": false,
		"stop_reason": "task_complete"
	}`

	hi, err := ReadInput(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ReadInput: %v", err)
	}
	if hi.HookEventName != "Stop" {
		t.Errorf("HookEventName = %q, want %q", hi.HookEventName, "Stop")
	}
	if hi.StopHookActive {
		t.Error("StopHookActive should be false")
	}
	if hi.StopReason != "task_complete" {
		t.Errorf("StopReason = %q, want %q", hi.StopReason, "task_complete")
	}
}

func TestReadInputSessionStart(t *testing.T) {
	input := `{
		"session_id": "s1",
		"cwd": "/proj",
		"hook_event_name": "SessionStart",
		"session_type": "startup"
	}`

	hi, err := ReadInput(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ReadInput: %v", err)
	}
	if hi.HookEventName != "SessionStart" {
		t.Errorf("HookEventName = %q, want %q", hi.HookEventName, "SessionStart")
	}
	if hi.SessionType != "startup" {
		t.Errorf("SessionType = %q, want %q", hi.SessionType, "startup")
	}
}

func TestReadInputSessionEnd(t *testing.T) {
	input := `{
		"session_id": "s1",
		"cwd": "/proj",
		"hook_event_name": "SessionEnd"
	}`

	hi, err := ReadInput(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ReadInput: %v", err)
	}
	if hi.HookEventName != "SessionEnd" {
		t.Errorf("HookEventName = %q, want %q", hi.HookEventName, "SessionEnd")
	}
}

func TestReadInputNotification(t *testing.T) {
	input := `{
		"session_id": "s1",
		"cwd": "/proj",
		"hook_event_name": "Notification",
		"notification_type": "permission_prompt",
		"notification_body": "Claude needs permission to run Bash"
	}`

	hi, err := ReadInput(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ReadInput: %v", err)
	}
	if hi.NotificationType != "permission_prompt" {
		t.Errorf("NotificationType = %q, want %q", hi.NotificationType, "permission_prompt")
	}
	if hi.NotificationBody != "Claude needs permission to run Bash" {
		t.Errorf("NotificationBody = %q", hi.NotificationBody)
	}
}

func TestReadInputPermissionRequest(t *testing.T) {
	input := `{
		"session_id": "s1",
		"cwd": "/proj",
		"hook_event_name": "PermissionRequest",
		"tool_name": "Bash",
		"tool_input": {"command": "rm -rf /tmp/test"}
	}`

	hi, err := ReadInput(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ReadInput: %v", err)
	}
	if hi.HookEventName != "PermissionRequest" {
		t.Errorf("HookEventName = %q, want %q", hi.HookEventName, "PermissionRequest")
	}
}

func TestReadInputUserPromptSubmit(t *testing.T) {
	input := `{
		"session_id": "s1",
		"cwd": "/proj",
		"hook_event_name": "UserPromptSubmit",
		"user_prompt": "Fix the bug in auth.go"
	}`

	hi, err := ReadInput(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ReadInput: %v", err)
	}
	if hi.UserPrompt != "Fix the bug in auth.go" {
		t.Errorf("UserPrompt = %q", hi.UserPrompt)
	}
}

func TestReadInputSubagentStart(t *testing.T) {
	input := `{
		"session_id": "s1",
		"cwd": "/proj",
		"hook_event_name": "SubagentStart",
		"agent_name": "test-runner",
		"agent_type": "Bash"
	}`

	hi, err := ReadInput(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ReadInput: %v", err)
	}
	if hi.AgentName != "test-runner" {
		t.Errorf("AgentName = %q, want %q", hi.AgentName, "test-runner")
	}
}

func TestReadInputSubagentStop(t *testing.T) {
	input := `{
		"session_id": "s1",
		"cwd": "/proj",
		"hook_event_name": "SubagentStop",
		"agent_name": "test-runner"
	}`

	hi, err := ReadInput(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ReadInput: %v", err)
	}
	if hi.HookEventName != "SubagentStop" {
		t.Errorf("HookEventName = %q, want %q", hi.HookEventName, "SubagentStop")
	}
}

func TestReadInputPreCompact(t *testing.T) {
	input := `{
		"session_id": "s1",
		"cwd": "/proj",
		"hook_event_name": "PreCompact"
	}`

	hi, err := ReadInput(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ReadInput: %v", err)
	}
	if hi.HookEventName != "PreCompact" {
		t.Errorf("HookEventName = %q, want %q", hi.HookEventName, "PreCompact")
	}
}

func TestReadInputEmpty(t *testing.T) {
	_, err := ReadInput(strings.NewReader(""))
	if err == nil {
		t.Error("expected error for empty input")
	}
}

func TestReadInputInvalidJSON(t *testing.T) {
	_, err := ReadInput(strings.NewReader("not json"))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestReadInputMissingEventName(t *testing.T) {
	_, err := ReadInput(strings.NewReader(`{"session_id": "s1"}`))
	if err == nil {
		t.Error("expected error for missing hook_event_name")
	}
}

func TestCommandAndFilePathOnNonToolInput(t *testing.T) {
	input := `{
		"session_id": "s1",
		"cwd": "/proj",
		"hook_event_name": "Stop"
	}`
	hi, _ := ReadInput(strings.NewReader(input))
	if hi.Command() != "" {
		t.Errorf("Command() on non-tool event should be empty")
	}
	if hi.FilePath() != "" {
		t.Errorf("FilePath() on non-tool event should be empty")
	}
}

func TestStopFailureTyped(t *testing.T) {
	input := `{
		"session_id": "s1",
		"cwd": "/proj",
		"hook_event_name": "StopFailure",
		"error_type": "timeout",
		"error_message": "operation timed out"
	}`
	hi, err := ReadInput(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ReadInput: %v", err)
	}
	sf := hi.StopFailure()
	if sf == nil {
		t.Fatal("StopFailure() returned nil")
	}
	if sf.ErrorType != "timeout" {
		t.Errorf("ErrorType = %q, want %q", sf.ErrorType, "timeout")
	}
	if sf.ErrorMessage != "operation timed out" {
		t.Errorf("ErrorMessage = %q, want %q", sf.ErrorMessage, "operation timed out")
	}
}

func TestStopFailureWrongEvent(t *testing.T) {
	input := `{
		"session_id": "s1",
		"cwd": "/proj",
		"hook_event_name": "Stop"
	}`
	hi, _ := ReadInput(strings.NewReader(input))
	if hi.StopFailure() != nil {
		t.Error("StopFailure() should return nil for wrong event type")
	}
}

func TestTaskTypedCreated(t *testing.T) {
	input := `{
		"session_id": "s1",
		"cwd": "/proj",
		"hook_event_name": "TaskCreated",
		"task_id": "task-123",
		"task_subject": "Fix bug",
		"task_description": "Detailed description",
		"teammate_name": "alice",
		"team_name": "alpha"
	}`
	hi, err := ReadInput(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ReadInput: %v", err)
	}
	ti := hi.Task()
	if ti == nil {
		t.Fatal("Task() returned nil")
	}
	if ti.TaskID != "task-123" {
		t.Errorf("TaskID = %q, want %q", ti.TaskID, "task-123")
	}
	if ti.TaskSubject != "Fix bug" {
		t.Errorf("TaskSubject = %q, want %q", ti.TaskSubject, "Fix bug")
	}
	if ti.TaskDescription != "Detailed description" {
		t.Errorf("TaskDescription = %q, want %q", ti.TaskDescription, "Detailed description")
	}
	if ti.TeammateNameHook != "alice" {
		t.Errorf("TeammateNameHook = %q, want %q", ti.TeammateNameHook, "alice")
	}
	if ti.TeamName != "alpha" {
		t.Errorf("TeamName = %q, want %q", ti.TeamName, "alpha")
	}
}

func TestTaskTypedCompleted(t *testing.T) {
	input := `{
		"session_id": "s1",
		"cwd": "/proj",
		"hook_event_name": "TaskCompleted",
		"task_id": "task-456",
		"task_subject": "Deploy service"
	}`
	hi, err := ReadInput(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ReadInput: %v", err)
	}
	ti := hi.Task()
	if ti == nil {
		t.Fatal("Task() returned nil for TaskCompleted")
	}
	if ti.TaskID != "task-456" {
		t.Errorf("TaskID = %q, want %q", ti.TaskID, "task-456")
	}
}

func TestTaskWrongEvent(t *testing.T) {
	input := `{
		"session_id": "s1",
		"cwd": "/proj",
		"hook_event_name": "Stop"
	}`
	hi, _ := ReadInput(strings.NewReader(input))
	if hi.Task() != nil {
		t.Error("Task() should return nil for wrong event type")
	}
}

func TestConfigChangeTyped(t *testing.T) {
	input := `{
		"session_id": "s1",
		"cwd": "/proj",
		"hook_event_name": "ConfigChange",
		"config_source": "project_settings"
	}`
	hi, err := ReadInput(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ReadInput: %v", err)
	}
	cc := hi.ConfigChange()
	if cc == nil {
		t.Fatal("ConfigChange() returned nil")
	}
	if cc.ConfigSource != "project_settings" {
		t.Errorf("ConfigSource = %q, want %q", cc.ConfigSource, "project_settings")
	}
}

func TestConfigChangeWrongEvent(t *testing.T) {
	input := `{
		"session_id": "s1",
		"cwd": "/proj",
		"hook_event_name": "Stop"
	}`
	hi, _ := ReadInput(strings.NewReader(input))
	if hi.ConfigChange() != nil {
		t.Error("ConfigChange() should return nil for wrong event type")
	}
}

func TestWorktreeTypedCreate(t *testing.T) {
	input := `{
		"session_id": "s1",
		"cwd": "/proj",
		"hook_event_name": "WorktreeCreate",
		"worktree_path": "/proj/.claude/worktrees/feature",
		"target_branch": "feature/new-thing",
		"source_branch": "main"
	}`
	hi, err := ReadInput(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ReadInput: %v", err)
	}
	wt := hi.Worktree()
	if wt == nil {
		t.Fatal("Worktree() returned nil")
	}
	if wt.WorktreePath != "/proj/.claude/worktrees/feature" {
		t.Errorf("WorktreePath = %q, want %q", wt.WorktreePath, "/proj/.claude/worktrees/feature")
	}
	if wt.TargetBranch != "feature/new-thing" {
		t.Errorf("TargetBranch = %q, want %q", wt.TargetBranch, "feature/new-thing")
	}
	if wt.SourceBranch != "main" {
		t.Errorf("SourceBranch = %q, want %q", wt.SourceBranch, "main")
	}
}

func TestWorktreeTypedRemove(t *testing.T) {
	input := `{
		"session_id": "s1",
		"cwd": "/proj",
		"hook_event_name": "WorktreeRemove",
		"worktree_path": "/proj/.claude/worktrees/old"
	}`
	hi, err := ReadInput(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ReadInput: %v", err)
	}
	wt := hi.Worktree()
	if wt == nil {
		t.Fatal("Worktree() returned nil for WorktreeRemove")
	}
	if wt.WorktreePath != "/proj/.claude/worktrees/old" {
		t.Errorf("WorktreePath = %q, want %q", wt.WorktreePath, "/proj/.claude/worktrees/old")
	}
}

func TestWorktreeWrongEvent(t *testing.T) {
	input := `{
		"session_id": "s1",
		"cwd": "/proj",
		"hook_event_name": "Stop"
	}`
	hi, _ := ReadInput(strings.NewReader(input))
	if hi.Worktree() != nil {
		t.Error("Worktree() should return nil for wrong event type")
	}
}

func TestElicitationTyped(t *testing.T) {
	input := `{
		"session_id": "s1",
		"cwd": "/proj",
		"hook_event_name": "Elicitation",
		"server_name": "my-mcp-server",
		"tool_name": "some_tool",
		"elicitation_schema": {"type": "object"}
	}`
	hi, err := ReadInput(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ReadInput: %v", err)
	}
	el := hi.Elicitation()
	if el == nil {
		t.Fatal("Elicitation() returned nil")
	}
	if el.ServerName != "my-mcp-server" {
		t.Errorf("ServerName = %q, want %q", el.ServerName, "my-mcp-server")
	}
	if el.ToolName != "some_tool" {
		t.Errorf("ToolName = %q, want %q", el.ToolName, "some_tool")
	}
	if el.ElicitationSchema == nil {
		t.Error("ElicitationSchema should not be nil")
	}
}

func TestElicitationResultTyped(t *testing.T) {
	input := `{
		"session_id": "s1",
		"cwd": "/proj",
		"hook_event_name": "ElicitationResult",
		"server_name": "my-mcp-server",
		"tool_name": "some_tool",
		"user_action": "accept",
		"user_content": {"answer": "yes"}
	}`
	hi, err := ReadInput(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ReadInput: %v", err)
	}
	el := hi.Elicitation()
	if el == nil {
		t.Fatal("Elicitation() returned nil for ElicitationResult")
	}
	if el.UserAction != "accept" {
		t.Errorf("UserAction = %q, want %q", el.UserAction, "accept")
	}
	if el.UserContent == nil {
		t.Error("UserContent should not be nil")
	}
}

func TestElicitationWrongEvent(t *testing.T) {
	input := `{
		"session_id": "s1",
		"cwd": "/proj",
		"hook_event_name": "Stop"
	}`
	hi, _ := ReadInput(strings.NewReader(input))
	if hi.Elicitation() != nil {
		t.Error("Elicitation() should return nil for wrong event type")
	}
}

func TestNewEventConstants(t *testing.T) {
	events := []string{
		EventPermissionDenied,
		EventStopFailure,
		EventTaskCreated,
		EventTaskCompleted,
		EventTeammateIdle,
		EventInstructionsLoaded,
		EventConfigChange,
		EventCwdChanged,
		EventFileChanged,
		EventWorktreeCreate,
		EventWorktreeRemove,
		EventPostCompact,
		EventElicitation,
		EventElicitationResult,
	}
	expected := []string{
		"PermissionDenied",
		"StopFailure",
		"TaskCreated",
		"TaskCompleted",
		"TeammateIdle",
		"InstructionsLoaded",
		"ConfigChange",
		"CwdChanged",
		"FileChanged",
		"WorktreeCreate",
		"WorktreeRemove",
		"PostCompact",
		"Elicitation",
		"ElicitationResult",
	}
	for i, got := range events {
		if got != expected[i] {
			t.Errorf("constant[%d] = %q, want %q", i, got, expected[i])
		}
	}
}
