package profiles

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/BrenanL/hitch/pkg/settings"
)

// settingsLocalPath returns the path to settings.local.json for the given project dir.
func settingsLocalPath(projectDir string) string {
	return filepath.Join(projectDir, ".claude", "settings.local.json")
}

// readSettingsLocal reads and parses the settings.local.json in the given project dir.
func readSettingsLocal(t *testing.T, projectDir string) *settings.Settings {
	t.Helper()
	s, err := settings.LoadScope(settings.ScopeLocal, projectDir)
	if err != nil {
		t.Fatalf("LoadScope(ScopeLocal): %v", err)
	}
	return s
}

// TestApplyProfileWritesExpectedKeys verifies ApplyProfile writes env and settings keys.
func TestApplyProfileWritesExpectedKeys(t *testing.T) {
	dir := t.TempDir()

	p := &Profile{
		Name:        "test-profile",
		Description: "test",
		Env: map[string]string{
			"ANTHROPIC_MODEL":          "claude-sonnet-4-6",
			"CLAUDE_CODE_EFFORT_LEVEL": "low",
		},
		Settings: map[string]any{
			"effortLevel": "low",
		},
	}

	written, err := ApplyProfile(p, dir)
	if err != nil {
		t.Fatalf("ApplyProfile: %v", err)
	}

	if len(written) == 0 {
		t.Error("ApplyProfile returned empty written keys")
	}

	s := readSettingsLocal(t, dir)

	if s.Env["ANTHROPIC_MODEL"] != "claude-sonnet-4-6" {
		t.Errorf("ANTHROPIC_MODEL = %q, want claude-sonnet-4-6", s.Env["ANTHROPIC_MODEL"])
	}
	if s.Env["CLAUDE_CODE_EFFORT_LEVEL"] != "low" {
		t.Errorf("CLAUDE_CODE_EFFORT_LEVEL = %q, want low", s.Env["CLAUDE_CODE_EFFORT_LEVEL"])
	}

	raw, ok := settings.GetRaw(s, "effortLevel")
	if !ok {
		t.Error("effortLevel not set in settings.local.json")
	}
	var effortLevel string
	if err := json.Unmarshal(raw, &effortLevel); err != nil {
		t.Fatalf("unmarshal effortLevel: %v", err)
	}
	if effortLevel != "low" {
		t.Errorf("effortLevel = %q, want low", effortLevel)
	}
}

// TestApplyProfileTracksWrittenKeys verifies the returned key list contains env: and settings: entries.
func TestApplyProfileTracksWrittenKeys(t *testing.T) {
	dir := t.TempDir()

	p := &Profile{
		Name:        "track-test",
		Description: "test",
		Env: map[string]string{
			"MY_ENV_VAR": "value1",
		},
		Settings: map[string]any{
			"mySettingKey": "value2",
		},
	}

	written, err := ApplyProfile(p, dir)
	if err != nil {
		t.Fatalf("ApplyProfile: %v", err)
	}

	hasEnvKey := false
	hasSettingsKey := false
	for _, k := range written {
		if k == "env:MY_ENV_VAR" {
			hasEnvKey = true
		}
		if k == "settings:mySettingKey" {
			hasSettingsKey = true
		}
	}
	if !hasEnvKey {
		t.Errorf("written keys missing env:MY_ENV_VAR, got %v", written)
	}
	if !hasSettingsKey {
		t.Errorf("written keys missing settings:mySettingKey, got %v", written)
	}
}

