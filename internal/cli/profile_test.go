package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BrenanL/hitch/pkg/profiles"
)

// captureStdout redirects os.Stdout to a pipe and returns the captured output
// after calling fn. This is needed because profile commands use fmt.Println
// which writes to os.Stdout directly.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stdout = w

	fn()

	w.Close()
	os.Stdout = old

	buf := make([]byte, 64*1024)
	n, _ := r.Read(buf)
	r.Close()
	return string(buf[:n])
}

// TestProfileListShowsBuiltins verifies profile list outputs the built-in profile names.
func TestProfileListShowsBuiltins(t *testing.T) {
	origHome := os.Getenv("HOME")
	tmp := t.TempDir()
	os.Setenv("HOME", tmp)
	defer os.Setenv("HOME", origHome)

	out := captureStdout(t, func() {
		cmd := newProfileListCmd()
		cmd.SetArgs([]string{})
		if err := cmd.Execute(); err != nil {
			t.Errorf("profile list: %v", err)
		}
	})

	for _, name := range []string{"default", "conservative", "autonomous", "research", "minimal"} {
		if !strings.Contains(out, name) {
			t.Errorf("profile list: expected %q in output, got:\n%s", name, out)
		}
	}
}

// TestProfileListActiveMarker verifies the active profile is marked with *.
func TestProfileListActiveMarker(t *testing.T) {
	origHome := os.Getenv("HOME")
	tmp := t.TempDir()
	os.Setenv("HOME", tmp)
	defer os.Setenv("HOME", origHome)

	// Apply the "default" profile to tmp (home dir = global scope).
	p, err := profiles.Load("default")
	if err != nil {
		t.Fatalf("Load('default'): %v", err)
	}
	if _, err := profiles.ApplyProfile(p, tmp); err != nil {
		t.Fatalf("ApplyProfile: %v", err)
	}

	out := captureStdout(t, func() {
		cmd := newProfileListCmd()
		cmd.SetArgs([]string{})
		if err := cmd.Execute(); err != nil {
			t.Errorf("profile list: %v", err)
		}
	})

	if !strings.Contains(out, "* default") {
		t.Errorf("expected '* default' in output, got:\n%s", out)
	}
}

// TestProfileListJSON verifies --json flag outputs a valid JSON array.
func TestProfileListJSON(t *testing.T) {
	origHome := os.Getenv("HOME")
	tmp := t.TempDir()
	os.Setenv("HOME", tmp)
	defer os.Setenv("HOME", origHome)

	out := captureStdout(t, func() {
		cmd := newProfileListCmd()
		cmd.SetArgs([]string{"--json"})
		if err := cmd.Execute(); err != nil {
			t.Errorf("profile list --json: %v", err)
		}
	})

	var items []map[string]interface{}
	if err := json.Unmarshal([]byte(out), &items); err != nil {
		t.Fatalf("JSON parse error: %v\noutput: %s", err, out)
	}

	if len(items) == 0 {
		t.Error("expected at least one profile in JSON output")
	}

	for _, item := range items {
		if _, ok := item["name"]; !ok {
			t.Errorf("JSON item missing 'name': %v", item)
		}
		if _, ok := item["source"]; !ok {
			t.Errorf("JSON item missing 'source': %v", item)
		}
		if _, ok := item["active"]; !ok {
			t.Errorf("JSON item missing 'active': %v", item)
		}
	}
}

// TestProfileShowBuiltin verifies "profile show" prints profile details.
func TestProfileShowBuiltin(t *testing.T) {
	origHome := os.Getenv("HOME")
	tmp := t.TempDir()
	os.Setenv("HOME", tmp)
	defer os.Setenv("HOME", origHome)

	out := captureStdout(t, func() {
		cmd := newProfileShowCmd()
		cmd.SetArgs([]string{"default"})
		if err := cmd.Execute(); err != nil {
			t.Errorf("profile show default: %v", err)
		}
	})

	if !strings.Contains(out, "# Source: builtin (embedded)") {
		t.Errorf("expected source annotation in output, got:\n%s", out)
	}
	if !strings.Contains(out, `"name"`) {
		t.Errorf("expected JSON output with 'name' field, got:\n%s", out)
	}
	if !strings.Contains(out, "default") {
		t.Errorf("expected 'default' in output, got:\n%s", out)
	}
}

