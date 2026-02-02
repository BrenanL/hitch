// Package test contains CLI binary tests that exercise the ht binary as a black box.
// These tests build the binary and run it with a temp $HOME to avoid touching real config.
package test

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// testEnv holds the isolated environment for a CLI test.
type testEnv struct {
	t       *testing.T
	binary  string // path to built ht binary
	homeDir string // temp HOME dir
}

// setupTestEnv builds the ht binary and creates a temp HOME.
// The binary is cached across tests using t.TempDir at the top level.
func setupTestEnv(t *testing.T) *testEnv {
	t.Helper()

	// Build binary to a temp location
	binDir := t.TempDir()
	binary := filepath.Join(binDir, "ht")

	goCmd := os.Getenv("GO_CMD")
	if goCmd == "" {
		goCmd = "go"
	}

	cmd := exec.Command(goCmd, "build", "-o", binary, "./cmd/ht")
	cmd.Dir = filepath.Join("..")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("building ht binary: %v\n%s", err, out)
	}

	homeDir := t.TempDir()

	return &testEnv{
		t:       t,
		binary:  binary,
		homeDir: homeDir,
	}
}

// run executes an ht subcommand in the isolated environment.
func (e *testEnv) run(args ...string) (string, error) {
	e.t.Helper()
	cmd := exec.Command(e.binary, args...)
	cmd.Env = append(os.Environ(),
		"HOME="+e.homeDir,
		"XDG_CONFIG_HOME="+filepath.Join(e.homeDir, ".config"),
	)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// runWithStdin executes an ht subcommand with piped stdin.
func (e *testEnv) runWithStdin(stdin string, args ...string) (stdout string, exitCode int, err error) {
	e.t.Helper()
	cmd := exec.Command(e.binary, args...)
	cmd.Env = append(os.Environ(),
		"HOME="+e.homeDir,
		"XDG_CONFIG_HOME="+filepath.Join(e.homeDir, ".config"),
	)
	cmd.Stdin = strings.NewReader(stdin)
	out, err := cmd.CombinedOutput()
	code := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			code = exitErr.ExitCode()
			err = nil // non-zero exit is not an error for us
		}
	}
	return string(out), code, err
}

// settingsJSON reads the generated settings.json.
func (e *testEnv) settingsJSON() string {
	e.t.Helper()
	data, err := os.ReadFile(filepath.Join(e.homeDir, ".claude", "settings.json"))
	if err != nil {
		e.t.Fatalf("reading settings.json: %v", err)
	}
	return string(data)
}

// fileExists checks if a file exists relative to the temp HOME.
func (e *testEnv) fileExists(relPath string) bool {
	_, err := os.Stat(filepath.Join(e.homeDir, relPath))
	return err == nil
}

// --- Tests ---

