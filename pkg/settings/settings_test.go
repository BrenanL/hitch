package settings

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadScope_FileNotExist(t *testing.T) {
	dir := t.TempDir()
	s, err := LoadScope(ScopeProject, dir)
	if err != nil {
		t.Fatalf("LoadScope returned unexpected error: %v", err)
	}
	if s == nil {
		t.Fatal("LoadScope returned nil Settings")
	}
	if len(s.Hooks) != 0 {
		t.Errorf("expected empty Hooks, got %d entries", len(s.Hooks))
	}
	if len(s.Env) != 0 {
		t.Errorf("expected empty Env, got %d entries", len(s.Env))
	}
}

func TestLoadScope_ValidJSON(t *testing.T) {
	dir := t.TempDir()
	settingsDir := filepath.Join(dir, ".claude")
	if err := os.MkdirAll(settingsDir, 0o755); err != nil {
		t.Fatalf("creating .claude dir: %v", err)
	}
	content := `{
		"hooks": {
			"PreToolUse": [{"matcher": "Bash", "hooks": [{"type": "command", "command": "ht hook exec"}]}]
		},
		"env": {"FOO": "bar"},
		"model": "claude-opus-4-6"
	}`
	if err := os.WriteFile(filepath.Join(settingsDir, "settings.json"), []byte(content), 0o644); err != nil {
		t.Fatalf("writing settings: %v", err)
	}

	s, err := LoadScope(ScopeProject, dir)
	if err != nil {
		t.Fatalf("LoadScope: %v", err)
	}
	if len(s.Hooks) == 0 {
		t.Error("expected Hooks to be populated")
	}
	if s.Env["FOO"] != "bar" {
		t.Errorf("env FOO = %q, want %q", s.Env["FOO"], "bar")
	}
	raw, ok := GetRaw(s, "model")
	if !ok {
		t.Error("expected 'model' key in raw")
	}
	var model string
	if err := json.Unmarshal(raw, &model); err != nil || model != "claude-opus-4-6" {
		t.Errorf("model = %q, want %q", model, "claude-opus-4-6")
	}
}