// TestResetProfileRemovesTrackedKeys verifies ResetProfile removes all keys written by ApplyProfile.
func TestResetProfileRemovesTrackedKeys(t *testing.T) {
	dir := t.TempDir()

	p := &Profile{
		Name:        "reset-test",
		Description: "test",
		Env: map[string]string{
			"RESET_ENV_KEY": "reset-value",
		},
		Settings: map[string]any{
			"resetSettingKey": "reset-setting",
		},
	}

	written, err := ApplyProfile(p, dir)
	if err != nil {
		t.Fatalf("ApplyProfile: %v", err)
	}

	s := readSettingsLocal(t, dir)
	if s.Env["RESET_ENV_KEY"] != "reset-value" {
		t.Fatalf("setup: RESET_ENV_KEY not written")
	}

	if err := ResetProfile(written, dir); err != nil {
		t.Fatalf("ResetProfile: %v", err)
	}

	s = readSettingsLocal(t, dir)
	if _, ok := s.Env["RESET_ENV_KEY"]; ok {
		t.Error("RESET_ENV_KEY still present after ResetProfile")
	}
	if _, ok := settings.GetRaw(s, "resetSettingKey"); ok {
		t.Error("resetSettingKey still present after ResetProfile")
	}
}

// TestSwitchProfileResetsOldAppliesNew verifies the switch pattern: reset old, apply new.
func TestSwitchProfileResetsOldAppliesNew(t *testing.T) {
	dir := t.TempDir()

	economy := &Profile{
		Name:        "economy",
		Description: "test economy",
		Env: map[string]string{
			"ANTHROPIC_MODEL":          "claude-sonnet-4-6",
			"CLAUDE_CODE_EFFORT_LEVEL": "low",
		},
	}

	performance := &Profile{
		Name:        "performance",
		Description: "test performance",
		Env: map[string]string{
			"ANTHROPIC_MODEL":          "claude-opus-4-6",
			"CLAUDE_CODE_EFFORT_LEVEL": "high",
		},
	}

	// Apply economy first.
	written, err := ApplyProfile(economy, dir)
	if err != nil {
		t.Fatalf("ApplyProfile(economy): %v", err)
	}

	s := readSettingsLocal(t, dir)
	if s.Env["ANTHROPIC_MODEL"] != "claude-sonnet-4-6" {
		t.Fatalf("after economy: ANTHROPIC_MODEL = %q", s.Env["ANTHROPIC_MODEL"])
	}

	// Switch: reset economy, apply performance.
	if err := ResetProfile(written, dir); err != nil {
		t.Fatalf("ResetProfile(economy): %v", err)
	}

	_, err = ApplyProfile(performance, dir)
	if err != nil {
		t.Fatalf("ApplyProfile(performance): %v", err)
	}

	s = readSettingsLocal(t, dir)
	if s.Env["ANTHROPIC_MODEL"] != "claude-opus-4-6" {
		t.Errorf("after switch: ANTHROPIC_MODEL = %q, want claude-opus-4-6", s.Env["ANTHROPIC_MODEL"])
	}
	if s.Env["CLAUDE_CODE_EFFORT_LEVEL"] != "high" {
		t.Errorf("after switch: CLAUDE_CODE_EFFORT_LEVEL = %q, want high", s.Env["CLAUDE_CODE_EFFORT_LEVEL"])
	}
}

// TestApplyProfilePreservesNonProfileKeys verifies keys not in the profile are untouched.
func TestApplyProfilePreservesNonProfileKeys(t *testing.T) {
	dir := t.TempDir()

	// Pre-write settings.local.json with ANTHROPIC_BASE_URL.
	initial := &settings.Settings{}
	// Use LoadScope to get an empty struct, then set our key.
	s, err := settings.LoadScope(settings.ScopeLocal, dir)
	if err != nil {
		t.Fatalf("LoadScope: %v", err)
	}
	_ = initial
	if err := settings.SetEnv(s, "ANTHROPIC_BASE_URL", "http://localhost:9800"); err != nil {
		t.Fatalf("SetEnv: %v", err)
	}
	if err := settings.Write(s, settings.ScopeLocal, dir); err != nil {
		t.Fatalf("Write: %v", err)
	}

	// Apply a profile that does NOT include ANTHROPIC_BASE_URL.
	p := &Profile{
		Name:        "no-url-profile",
		Description: "test",
		Env: map[string]string{
			"ANTHROPIC_MODEL": "claude-sonnet-4-6",
		},
	}

	written, err := ApplyProfile(p, dir)
	if err != nil {
		t.Fatalf("ApplyProfile: %v", err)
	}

	s = readSettingsLocal(t, dir)
	if s.Env["ANTHROPIC_BASE_URL"] != "http://localhost:9800" {
		t.Errorf("ANTHROPIC_BASE_URL was modified or removed, got %q", s.Env["ANTHROPIC_BASE_URL"])
	}

	// ResetProfile should also not remove ANTHROPIC_BASE_URL.
	if err := ResetProfile(written, dir); err != nil {
		t.Fatalf("ResetProfile: %v", err)
	}

	s = readSettingsLocal(t, dir)
	if s.Env["ANTHROPIC_BASE_URL"] != "http://localhost:9800" {
		t.Errorf("ANTHROPIC_BASE_URL removed by ResetProfile, got %q", s.Env["ANTHROPIC_BASE_URL"])
	}
}