func TestCLIVersion(t *testing.T) {
	env := setupTestEnv(t)
	out, err := env.run("--version")
	if err != nil {
		t.Fatalf("--version failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, "ht version") {
		t.Errorf("version output = %q, want 'ht version ...'", out)
	}
}

func TestCLIHelp(t *testing.T) {
	env := setupTestEnv(t)
	out, err := env.run("--help")
	if err != nil {
		t.Fatalf("--help failed: %v\n%s", err, out)
	}
	// Should mention key subcommands
	for _, sub := range []string{"init", "rule", "channel", "hook", "sync", "status"} {
		if !strings.Contains(out, sub) {
			t.Errorf("help output missing subcommand %q", sub)
		}
	}
}

func TestCLIInitGlobal(t *testing.T) {
	env := setupTestEnv(t)

	out, err := env.run("init", "--global")
	if err != nil {
		t.Fatalf("init --global failed: %v\n%s", err, out)
	}

	// Should have created directories and files
	if !env.fileExists(".hitch") {
		t.Error(".hitch directory not created")
	}
	if !env.fileExists(".hitch/state.db") {
		t.Error("state.db not created")
	}
	if !env.fileExists(".claude/settings.json") {
		t.Error("settings.json not created")
	}
	if !env.fileExists(".hitch/manifest.json") {
		t.Error("manifest.json not created")
	}

	// settings.json should contain system hooks
	settings := env.settingsJSON()
	if !strings.Contains(settings, "ht:system:session-start") {
		t.Error("settings.json missing session-start system hook")
	}
	if !strings.Contains(settings, "ht:system:user-prompt") {
		t.Error("settings.json missing user-prompt system hook")
	}
}

func TestCLIRuleAddAndList(t *testing.T) {
	env := setupTestEnv(t)

	// Init first
	out, err := env.run("init", "--global")
	if err != nil {
		t.Fatalf("init: %v\n%s", err, out)
	}

	// Add a rule (--global so it syncs to global settings.json)
	out, err = env.run("rule", "add", "--global", "on stop -> log")
	if err != nil {
		t.Fatalf("rule add: %v\n%s", err, out)
	}
	if !strings.Contains(out, "Rule") && !strings.Contains(out, "added") {
		t.Errorf("unexpected rule add output: %q", out)
	}

	// List rules
	out, err = env.run("rule", "list")
	if err != nil {
		t.Fatalf("rule list: %v\n%s", err, out)
	}
	if !strings.Contains(out, "on stop -> log") {
		t.Errorf("rule list should contain the rule DSL, got: %q", out)
	}
	// Should show enabled
	if !strings.Contains(out, "[+]") {
		t.Errorf("rule list should show enabled marker [+], got: %q", out)
	}
}

func TestCLIRuleAddInvalidDSL(t *testing.T) {
	env := setupTestEnv(t)
	env.run("init", "--global")

	out, err := env.run("rule", "add", "this is not valid dsl")
	if err == nil {
		t.Error("expected error for invalid DSL")
	}
	_ = out
}

func TestCLIRuleAddSyncsSettings(t *testing.T) {
	env := setupTestEnv(t)
	env.run("init", "--global")

	// Add a rule
	env.run("rule", "add", "--global", "on pre-bash -> deny if matches deny-list:destructive")

	// settings.json should now contain the rule
	settings := env.settingsJSON()
	if !strings.Contains(settings, "ht:rule-") {
		t.Error("settings.json should contain rule marker after add")
	}
}

func TestCLIRuleEnableDisable(t *testing.T) {
	env := setupTestEnv(t)
	env.run("init", "--global")

	// Add a rule and capture its ID
	out, _ := env.run("rule", "add", "--global", "on stop -> log")
	// Output is like "Rule abc123 added: on stop -> log"
	parts := strings.Fields(out)
	var ruleID string
	for i, p := range parts {
		if p == "Rule" && i+1 < len(parts) {
			ruleID = parts[i+1]
			break
		}
	}
	if ruleID == "" {
		t.Fatalf("could not parse rule ID from output: %q", out)
	}

	// Disable
	out, err := env.run("rule", "disable", ruleID)
	if err != nil {
		t.Fatalf("rule disable: %v\n%s", err, out)
	}

	// List should show disabled
	out, _ = env.run("rule", "list")
	if !strings.Contains(out, "[-]") {
		t.Errorf("disabled rule should show [-], got: %q", out)
	}

	// Enable
	out, err = env.run("rule", "enable", ruleID)
	if err != nil {
		t.Fatalf("rule enable: %v\n%s", err, out)
	}

	out, _ = env.run("rule", "list")
	if !strings.Contains(out, "[+]") {
		t.Errorf("re-enabled rule should show [+], got: %q", out)
	}
}

func TestCLIRuleRemove(t *testing.T) {
	env := setupTestEnv(t)
	env.run("init", "--global")

	out, _ := env.run("rule", "add", "--global", "on stop -> log")
	parts := strings.Fields(out)
	var ruleID string
	for i, p := range parts {
		if p == "Rule" && i+1 < len(parts) {
			ruleID = parts[i+1]
			break
		}
	}

	// Remove
	out, err := env.run("rule", "remove", ruleID)
	if err != nil {
		t.Fatalf("rule remove: %v\n%s", err, out)
	}

	// List should be empty
	out, _ = env.run("rule", "list")
	if strings.Contains(out, "on stop") {
		t.Error("rule should be gone after remove")
	}
}

// TestCLIHookExecDeny is the critical test: pipe JSON stdin to ht hook exec,
// verify stdout JSON and exit code. This is the exact contract Claude Code depends on.
func TestCLIHookExecDeny(t *testing.T) {
	env := setupTestEnv(t)
	env.run("init", "--global")

	// Add a deny rule
	out, _ := env.run("rule", "add", "--global", `on pre-bash -> deny "blocked" if command matches "rm -rf"`)
	ruleID := extractRuleID(t, out)

	// Simulate destructive command — should be blocked (exit 2)
	input := `{"session_id":"s1","hook_event_name":"PreToolUse","tool_name":"Bash","tool_input":{"command":"rm -rf /"}}`
	stdout, exitCode, err := env.runWithStdin(input, "hook", "exec", ruleID)
	if err != nil {
		t.Fatalf("hook exec failed: %v", err)
	}
	if exitCode != 2 {
		t.Errorf("destructive command: exit code = %d, want 2\nstdout: %s", exitCode, stdout)
	}

	// stdout should be valid JSON with deny decision
	var output map[string]any
	if err := json.Unmarshal([]byte(stdout), &output); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\nraw: %q", err, stdout)
	}
	if output["decision"] != "deny" {
		t.Errorf("decision = %v, want deny", output["decision"])
	}
}

