package integration

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BrenanL/hitch/pkg/envvars"
	"github.com/BrenanL/hitch/pkg/settings"
)

// writeSettingsFile writes raw JSON to the given directory as a settings file.
func writeSettingsFile(t *testing.T, dir, filename, content string) {
	t.Helper()
	claudeDir := filepath.Join(dir, ".claude")
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		t.Fatalf("creating .claude dir: %v", err)
	}
	path := filepath.Join(claudeDir, filename)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing %s: %v", filename, err)
	}
}

// writeUserSettingsFile writes the user-scope settings.json using the actual home dir path.
// Since LoadScope(ScopeUser) uses ~/.claude/settings.json, we manipulate HOME for isolation.
func isolateHome(t *testing.T) string {
	t.Helper()
	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)
	claudeDir := filepath.Join(fakeHome, ".claude")
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		t.Fatalf("creating fake .claude dir: %v", err)
	}
	return fakeHome
}

// TestSettingsAllScopesCompute loads all 4 scopes and verifies that Compute()
// returns effective values reflecting each scope's contribution, with higher
// scopes (project) overriding lower ones (user) for the same key.
func TestSettingsAllScopesCompute(t *testing.T) {
	fakeHome := isolateHome(t)
	projectDir := t.TempDir()

	// User scope (~/.claude/settings.json): set model and an env var
	userSettingsPath := filepath.Join(fakeHome, ".claude", "settings.json")
	if err := os.WriteFile(userSettingsPath, []byte(`{
		"model": "user-model",
		"env": {"API_ENV": "user-value", "USER_ONLY": "from-user"}
	}`), 0o644); err != nil {
		t.Fatalf("writing user settings: %v", err)
	}

	// Project scope: override model, override env var API_ENV
	writeSettingsFile(t, projectDir, "settings.json", `{
		"model": "project-model",
		"env": {"API_ENV": "project-value", "PROJECT_ONLY": "from-project"}
	}`)

	// Local scope: set a local-only key
	writeSettingsFile(t, projectDir, "settings.local.json", `{
		"env": {"LOCAL_ONLY": "from-local"}
	}`)

	// Managed scope: set a managed key
	writeSettingsFile(t, projectDir, "settings.managed.json", `{
		"model": "managed-model",
		"env": {"MANAGED_ONLY": "from-managed"}
	}`)

	all, err := settings.LoadAll(projectDir)
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}
	if len(all) != 4 {
		t.Fatalf("LoadAll returned %d scopes, want 4", len(all))
	}

	effective := settings.Compute(all)

	// model: managed scope has highest precedence
	modelRaw, modelScope, ok := effective.GetEffective("model")
	if !ok {
		t.Fatal("effective model key not found")
	}
	var model string
	if err := json.Unmarshal(modelRaw, &model); err != nil {
		t.Fatalf("unmarshal model: %v", err)
	}
	if model != "managed-model" {
		t.Errorf("effective model = %q, want managed-model (managed scope wins)", model)
	}
	if modelScope != settings.ScopeManaged {
		t.Errorf("model source = %v, want ScopeManaged", modelScope)
	}

	// env: higher scope wins for same key
	apiEnvVal, apiEnvScope, ok := effective.GetEnv("API_ENV")
	if !ok {
		t.Fatal("effective API_ENV not found")
	}
	// Managed scope has no API_ENV, local has no API_ENV, project has API_ENV = "project-value"
	if apiEnvVal != "project-value" {
		t.Errorf("effective API_ENV = %q, want project-value (project wins over user)", apiEnvVal)
	}
	if apiEnvScope != settings.ScopeProject {
		t.Errorf("API_ENV scope = %v, want ScopeProject", apiEnvScope)
	}

	// User-only env var present
	userOnlyVal, _, ok := effective.GetEnv("USER_ONLY")
	if !ok {
		t.Fatal("effective USER_ONLY not found")
	}
	if userOnlyVal != "from-user" {
		t.Errorf("effective USER_ONLY = %q, want from-user", userOnlyVal)
	}

	// Project-only env var present
	projectOnlyVal, _, ok := effective.GetEnv("PROJECT_ONLY")
	if !ok {
		t.Fatal("effective PROJECT_ONLY not found")
	}
	if projectOnlyVal != "from-project" {
		t.Errorf("effective PROJECT_ONLY = %q, want from-project", projectOnlyVal)
	}

	// Local-only env var present
	localOnlyVal, localOnlyScope, ok := effective.GetEnv("LOCAL_ONLY")
	if !ok {
		t.Fatal("effective LOCAL_ONLY not found")
	}
	if localOnlyVal != "from-local" {
		t.Errorf("effective LOCAL_ONLY = %q, want from-local", localOnlyVal)
	}
	if localOnlyScope != settings.ScopeLocal {
		t.Errorf("LOCAL_ONLY scope = %v, want ScopeLocal", localOnlyScope)
	}

	// Managed-only env var present
	managedOnlyVal, managedOnlyScope, ok := effective.GetEnv("MANAGED_ONLY")
	if !ok {
		t.Fatal("effective MANAGED_ONLY not found")
	}
	if managedOnlyVal != "from-managed" {
		t.Errorf("effective MANAGED_ONLY = %q, want from-managed", managedOnlyVal)
	}
	if managedOnlyScope != settings.ScopeManaged {
		t.Errorf("MANAGED_ONLY scope = %v, want ScopeManaged", managedOnlyScope)
	}
}

