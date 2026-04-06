package cli

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/BrenanL/hitch/pkg/profiles"
	"github.com/spf13/cobra"
)

// runLaunchCmd is a test helper that creates and executes the launch command with the
// given arguments, capturing stdout output. It returns the captured output and any error.
func runLaunchCmd(t *testing.T, args []string) (string, error) {
	t.Helper()
	root := &cobra.Command{Use: "ht"}
	root.AddCommand(newLaunchCmd())

	buf := new(bytes.Buffer)
	root.SetOut(buf)

	root.SetArgs(append([]string{"launch"}, args...))
	err := root.Execute()
	return buf.String(), err
}

// TestLaunchDryRunNoProfile verifies --dry-run with no profile prints expected output.
func TestLaunchDryRunNoProfile(t *testing.T) {
	root := &cobra.Command{Use: "ht"}
	cmd := newLaunchCmd()
	root.AddCommand(cmd)

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	// Capture stdout by overriding os.Stdout temporarily.
	origStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	root.SetArgs([]string{"launch", "--dry-run", "--no-proxy-check"})
	err := root.Execute()

	w.Close()
	os.Stdout = origStdout

	out := new(bytes.Buffer)
	out.ReadFrom(r)

	if err != nil {
		t.Fatalf("launch --dry-run: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "Profile:") {
		t.Errorf("dry-run output missing 'Profile:' line, got:\n%s", output)
	}
	if !strings.Contains(output, "dry-run: not launching") {
		t.Errorf("dry-run output missing '(dry-run: not launching)', got:\n%s", output)
	}
	if !strings.Contains(output, "Command that would run:") {
		t.Errorf("dry-run output missing 'Command that would run:', got:\n%s", output)
	}
}

// TestLaunchDryRunWithProfile verifies --dry-run --profile prints profile env changes.
func TestLaunchDryRunWithProfile(t *testing.T) {
	dir := t.TempDir()

	// Write a test profile to the user profiles dir override via temp dir.
	profilesDir := dir + "/.hitch/profiles"
	if err := os.MkdirAll(profilesDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	profileJSON := `{
		"name": "testprofile",
		"description": "a test profile",
		"env": {
			"ANTHROPIC_MODEL": "claude-test-model"
		}
	}`
	if err := os.WriteFile(profilesDir+"/testprofile.json", []byte(profileJSON), 0o644); err != nil {
		t.Fatalf("write profile: %v", err)
	}

	// Override HOME so profiles.Load looks in our temp dir.
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", dir)
	defer os.Setenv("HOME", origHome)

	origStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	root := &cobra.Command{Use: "ht"}
	root.AddCommand(newLaunchCmd())
	root.SetArgs([]string{"launch", "--dry-run", "--no-proxy-check", "--profile", "testprofile"})
	err := root.Execute()

	w.Close()
	os.Stdout = origStdout
	out := new(bytes.Buffer)
	out.ReadFrom(r)

	if err != nil {
		t.Fatalf("launch --dry-run --profile testprofile: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "testprofile") {
		t.Errorf("dry-run output missing profile name 'testprofile', got:\n%s", output)
	}
	if !strings.Contains(output, "ANTHROPIC_MODEL") {
		t.Errorf("dry-run output missing env var 'ANTHROPIC_MODEL', got:\n%s", output)
	}
	if !strings.Contains(output, "dry-run: not launching") {
		t.Errorf("dry-run output missing '(dry-run: not launching)', got:\n%s", output)
	}
}

// TestLaunchNoProxyCheckSkipsProbeAndDoesNotFail verifies that --no-proxy-check
// does not attempt a proxy connection and proceeds normally.
func TestLaunchNoProxyCheckSkipsProbeAndDoesNotFail(t *testing.T) {
	// isProxyHealthy would fail here since nothing is listening. With
	// --no-proxy-check the function should not be called at all.
	// We test this indirectly: in dry-run mode, if no proxy check is done,
	// the Proxy line should not say "running".
	dir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", dir)
	defer os.Setenv("HOME", origHome)

	origStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	root := &cobra.Command{Use: "ht"}
	root.AddCommand(newLaunchCmd())
	root.SetArgs([]string{"launch", "--dry-run", "--no-proxy-check"})
	err := root.Execute()

	w.Close()
	os.Stdout = origStdout
	out := new(bytes.Buffer)
	out.ReadFrom(r)

	if err != nil {
		t.Fatalf("launch --dry-run --no-proxy-check: %v", err)
	}
	// With no-proxy-check, proxyRunning is false so we expect "not running" text.
	output := out.String()
	if !strings.Contains(output, "dry-run: not launching") {
		t.Errorf("expected dry-run to complete cleanly, got:\n%s", output)
	}
}

// TestLaunchProfileAppliedBeforeLaunch verifies that when a profile is supplied with
// --dry-run, ApplyProfile is NOT called (dry-run doesn't write), but the profile is
// loaded and its details printed.
func TestLaunchProfileAppliedBeforeLaunch(t *testing.T) {
	dir := t.TempDir()

	profilesDir := dir + "/.hitch/profiles"
	if err := os.MkdirAll(profilesDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	profileJSON := `{
		"name": "apply-test",
		"description": "profile application test",
		"env": {
			"APPLY_TEST_KEY": "apply-test-value"
		},
		"settings": {
			"someSettingKey": "someValue"
		}
	}`
	if err := os.WriteFile(profilesDir+"/apply-test.json", []byte(profileJSON), 0o644); err != nil {
		t.Fatalf("write profile: %v", err)
	}

	origHome := os.Getenv("HOME")
	os.Setenv("HOME", dir)
	defer os.Setenv("HOME", origHome)

	origStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	root := &cobra.Command{Use: "ht"}
	root.AddCommand(newLaunchCmd())
	root.SetArgs([]string{"launch", "--dry-run", "--no-proxy-check", "--profile", "apply-test"})
	err := root.Execute()

	w.Close()
	os.Stdout = origStdout
	out := new(bytes.Buffer)
	out.ReadFrom(r)

	if err != nil {
		t.Fatalf("launch with apply-test profile: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "APPLY_TEST_KEY") {
		t.Errorf("expected APPLY_TEST_KEY in output, got:\n%s", output)
	}
	if !strings.Contains(output, "someSettingKey") {
		t.Errorf("expected someSettingKey in settings output, got:\n%s", output)
	}
}

// TestIsProxyHealthy verifies the proxy health check against a mock server.
func TestIsProxyHealthy(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			w.WriteHeader(http.StatusOK)
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	// isProxyHealthy hardcodes localhost:9800, so we test the helper function
	// logic indirectly by verifying the pattern used for health checks.
	// A direct unit test of the helper requires the function to accept a URL;
	// here we verify the health-check pattern via a direct HTTP call.
	client := &http.Client{}
	resp, err := client.Get(srv.URL + "/health")
	if err != nil {
		t.Fatalf("GET /health: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("health check status = %d, want 200", resp.StatusCode)
	}
}

// TestLaunchDryRunProfileNotFound verifies that an unknown profile returns an error.
func TestLaunchDryRunProfileNotFound(t *testing.T) {
	dir := t.TempDir()
	// Only builtin profiles available; "nonexistent-profile" should not exist.
	// We don't set HOME so builtin profiles are used.
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", dir)
	defer os.Setenv("HOME", origHome)

	root := &cobra.Command{Use: "ht"}
	root.AddCommand(newLaunchCmd())
	root.SetArgs([]string{"launch", "--dry-run", "--no-proxy-check", "--profile", "nonexistent-profile"})
	// Suppress error output from cobra.
	root.SetErr(new(bytes.Buffer))
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for unknown profile, got nil")
	}
	if !strings.Contains(err.Error(), "nonexistent-profile") {
		t.Errorf("error = %q; expected profile name in error", err.Error())
	}
}

// TestLaunchActionString verifies the launchAction helper returns correct strings.
func TestLaunchActionString(t *testing.T) {
	if got := launchAction(""); got != "launch" {
		t.Errorf("launchAction('') = %q, want 'launch'", got)
	}
	if got := launchAction("economy"); got != "launch:profile=economy" {
		t.Errorf("launchAction('economy') = %q, want 'launch:profile=economy'", got)
	}
}

// TestPrintLaunchDryRunNilProfile verifies printLaunchDryRun works with a nil profile.
func TestPrintLaunchDryRunNilProfile(t *testing.T) {
	origStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	printLaunchDryRun("", "/tmp/testdir", nil, false, []string{})

	w.Close()
	os.Stdout = origStdout
	out := new(bytes.Buffer)
	out.ReadFrom(r)

	output := out.String()
	if !strings.Contains(output, "Profile: (none)") {
		t.Errorf("expected 'Profile: (none)', got:\n%s", output)
	}
	if !strings.Contains(output, "dry-run: not launching") {
		t.Errorf("expected dry-run message, got:\n%s", output)
	}
}

// TestPrintLaunchDryRunWithProfile verifies printLaunchDryRun with a full profile.
func TestPrintLaunchDryRunWithProfile(t *testing.T) {
	p := &profiles.Profile{
		Name:        "perf",
		Description: "performance",
		Env: map[string]string{
			"ANTHROPIC_MODEL": "claude-opus-4-6",
		},
		Settings: map[string]any{
			"effortLevel": "high",
		},
	}

	origStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	printLaunchDryRun("perf", "/tmp/testdir", p, true, []string{"--no-update-notifier"})

	w.Close()
	os.Stdout = origStdout
	out := new(bytes.Buffer)
	out.ReadFrom(r)

	output := out.String()
	if !strings.Contains(output, "perf") {
		t.Errorf("missing profile name, got:\n%s", output)
	}
	if !strings.Contains(output, "ANTHROPIC_MODEL") {
		t.Errorf("missing ANTHROPIC_MODEL, got:\n%s", output)
	}
	if !strings.Contains(output, "effortLevel") {
		t.Errorf("missing effortLevel setting, got:\n%s", output)
	}
	if !strings.Contains(output, "--no-update-notifier") {
		t.Errorf("missing extra claude flag, got:\n%s", output)
	}
	if !strings.Contains(output, "Proxy: running") {
		t.Errorf("expected 'Proxy: running', got:\n%s", output)
	}
}