// TestCLIHookExecAllow tests that safe commands pass through.
func TestCLIHookExecAllow(t *testing.T) {
	env := setupTestEnv(t)
	env.run("init", "--global")

	out, _ := env.run("rule", "add", "--global", `on pre-bash -> deny "blocked" if command matches "rm -rf"`)
	ruleID := extractRuleID(t, out)

	// Simulate safe command — should allow (exit 0)
	input := `{"session_id":"s1","hook_event_name":"PreToolUse","tool_name":"Bash","tool_input":{"command":"npm test"}}`
	stdout, exitCode, err := env.runWithStdin(input, "hook", "exec", ruleID)
	if err != nil {
		t.Fatalf("hook exec failed: %v", err)
	}
	if exitCode != 0 {
		t.Errorf("safe command: exit code = %d, want 0\nstdout: %s", exitCode, stdout)
	}

	// stdout should be valid JSON (allow = empty object)
	var output map[string]any
	if err := json.Unmarshal([]byte(stdout), &output); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\nraw: %q", err, stdout)
	}
}

// TestCLIHookExecUnknownRule tests that unknown rule IDs return allow (fail open).
func TestCLIHookExecUnknownRule(t *testing.T) {
	env := setupTestEnv(t)
	env.run("init", "--global")

	input := `{"session_id":"s1","hook_event_name":"PreToolUse","tool_name":"Bash","tool_input":{"command":"echo hi"}}`
	stdout, exitCode, err := env.runWithStdin(input, "hook", "exec", "nonexistent-rule")
	if err != nil {
		t.Fatalf("hook exec failed: %v", err)
	}
	if exitCode != 0 {
		t.Errorf("unknown rule: exit code = %d, want 0 (fail open)", exitCode)
	}
	_ = stdout
}

// TestCLIHookExecDisabledRule tests that disabled rules return allow.
func TestCLIHookExecDisabledRule(t *testing.T) {
	env := setupTestEnv(t)
	env.run("init", "--global")

	out, _ := env.run("rule", "add", "--global", `on pre-bash -> deny "blocked"`)
	ruleID := extractRuleID(t, out)

	// Disable the rule
	env.run("rule", "disable", ruleID)

	input := `{"session_id":"s1","hook_event_name":"PreToolUse","tool_name":"Bash","tool_input":{"command":"rm -rf /"}}`
	_, exitCode, err := env.runWithStdin(input, "hook", "exec", ruleID)
	if err != nil {
		t.Fatalf("hook exec failed: %v", err)
	}
	if exitCode != 0 {
		t.Errorf("disabled rule: exit code = %d, want 0 (allow)", exitCode)
	}
}

// TestCLIHookExecSystemHook tests the system hook path (session-start).
func TestCLIHookExecSystemHook(t *testing.T) {
	env := setupTestEnv(t)
	env.run("init", "--global")

	input := `{"session_id":"test-sess","hook_event_name":"SessionStart","cwd":"/tmp/test"}`
	stdout, exitCode, err := env.runWithStdin(input, "hook", "exec", "system:session-start")
	if err != nil {
		t.Fatalf("hook exec system: %v", err)
	}
	if exitCode != 0 {
		t.Errorf("system hook: exit code = %d, want 0\nstdout: %s", exitCode, stdout)
	}
}