// TestEnvvarsValidateDeprecatedVar verifies that Validate() reports a warning when
// a deprecated environment variable is set in the OS environment.
func TestEnvvarsValidateDeprecatedVar(t *testing.T) {
	// ANTHROPIC_SMALL_FAST_MODEL is marked Deprecated with ReplacedBy = ANTHROPIC_DEFAULT_HAIKU_MODEL
	t.Setenv("ANTHROPIC_SMALL_FAST_MODEL", "claude-haiku-3")

	issues := envvars.Validate()

	found := false
	for _, issue := range issues {
		if issue.Var == "ANTHROPIC_SMALL_FAST_MODEL" && issue.Level == "warning" {
			found = true
			if !strings.Contains(issue.Message, "deprecated") {
				t.Errorf("deprecation message = %q, want message containing 'deprecated'", issue.Message)
			}
			if !strings.Contains(issue.Message, "ANTHROPIC_DEFAULT_HAIKU_MODEL") {
				t.Errorf("deprecation message = %q, want message containing replacement var name", issue.Message)
			}
			break
		}
	}
	if !found {
		t.Errorf("expected deprecation warning for ANTHROPIC_SMALL_FAST_MODEL, issues: %v", issues)
	}
}

// TestSettingsRoundTrip verifies that writing settings, loading them, modifying a key,
// writing again, and reloading produces the correct final state.
func TestSettingsRoundTrip(t *testing.T) {
	projectDir := t.TempDir()

	// Step 1: write initial settings
	initial, err := settings.ParseSettings([]byte(`{}`))
	if err != nil {
		t.Fatalf("ParseSettings (initial): %v", err)
	}
	if err := settings.SetKey(initial, "model", "initial-model"); err != nil {
		t.Fatalf("SetKey: %v", err)
	}
	if err := settings.SetEnv(initial, "FOO", "bar"); err != nil {
		t.Fatalf("SetEnv: %v", err)
	}

	if err := settings.Write(initial, settings.ScopeProject, projectDir); err != nil {
		t.Fatalf("Write (initial): %v", err)
	}

	// Step 2: load and verify
	loaded, err := settings.LoadScope(settings.ScopeProject, projectDir)
	if err != nil {
		t.Fatalf("LoadScope (after write): %v", err)
	}
	modelRaw, ok := settings.GetRaw(loaded, "model")
	if !ok {
		t.Fatal("model key missing after load")
	}
	var model string
	if err := json.Unmarshal(modelRaw, &model); err != nil {
		t.Fatalf("unmarshal model: %v", err)
	}
	if model != "initial-model" {
		t.Errorf("loaded model = %q, want initial-model", model)
	}
	if loaded.Env["FOO"] != "bar" {
		t.Errorf("loaded env FOO = %q, want bar", loaded.Env["FOO"])
	}

	// Step 3: modify and write again
	if err := settings.SetKey(loaded, "model", "modified-model"); err != nil {
		t.Fatalf("SetKey (modify): %v", err)
	}
	if err := settings.SetEnv(loaded, "NEW_KEY", "new-value"); err != nil {
		t.Fatalf("SetEnv (modify): %v", err)
	}

	if err := settings.Write(loaded, settings.ScopeProject, projectDir); err != nil {
		t.Fatalf("Write (modified): %v", err)
	}

	// Step 4: reload and verify final state
	final, err := settings.LoadScope(settings.ScopeProject, projectDir)
	if err != nil {
		t.Fatalf("LoadScope (final): %v", err)
	}
	finalModelRaw, ok := settings.GetRaw(final, "model")
	if !ok {
		t.Fatal("model key missing in final load")
	}
	var finalModel string
	if err := json.Unmarshal(finalModelRaw, &finalModel); err != nil {
		t.Fatalf("unmarshal final model: %v", err)
	}
	if finalModel != "modified-model" {
		t.Errorf("final model = %q, want modified-model", finalModel)
	}
	if final.Env["FOO"] != "bar" {
		t.Errorf("final env FOO = %q, want bar (should be preserved)", final.Env["FOO"])
	}
	if final.Env["NEW_KEY"] != "new-value" {
		t.Errorf("final env NEW_KEY = %q, want new-value", final.Env["NEW_KEY"])
	}
}