func TestLoadScope_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	settingsDir := filepath.Join(dir, ".claude")
	if err := os.MkdirAll(settingsDir, 0o755); err != nil {
		t.Fatalf("creating .claude dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(settingsDir, "settings.json"), []byte("{not valid json"), 0o644); err != nil {
		t.Fatalf("writing settings: %v", err)
	}

	_, err := LoadScope(ScopeProject, dir)
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
	if !strings.Contains(err.Error(), "has invalid JSON, refusing to modify") {
		t.Errorf("error message = %q, want it to contain %q", err.Error(), "has invalid JSON, refusing to modify")
	}
}

func TestParseSettings_HooksRoundTrip(t *testing.T) {
	input := `{"hooks":{"PreToolUse":[{"matcher":"Bash","hooks":[{"type":"command","command":"ht hook exec"}]}]}}`

	s, err := ParseSettings([]byte(input))
	if err != nil {
		t.Fatalf("ParseSettings: %v", err)
	}

	out, err := MarshalSettings(s)
	if err != nil {
		t.Fatalf("MarshalSettings: %v", err)
	}

	s2, err := ParseSettings(out)
	if err != nil {
		t.Fatalf("ParseSettings (round-trip): %v", err)
	}

	groups, ok := s2.Hooks["PreToolUse"]
	if !ok || len(groups) == 0 {
		t.Fatal("hooks not present after round-trip")
	}
	if groups[0].Matcher != "Bash" {
		t.Errorf("matcher = %q, want %q", groups[0].Matcher, "Bash")
	}
	if len(groups[0].Hooks) == 0 {
		t.Fatal("hook entries missing after round-trip")
	}
	if groups[0].Hooks[0].Command != "ht hook exec" {
		t.Errorf("command = %q, want %q", groups[0].Hooks[0].Command, "ht hook exec")
	}
}

func TestParseSettings_UnknownFieldsPreserved(t *testing.T) {
	input := `{"zzz_unknown":"preserved_value","model":"test-model"}`

	s, err := ParseSettings([]byte(input))
	if err != nil {
		t.Fatalf("ParseSettings: %v", err)
	}

	out, err := MarshalSettings(s)
	if err != nil {
		t.Fatalf("MarshalSettings: %v", err)
	}

	var result map[string]json.RawMessage
	if err := json.Unmarshal(out, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}

	raw, ok := result["zzz_unknown"]
	if !ok {
		t.Fatal("zzz_unknown not present after round-trip")
	}
	var val string
	if err := json.Unmarshal(raw, &val); err != nil || val != "preserved_value" {
		t.Errorf("zzz_unknown = %q, want %q", val, "preserved_value")
	}
}

func TestWrite_AtomicWrite(t *testing.T) {
	dir := t.TempDir()

	s, err := ParseSettings([]byte(`{"model":"test-model"}`))
	if err != nil {
		t.Fatalf("ParseSettings: %v", err)
	}

	if err := Write(s, ScopeProject, dir); err != nil {
		t.Fatalf("Write: %v", err)
	}

	path := filepath.Join(dir, ".claude", "settings.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	var result map[string]json.RawMessage
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	raw, ok := result["model"]
	if !ok {
		t.Fatal("model key missing")
	}
	var model string
	if err := json.Unmarshal(raw, &model); err != nil || model != "test-model" {
		t.Errorf("model = %q, want %q", model, "test-model")
	}

	// Verify temp file was cleaned up
	if _, err := os.Stat(path + ".tmp"); !os.IsNotExist(err) {
		t.Error("temp file still exists after Write")
	}
}

func TestWrite_CreatesDirectory(t *testing.T) {
	dir := t.TempDir()
	// The .claude subdirectory does not exist yet.
	expectedDir := filepath.Join(dir, ".claude")
	if _, err := os.Stat(expectedDir); !os.IsNotExist(err) {
		t.Fatal(".claude dir should not exist yet")
	}

	s, err := ParseSettings([]byte(`{}`))
	if err != nil {
		t.Fatalf("ParseSettings: %v", err)
	}

	if err := Write(s, ScopeProject, dir); err != nil {
		t.Fatalf("Write: %v", err)
	}

	path := filepath.Join(expectedDir, "settings.json")
	if _, err := os.Stat(path); err != nil {
		t.Errorf("settings.json not created: %v", err)
	}
}

func TestSetKey_NewKey(t *testing.T) {
	s, err := ParseSettings([]byte(`{}`))
	if err != nil {
		t.Fatalf("ParseSettings: %v", err)
	}

	if err := SetKey(s, "effortLevel", "high"); err != nil {
		t.Fatalf("SetKey: %v", err)
	}

	out, err := MarshalSettings(s)
	if err != nil {
		t.Fatalf("MarshalSettings: %v", err)
	}

	var result map[string]json.RawMessage
	if err := json.Unmarshal(out, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	raw, ok := result["effortLevel"]
	if !ok {
		t.Fatal("effortLevel not present")
	}
	var level string
	if err := json.Unmarshal(raw, &level); err != nil || level != "high" {
		t.Errorf("effortLevel = %q, want %q", level, "high")
	}
}

func TestSetKey_NilDeletesKey(t *testing.T) {
	s, err := ParseSettings([]byte(`{"model":"test-model","effortLevel":"high"}`))
	if err != nil {
		t.Fatalf("ParseSettings: %v", err)
	}

	if err := SetKey(s, "model", nil); err != nil {
		t.Fatalf("SetKey nil: %v", err)
	}

	out, err := MarshalSettings(s)
	if err != nil {
		t.Fatalf("MarshalSettings: %v", err)
	}

	var result map[string]json.RawMessage
	if err := json.Unmarshal(out, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, ok := result["model"]; ok {
		t.Error("model key should have been deleted")
	}
	if _, ok := result["effortLevel"]; !ok {
		t.Error("effortLevel key should still be present")
	}
}

func TestDeleteKey(t *testing.T) {
	s, err := ParseSettings([]byte(`{"model":"test-model","effortLevel":"high"}`))
	if err != nil {
		t.Fatalf("ParseSettings: %v", err)
	}

	DeleteKey(s, "model")

	out, err := MarshalSettings(s)
	if err != nil {
		t.Fatalf("MarshalSettings: %v", err)
	}

	var result map[string]json.RawMessage
	if err := json.Unmarshal(out, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, ok := result["model"]; ok {
		t.Error("model key should have been deleted")
	}
	if _, ok := result["effortLevel"]; !ok {
		t.Error("effortLevel key should still be present")
	}
}

func TestSetEnv_SetsValue(t *testing.T) {
	s, err := ParseSettings([]byte(`{}`))
	if err != nil {
		t.Fatalf("ParseSettings: %v", err)
	}

	if err := SetEnv(s, "ANTHROPIC_BASE_URL", "http://localhost:9800"); err != nil {
		t.Fatalf("SetEnv: %v", err)
	}

	if s.Env["ANTHROPIC_BASE_URL"] != "http://localhost:9800" {
		t.Errorf("Env[ANTHROPIC_BASE_URL] = %q, want %q", s.Env["ANTHROPIC_BASE_URL"], "http://localhost:9800")
	}

	// Verify the value survives marshal round-trip.
	out, err := MarshalSettings(s)
	if err != nil {
		t.Fatalf("MarshalSettings: %v", err)
	}
	s2, err := ParseSettings(out)
	if err != nil {
		t.Fatalf("ParseSettings round-trip: %v", err)
	}
	if s2.Env["ANTHROPIC_BASE_URL"] != "http://localhost:9800" {
		t.Errorf("round-trip Env[ANTHROPIC_BASE_URL] = %q, want %q", s2.Env["ANTHROPIC_BASE_URL"], "http://localhost:9800")
	}
}

func TestDeleteEnv_RemovesValue(t *testing.T) {
	s, err := ParseSettings([]byte(`{"env":{"KEY1":"val1","KEY2":"val2"}}`))
	if err != nil {
		t.Fatalf("ParseSettings: %v", err)
	}

	DeleteEnv(s, "KEY1")

	if _, ok := s.Env["KEY1"]; ok {
		t.Error("KEY1 should have been deleted from Env map")
	}
	if s.Env["KEY2"] != "val2" {
		t.Errorf("KEY2 = %q, want %q", s.Env["KEY2"], "val2")
	}

	// Verify KEY1 is absent after marshal.
	out, err := MarshalSettings(s)
	if err != nil {
		t.Fatalf("MarshalSettings: %v", err)
	}
	s2, err := ParseSettings(out)
	if err != nil {
		t.Fatalf("ParseSettings round-trip: %v", err)
	}
	if _, ok := s2.Env["KEY1"]; ok {
		t.Error("KEY1 present after DeleteEnv round-trip")
	}
	if s2.Env["KEY2"] != "val2" {
		t.Errorf("round-trip KEY2 = %q, want %q", s2.Env["KEY2"], "val2")
	}
}