// TestCLIHookExecInvalidStdin tests that invalid stdin is handled gracefully.
func TestCLIHookExecInvalidStdin(t *testing.T) {
	env := setupTestEnv(t)
	env.run("init", "--global")

	out, _ := env.run("rule", "add", "--global", "on stop -> log")
	ruleID := extractRuleID(t, out)

	// Send garbage stdin — should not crash
	_, exitCode, err := env.runWithStdin("not json at all", "hook", "exec", ruleID)
	if err != nil {
		t.Fatalf("hook exec with bad stdin: %v", err)
	}
	// Should not crash (exit code 0 = allow, or at least not a panic)
	if exitCode > 2 {
		t.Errorf("bad stdin: exit code = %d, want <= 2 (not a crash)", exitCode)
	}
}

// TestCLIHookExecEmptyStdin tests that empty stdin is handled gracefully.
func TestCLIHookExecEmptyStdin(t *testing.T) {
	env := setupTestEnv(t)
	env.run("init", "--global")

	out, _ := env.run("rule", "add", "--global", "on stop -> log")
	ruleID := extractRuleID(t, out)

	_, exitCode, err := env.runWithStdin("", "hook", "exec", ruleID)
	if err != nil {
		t.Fatalf("hook exec with empty stdin: %v", err)
	}
	if exitCode > 2 {
		t.Errorf("empty stdin: exit code = %d, want <= 2", exitCode)
	}
}

func TestCLIDenyListList(t *testing.T) {
	env := setupTestEnv(t)
	env.run("init", "--global")

	out, err := env.run("deny-list", "list")
	if err != nil {
		t.Fatalf("deny-list list: %v\n%s", err, out)
	}
	if !strings.Contains(out, "destructive") {
		t.Errorf("deny-list list should show 'destructive', got: %q", out)
	}
}

func TestCLIDenyListShow(t *testing.T) {
	env := setupTestEnv(t)
	env.run("init", "--global")

	out, err := env.run("deny-list", "show", "destructive")
	if err != nil {
		t.Fatalf("deny-list show: %v\n%s", err, out)
	}
	if !strings.Contains(out, "rm -rf /") {
		t.Errorf("deny-list show should include 'rm -rf /', got: %q", out)
	}
}

func TestCLISyncDryRun(t *testing.T) {
	env := setupTestEnv(t)
	env.run("init", "--global")
	env.run("rule", "add", "--global", "on stop -> log")

	out, err := env.run("sync", "--dry-run")
	if err != nil {
		t.Fatalf("sync --dry-run: %v\n%s", err, out)
	}
}

func TestCLISyncIdempotent(t *testing.T) {
	env := setupTestEnv(t)
	env.run("init", "--global")
	env.run("rule", "add", "--global", "on stop -> log")

	// Run sync twice
	env.run("sync")
	settings1 := env.settingsJSON()

	env.run("sync")
	settings2 := env.settingsJSON()

	// Should be identical (no duplicates)
	if settings1 != settings2 {
		t.Error("sync is not idempotent — settings.json changed on second run")
	}
}

func TestCLISettingsPreservesUserHooks(t *testing.T) {
	env := setupTestEnv(t)

	// Write a pre-existing settings.json with a user hook
	settingsDir := filepath.Join(env.homeDir, ".claude")
	os.MkdirAll(settingsDir, 0o755)
	userSettings := `{
  "hooks": {
    "Stop": [
      {
        "matcher": "",
        "hooks": [
          {"type": "command", "command": "echo my-custom-hook"}
        ]
      }
    ]
  }
}`
	os.WriteFile(filepath.Join(settingsDir, "settings.json"), []byte(userSettings), 0o644)

	// Init (which syncs system hooks)
	env.run("init", "--global")

	// User hook should still be there
	settings := env.settingsJSON()
	if !strings.Contains(settings, "my-custom-hook") {
		t.Error("user hook was destroyed by init")
	}
}