// TestEnvvarsGetAllCurrentIncludesAPIKey verifies that GetAllCurrent() includes
// ANTHROPIC_API_KEY when it is set in the OS environment.
func TestEnvvarsGetAllCurrentIncludesAPIKey(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "sk-integration-test-key")

	current := envvars.GetAllCurrent()

	val, ok := current["ANTHROPIC_API_KEY"]
	if !ok {
		t.Fatal("GetAllCurrent did not include ANTHROPIC_API_KEY")
	}
	if val != "sk-integration-test-key" {
		t.Errorf("ANTHROPIC_API_KEY = %q, want sk-integration-test-key", val)
	}
}

// TestSettingsValidateManagedOnlyInProject verifies that Validate() reports an error
// when a managed-only key is present in the project scope settings.
func TestSettingsValidateManagedOnlyInProject(t *testing.T) {
	// "forceLoginMethod" is a managed-only key per the schema.
	// Place it in a project-scope settings object.
	s, err := settings.ParseSettings([]byte(`{"forceLoginMethod": "claudeai"}`))
	if err != nil {
		t.Fatalf("ParseSettings: %v", err)
	}

	issues := settings.Validate(s, settings.ScopeProject)

	found := false
	for _, issue := range issues {
		if issue.Key == "forceLoginMethod" && issue.Level == "error" {
			found = true
			if !strings.Contains(issue.Message, "managed-only") {
				t.Errorf("issue message = %q, want message containing 'managed-only'", issue.Message)
			}
			break
		}
	}
	if !found {
		t.Errorf("expected error for managed-only key forceLoginMethod in project scope, issues: %v", issues)
	}

	// Sanity check: the same key in managed scope should NOT produce an error
	issuesManaged := settings.Validate(s, settings.ScopeManaged)
	for _, issue := range issuesManaged {
		if issue.Key == "forceLoginMethod" && issue.Level == "error" && strings.Contains(issue.Message, "managed-only") {
			t.Errorf("unexpected managed-only error when in managed scope: %v", issue)
		}
	}
}