// TestProfileShowNotFound verifies an error is returned for unknown profiles.
func TestProfileShowNotFound(t *testing.T) {
	origHome := os.Getenv("HOME")
	tmp := t.TempDir()
	os.Setenv("HOME", tmp)
	defer os.Setenv("HOME", origHome)

	cmd := newProfileShowCmd()
	cmd.SetArgs([]string{"nonexistent-profile-xyz"})
	if err := cmd.Execute(); err == nil {
		t.Error("expected error for nonexistent profile, got nil")
	}
}

// TestProfileSwitchAppliesProfile verifies "profile switch" writes settings.local.json.
func TestProfileSwitchAppliesProfile(t *testing.T) {
	origHome := os.Getenv("HOME")
	tmp := t.TempDir()
	os.Setenv("HOME", tmp)
	defer os.Setenv("HOME", origHome)

	origCwd, _ := os.Getwd()
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("Chdir: %v", err)
	}
	defer os.Chdir(origCwd)

	out := captureStdout(t, func() {
		cmd := newProfileSwitchCmd()
		cmd.SetArgs([]string{"--project", "default"})
		if err := cmd.Execute(); err != nil {
			t.Errorf("profile switch --project default: %v", err)
		}
	})

	if !strings.Contains(out, "Applied profile") {
		t.Errorf("expected 'Applied profile' in output, got: %s", out)
	}

	settingsPath := filepath.Join(tmp, ".claude", "settings.local.json")
	if _, err := os.Stat(settingsPath); err != nil {
		t.Errorf("settings.local.json not created: %v", err)
	}
}

// TestProfileSwitchReset verifies --reset removes profile keys.
func TestProfileSwitchReset(t *testing.T) {
	origHome := os.Getenv("HOME")
	tmp := t.TempDir()
	os.Setenv("HOME", tmp)
	defer os.Setenv("HOME", origHome)

	origCwd, _ := os.Getwd()
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("Chdir: %v", err)
	}
	defer os.Chdir(origCwd)

	// Apply a profile first.
	captureStdout(t, func() {
		cmd := newProfileSwitchCmd()
		cmd.SetArgs([]string{"--project", "default"})
		_ = cmd.Execute()
	})

	// Verify active.
	name, err := profiles.CurrentProfile(tmp)
	if err != nil {
		t.Fatalf("CurrentProfile: %v", err)
	}
	if name != "default" {
		t.Fatalf("expected active profile 'default', got %q", name)
	}

	out := captureStdout(t, func() {
		cmd := newProfileSwitchCmd()
		cmd.SetArgs([]string{"--project", "--reset"})
		if err := cmd.Execute(); err != nil {
			t.Errorf("profile switch --reset: %v", err)
		}
	})

	if !strings.Contains(out, "Reset profile") {
		t.Errorf("expected 'Reset profile' in output, got: %s", out)
	}

	// Active profile should be cleared.
	name, err = profiles.CurrentProfile(tmp)
	if err != nil {
		t.Fatalf("CurrentProfile after reset: %v", err)
	}
	if name != "" {
		t.Errorf("expected no active profile after reset, got %q", name)
	}
}

// TestProfileCurrentNoActiveProfile verifies "profile current" prints "none" when no profile active.
func TestProfileCurrentNoActiveProfile(t *testing.T) {
	origHome := os.Getenv("HOME")
	tmp := t.TempDir()
	os.Setenv("HOME", tmp)
	defer os.Setenv("HOME", origHome)

	out := captureStdout(t, func() {
		cmd := newProfileCurrentCmd()
		cmd.SetArgs([]string{})
		if err := cmd.Execute(); err != nil {
			t.Errorf("profile current: %v", err)
		}
	})

	if !strings.Contains(out, "none") {
		t.Errorf("expected 'none' in output, got: %s", out)
	}
}

// TestProfileCurrentShowsActiveName verifies "profile current" shows the active profile name.
func TestProfileCurrentShowsActiveName(t *testing.T) {
	origHome := os.Getenv("HOME")
	tmp := t.TempDir()
	os.Setenv("HOME", tmp)
	defer os.Setenv("HOME", origHome)

	p, err := profiles.Load("default")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if _, err := profiles.ApplyProfile(p, tmp); err != nil {
		t.Fatalf("ApplyProfile: %v", err)
	}

	out := captureStdout(t, func() {
		cmd := newProfileCurrentCmd()
		cmd.SetArgs([]string{})
		if err := cmd.Execute(); err != nil {
			t.Errorf("profile current: %v", err)
		}
	})

	if !strings.Contains(out, "default") {
		t.Errorf("expected 'default' in output, got: %s", out)
	}
}