func TestCLIStatus(t *testing.T) {
	env := setupTestEnv(t)
	env.run("init", "--global")

	out, err := env.run("status")
	if err != nil {
		t.Fatalf("status: %v\n%s", err, out)
	}
	// Should not crash, output some info
	if len(out) == 0 {
		t.Error("status produced no output")
	}
}

func TestCLIMuteUnmute(t *testing.T) {
	env := setupTestEnv(t)
	env.run("init", "--global")

	out, err := env.run("mute", "30m")
	if err != nil {
		t.Fatalf("mute: %v\n%s", err, out)
	}

	out, err = env.run("unmute")
	if err != nil {
		t.Fatalf("unmute: %v\n%s", err, out)
	}
}

func TestCLIExport(t *testing.T) {
	env := setupTestEnv(t)
	env.run("init", "--global")
	env.run("rule", "add", "--global", "on stop -> log")

	out, err := env.run("export")
	if err != nil {
		t.Fatalf("export: %v\n%s", err, out)
	}
	if !strings.Contains(out, "on stop") {
		t.Errorf("export should contain rule DSL, got: %q", out)
	}
}

// TestCLIFullWorkflow exercises the complete user workflow end-to-end.
func TestCLIFullWorkflow(t *testing.T) {
	env := setupTestEnv(t)

	// 1. Init
	out, err := env.run("init", "--global")
	if err != nil {
		t.Fatalf("init: %v\n%s", err, out)
	}

	// 2. Add channel (ntfy — will fail on send but that's fine for structure)
	out, err = env.run("channel", "add", "ntfy", "test-topic")
	if err != nil {
		t.Fatalf("channel add: %v\n%s", err, out)
	}

	// 3. List channels
	out, _ = env.run("channel", "list")
	if !strings.Contains(out, "ntfy") {
		t.Errorf("channel list should show ntfy, got: %q", out)
	}

	// 4. Add rules
	env.run("rule", "add", "--global", "on pre-bash -> deny if matches deny-list:destructive")
	env.run("rule", "add", "--global", "on stop -> log")

	// 5. List rules
	out, _ = env.run("rule", "list")
	if !strings.Contains(out, "deny") {
		t.Errorf("rule list missing deny rule: %q", out)
	}
	if !strings.Contains(out, "log") {
		t.Errorf("rule list missing log rule: %q", out)
	}

	// 6. Verify settings.json has everything
	settings := env.settingsJSON()
	if !strings.Contains(settings, "ht:system:session-start") {
		t.Error("settings missing system hooks")
	}
	if !strings.Contains(settings, "ht:rule-") {
		t.Error("settings missing rule hooks")
	}

	// 7. Simulate hook — destructive command should be blocked
	out2, _ := env.run("rule", "list")
	// Extract the deny rule ID from the list output
	var denyRuleID string
	for _, line := range strings.Split(out2, "\n") {
		if strings.Contains(line, "deny") && strings.Contains(line, "[+]") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				denyRuleID = fields[1]
			}
		}
	}
	if denyRuleID == "" {
		t.Fatal("could not find deny rule ID in list output")
	}

	input := fmt.Sprintf(`{"session_id":"s1","hook_event_name":"PreToolUse","tool_name":"Bash","tool_input":{"command":"rm -rf /"}}`)
	stdout, exitCode, err := env.runWithStdin(input, "hook", "exec", denyRuleID)
	if err != nil {
		t.Fatalf("hook exec: %v", err)
	}
	if exitCode != 2 {
		t.Errorf("workflow deny: exit code = %d, want 2\nstdout: %s", exitCode, stdout)
	}

	// 8. Check logs
	out, _ = env.run("log")
	// After a hook exec, there should be events logged
	_ = out // log output depends on whether events are visible (may be empty if no session filter)
}

// --- Helpers ---

// extractRuleID parses the rule ID from "Rule abc123 added: ..." output.
func extractRuleID(t *testing.T, output string) string {
	t.Helper()
	for _, line := range strings.Split(output, "\n") {
		parts := strings.Fields(line)
		for i, p := range parts {
			if p == "Rule" && i+1 < len(parts) {
				return parts[i+1]
			}
		}
	}
	t.Fatalf("could not extract rule ID from output: %q", output)
	return ""
}