// TestApplyProfileNullValueDeletesKey verifies that EnvDeletes causes the key to be removed.
func TestApplyProfileNullValueDeletesKey(t *testing.T) {
	dir := t.TempDir()

	// Pre-write the key we want to delete.
	s, err := settings.LoadScope(settings.ScopeLocal, dir)
	if err != nil {
		t.Fatalf("LoadScope: %v", err)
	}
	if err := settings.SetEnv(s, "SOME_KEY_TO_DELETE", "oldvalue"); err != nil {
		t.Fatalf("SetEnv: %v", err)
	}
	if err := settings.Write(s, settings.ScopeLocal, dir); err != nil {
		t.Fatalf("Write: %v", err)
	}

	// Apply a profile with EnvDeletes (the "explicit null" mechanism).
	p := &Profile{
		Name:        "delete-test",
		Description: "test",
		EnvDeletes:  []string{"SOME_KEY_TO_DELETE"},
	}

	_, err = ApplyProfile(p, dir)
	if err != nil {
		t.Fatalf("ApplyProfile: %v", err)
	}

	s = readSettingsLocal(t, dir)
	if _, ok := s.Env["SOME_KEY_TO_DELETE"]; ok {
		t.Error("SOME_KEY_TO_DELETE still present after applying profile with EnvDeletes")
	}
}

// TestCurrentProfileReturnsActiveName verifies CurrentProfile returns the name set by ApplyProfile.
func TestCurrentProfileReturnsActiveName(t *testing.T) {
	dir := t.TempDir()

	name, err := CurrentProfile(dir)
	if err != nil {
		t.Fatalf("CurrentProfile (no profile): %v", err)
	}
	if name != "" {
		t.Errorf("CurrentProfile with no active profile = %q, want empty", name)
	}

	p := &Profile{
		Name:        "active-profile-test",
		Description: "test",
		Env: map[string]string{
			"SOME_KEY": "somevalue",
		},
	}

	_, err = ApplyProfile(p, dir)
	if err != nil {
		t.Fatalf("ApplyProfile: %v", err)
	}

	name, err = CurrentProfile(dir)
	if err != nil {
		t.Fatalf("CurrentProfile after apply: %v", err)
	}
	if name != "active-profile-test" {
		t.Errorf("CurrentProfile = %q, want active-profile-test", name)
	}
}

// TestCurrentProfileClearedAfterReset verifies CurrentProfile returns empty after ResetProfile.
func TestCurrentProfileClearedAfterReset(t *testing.T) {
	dir := t.TempDir()

	p := &Profile{
		Name:        "to-reset",
		Description: "test",
		Env: map[string]string{
			"RESET_KEY": "val",
		},
	}

	written, err := ApplyProfile(p, dir)
	if err != nil {
		t.Fatalf("ApplyProfile: %v", err)
	}

	if err := ResetProfile(written, dir); err != nil {
		t.Fatalf("ResetProfile: %v", err)
	}

	name, err := CurrentProfile(dir)
	if err != nil {
		t.Fatalf("CurrentProfile after reset: %v", err)
	}
	if name != "" {
		t.Errorf("CurrentProfile after reset = %q, want empty", name)
	}
}