// TestProfileCreateWritesFile verifies "profile create" creates a JSON file.
func TestProfileCreateWritesFile(t *testing.T) {
	origHome := os.Getenv("HOME")
	tmp := t.TempDir()
	os.Setenv("HOME", tmp)
	defer os.Setenv("HOME", origHome)

	captureStdout(t, func() {
		cmd := newProfileCreateCmd()
		cmd.SetArgs([]string{"myprofile",
			"--description", "My custom profile",
			"--env", "CLAUDE_CODE_EFFORT_LEVEL=low",
		})
		if err := cmd.Execute(); err != nil {
			t.Errorf("profile create: %v", err)
		}
	})

	profilePath := filepath.Join(tmp, ".hitch", "profiles", "myprofile.json")
	data, err := os.ReadFile(profilePath)
	if err != nil {
		t.Fatalf("profile file not created: %v", err)
	}

	var p profiles.Profile
	if err := json.Unmarshal(data, &p); err != nil {
		t.Fatalf("invalid JSON in created profile: %v", err)
	}

	if p.Name != "myprofile" {
		t.Errorf("name = %q, want 'myprofile'", p.Name)
	}
	if p.Description != "My custom profile" {
		t.Errorf("description = %q, want 'My custom profile'", p.Description)
	}
	if p.Env["CLAUDE_CODE_EFFORT_LEVEL"] != "low" {
		t.Errorf("env CLAUDE_CODE_EFFORT_LEVEL = %q, want 'low'", p.Env["CLAUDE_CODE_EFFORT_LEVEL"])
	}
}

// TestProfileDeleteUserProfile verifies "profile delete" removes a user profile file.
func TestProfileDeleteUserProfile(t *testing.T) {
	origHome := os.Getenv("HOME")
	tmp := t.TempDir()
	os.Setenv("HOME", tmp)
	defer os.Setenv("HOME", origHome)

	profilesDir := filepath.Join(tmp, ".hitch", "profiles")
	if err := os.MkdirAll(profilesDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	p := profiles.Profile{Name: "todelete", Description: "Delete me"}
	data, _ := json.Marshal(p)
	profilePath := filepath.Join(profilesDir, "todelete.json")
	if err := os.WriteFile(profilePath, data, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	captureStdout(t, func() {
		cmd := newProfileDeleteCmd()
		cmd.SetArgs([]string{"todelete"})
		if err := cmd.Execute(); err != nil {
			t.Errorf("profile delete: %v", err)
		}
	})

	if _, err := os.Stat(profilePath); !os.IsNotExist(err) {
		t.Error("profile file still exists after delete")
	}
}

// TestProfileDeleteBuiltinRejected verifies "profile delete" refuses to delete built-ins.
func TestProfileDeleteBuiltinRejected(t *testing.T) {
	origHome := os.Getenv("HOME")
	tmp := t.TempDir()
	os.Setenv("HOME", tmp)
	defer os.Setenv("HOME", origHome)

	cmd := newProfileDeleteCmd()
	cmd.SetArgs([]string{"default"})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error when deleting built-in profile, got nil")
	}
}

// TestIsBuiltinProfile verifies the isBuiltinProfile helper detects built-in tag.
func TestIsBuiltinProfile(t *testing.T) {
	builtin := profiles.Profile{
		Name: "builtin-test",
		Tags: []string{"builtin"},
	}
	if !isBuiltinProfile(builtin) {
		t.Error("expected isBuiltinProfile to return true for profile with 'builtin' tag")
	}

	user := profiles.Profile{
		Name: "user-test",
		Tags: []string{"user"},
	}
	if isBuiltinProfile(user) {
		t.Error("expected isBuiltinProfile to return false for user profile")
	}

	noTags := profiles.Profile{Name: "notags"}
	if isBuiltinProfile(noTags) {
		t.Error("expected isBuiltinProfile to return false for profile with no tags")
	}
}
