package settings

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultBaseline_HasEnvVars(t *testing.T) {
	m := DefaultBaseline("")
	envRaw, ok := m["env"]
	if !ok {
		t.Fatal("expected 'env' key in baseline map")
	}
	env, ok := envRaw.(map[string]string)
	if !ok {
		t.Fatalf("env is %T, want map[string]string", envRaw)
	}
	wantKeys := []string{
		"CLAUDE_CODE_ENABLE_TELEMETRY",
		"CLAUDE_ENABLE_STREAM_WATCHDOG",
		"DISABLE_TELEMETRY",
		"DISABLE_ERROR_REPORTING",
		"CLAUDE_CODE_DEBUG_LOG_LEVEL",
	}
	for _, k := range wantKeys {
		if _, present := env[k]; !present {
			t.Errorf("missing env key %q", k)
		}
	}
	if len(env) != 5 {
		t.Errorf("expected exactly 5 env keys (no proxy URL), got %d", len(env))
	}
}

func TestDefaultBaseline_HasNonEnvKeys(t *testing.T) {
	m := DefaultBaseline("")

	// Verify effortLevel = "medium".
	effortRaw, ok := m["effortLevel"]
	if !ok {
		t.Fatal("expected 'effortLevel' key in baseline map")
	}
	effort, ok := effortRaw.(string)
	if !ok {
		t.Fatalf("effortLevel is %T, want string", effortRaw)
	}
	if effort != "medium" {
		t.Errorf("effortLevel = %q, want %q", effort, "medium")
	}

	// Verify showThinkingSummaries = true.
	thinkingRaw, ok := m["showThinkingSummaries"]
	if !ok {
		t.Fatal("expected 'showThinkingSummaries' key in baseline map")
	}
	thinking, ok := thinkingRaw.(bool)
	if !ok {
		t.Fatalf("showThinkingSummaries is %T, want bool", thinkingRaw)
	}
	if !thinking {
		t.Error("showThinkingSummaries should be true")
	}
}

func TestDefaultBaseline_ProxyURL(t *testing.T) {
	proxyURL := "http://localhost:9800"
	m := DefaultBaseline(proxyURL)
	envRaw, ok := m["env"]
	if !ok {
		t.Fatal("expected 'env' key in baseline map")
	}
	env, ok := envRaw.(map[string]string)
	if !ok {
		t.Fatalf("env is %T, want map[string]string", envRaw)
	}
	got, present := env["ANTHROPIC_BASE_URL"]
	if !present {
		t.Fatal("expected ANTHROPIC_BASE_URL in env when proxyURL is set")
	}
	if got != proxyURL {
		t.Errorf("ANTHROPIC_BASE_URL = %q, want %q", got, proxyURL)
	}
}

func TestDefaultBaseline_NoProxyURL(t *testing.T) {
	m := DefaultBaseline("")
	envRaw, ok := m["env"]
	if !ok {
		t.Fatal("expected 'env' key in baseline map")
	}
	env, ok := envRaw.(map[string]string)
	if !ok {
		t.Fatalf("env is %T, want map[string]string", envRaw)
	}
	if _, present := env["ANTHROPIC_BASE_URL"]; present {
		t.Error("ANTHROPIC_BASE_URL should not be present when proxyURL is empty")
	}
}

func TestLoadHitchDefaults_FileAbsent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	m, err := LoadHitchDefaults(path)
	if err != nil {
		t.Fatalf("LoadHitchDefaults returned unexpected error: %v", err)
	}
	if len(m) != 0 {
		t.Errorf("expected empty map for absent file, got %d keys", len(m))
	}
}

func TestLoadHitchDefaults_ValidToml(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	content := `# Hitch defaults
effortLevel = "high"
showThinkingSummaries = false

[env]
ANTHROPIC_BASE_URL = "http://localhost:9800"
DISABLE_TELEMETRY = "1"
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing config.toml: %v", err)
	}

	m, err := LoadHitchDefaults(path)
	if err != nil {
		t.Fatalf("LoadHitchDefaults: %v", err)
	}

	if got, ok := m["effortLevel"]; !ok || got != "high" {
		t.Errorf("effortLevel = %v (%v), want %q", got, ok, "high")
	}
	if got, ok := m["showThinkingSummaries"]; !ok || got != false {
		t.Errorf("showThinkingSummaries = %v (%v), want false", got, ok)
	}

	envRaw, ok := m["env"]
	if !ok {
		t.Fatal("expected 'env' key in result")
	}
	env, ok := envRaw.(map[string]string)
	if !ok {
		t.Fatalf("env is %T, want map[string]string", envRaw)
	}
	if env["ANTHROPIC_BASE_URL"] != "http://localhost:9800" {
		t.Errorf("ANTHROPIC_BASE_URL = %q, want %q", env["ANTHROPIC_BASE_URL"], "http://localhost:9800")
	}
	if env["DISABLE_TELEMETRY"] != "1" {
		t.Errorf("DISABLE_TELEMETRY = %q, want %q", env["DISABLE_TELEMETRY"], "1")
	}
}