// TestApplyProfileEnvDeleteTrackedKey verifies that EnvDeletes entries appear in the tracked
// keys list as "env_delete:KEY".
func TestApplyProfileEnvDeleteTrackedKey(t *testing.T) {
	dir := t.TempDir()

	p := &Profile{
		Name:        "env-delete-tracking-test",
		Description: "test",
		EnvDeletes:  []string{"KEY_TO_TRACK"},
	}

	written, err := ApplyProfile(p, dir)
	if err != nil {
		t.Fatalf("ApplyProfile: %v", err)
	}

	found := false
	for _, k := range written {
		if k == "env_delete:KEY_TO_TRACK" {
			found = true
		}
	}
	if !found {
		t.Errorf("tracked keys missing env_delete:KEY_TO_TRACK, got %v", written)
	}
}

// TestApplyProfileSettingsDeleteTrackedKey verifies that nil Settings values appear in the
// tracked keys list as "settings_delete:KEY".
func TestApplyProfileSettingsDeleteTrackedKey(t *testing.T) {
	dir := t.TempDir()

	p := &Profile{
		Name:        "settings-delete-tracking-test",
		Description: "test",
		Settings: map[string]any{
			"keyToDelete": nil,
		},
	}

	written, err := ApplyProfile(p, dir)
	if err != nil {
		t.Fatalf("ApplyProfile: %v", err)
	}

	found := false
	for _, k := range written {
		if k == "settings_delete:keyToDelete" {
			found = true
		}
	}
	if !found {
		t.Errorf("tracked keys missing settings_delete:keyToDelete, got %v", written)
	}
}

// TestResetProfileEmptyTrackedKeys verifies ResetProfile is a no-op when called with an empty list.
func TestResetProfileEmptyTrackedKeys(t *testing.T) {
	dir := t.TempDir()

	if err := ResetProfile([]string{}, dir); err != nil {
		t.Errorf("ResetProfile with empty keys: unexpected error: %v", err)
	}
	if err := ResetProfile(nil, dir); err != nil {
		t.Errorf("ResetProfile with nil keys: unexpected error: %v", err)
	}
}

// TestApplyProfileHooksOnlyWritesActiveRecord verifies ApplyProfile with only Hooks (no env/settings)
// still writes the active-profile record.
func TestApplyProfileHooksOnlyWritesActiveRecord(t *testing.T) {
	dir := t.TempDir()

	p := &Profile{
		Name:        "hooks-only-profile",
		Description: "test",
		Hooks: map[string]any{
			"PreToolUse": []any{"some-hook"},
		},
	}

	written, err := ApplyProfile(p, dir)
	if err != nil {
		t.Fatalf("ApplyProfile: %v", err)
	}

	name, err := CurrentProfile(dir)
	if err != nil {
		t.Fatalf("CurrentProfile: %v", err)
	}
	if name != "hooks-only-profile" {
		t.Errorf("CurrentProfile = %q, want hooks-only-profile", name)
	}
	// written may be empty since no env/settings were set — that's valid
	_ = written
}

// TestApplyProfileSettingsLocalPathCreatedAutomatically verifies ApplyProfile creates
// the .claude directory if it does not already exist.
func TestApplyProfileSettingsLocalPathCreatedAutomatically(t *testing.T) {
	dir := t.TempDir()

	// Ensure .claude does not exist.
	claudeDir := filepath.Join(dir, ".claude")
	if _, err := os.Stat(claudeDir); !os.IsNotExist(err) {
		t.Skip(".claude dir already exists")
	}

	p := &Profile{
		Name:        "mkdir-test",
		Description: "test",
		Env: map[string]string{
			"MKDIR_KEY": "mkdir-value",
		},
	}

	_, err := ApplyProfile(p, dir)
	if err != nil {
		t.Fatalf("ApplyProfile: %v", err)
	}

	if _, err := os.Stat(settingsLocalPath(dir)); err != nil {
		t.Errorf("settings.local.json not created: %v", err)
	}
}
