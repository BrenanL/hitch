package envvars

import (
	"os"
	"sort"
	"strings"
	"testing"
)

// TestRegistry_Count verifies the registry has the expected number of vars.
// The reference table (tables-env-vars.md) has 148 unique variables,
// though the document header claims 128. We register all variables in the table.
func TestRegistry_Count(t *testing.T) {
	all := All()
	if len(all) != 148 {
		t.Errorf("registry count = %d, want 148", len(all))
	}
}

func TestRegistry_NoDuplicateNames(t *testing.T) {
	seen := make(map[string]bool)
	for _, v := range All() {
		if seen[v.Name] {
			t.Errorf("duplicate name: %s", v.Name)
		}
		seen[v.Name] = true
	}
}

func TestGet_KnownVar(t *testing.T) {
	v, ok := Get("ANTHROPIC_API_KEY")
	if !ok {
		t.Fatal("Get(ANTHROPIC_API_KEY) returned false")
	}
	if v.Name != "ANTHROPIC_API_KEY" {
		t.Errorf("Name = %q, want ANTHROPIC_API_KEY", v.Name)
	}
	if v.Category != "API Authentication" {
		t.Errorf("Category = %q, want API Authentication", v.Category)
	}
	if v.Description == "" {
		t.Error("Description is empty")
	}
}

func TestGet_UnknownVar(t *testing.T) {
	_, ok := Get("NONEXISTENT")
	if ok {
		t.Error("Get(NONEXISTENT) returned true, want false")
	}
}

func TestGetByCategory_APIAuth(t *testing.T) {
	vars := GetByCategory("API Authentication")
	if len(vars) == 0 {
		t.Fatal("GetByCategory(API Authentication) returned empty slice")
	}
	expectedNames := map[string]bool{
		"ANTHROPIC_API_KEY":   true,
		"ANTHROPIC_AUTH_TOKEN": true,
	}
	foundNames := make(map[string]bool)
	for _, v := range vars {
		if v.Category != "API Authentication" {
			t.Errorf("var %s has category %q, want API Authentication", v.Name, v.Category)
		}
		foundNames[v.Name] = true
	}
	for name := range expectedNames {
		if !foundNames[name] {
			t.Errorf("expected var %s not found in API Authentication category", name)
		}
	}
}

func TestCategories_NonEmpty(t *testing.T) {
	cats := Categories()
	if len(cats) == 0 {
		t.Fatal("Categories() returned empty slice")
	}
	// Verify it's sorted
	sorted := make([]string, len(cats))
	copy(sorted, cats)
	sort.Strings(sorted)
	for i := range cats {
		if cats[i] != sorted[i] {
			t.Errorf("Categories() not sorted: got %v", cats)
			break
		}
	}
	// Verify expected categories are present
	catSet := make(map[string]bool)
	for _, c := range cats {
		catSet[c] = true
	}
	expected := []string{
		"API Authentication",
		"API Configuration",
		"Model Configuration",
	}
	for _, e := range expected {
		if !catSet[e] {
			t.Errorf("category %q not found in Categories()", e)
		}
	}
}

func TestGetCurrent_SetVar(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "test-key-value")
	val, ok := GetCurrent("ANTHROPIC_API_KEY")
	if !ok {
		t.Fatal("GetCurrent returned false for set var")
	}
	if val != "test-key-value" {
		t.Errorf("GetCurrent = %q, want test-key-value", val)
	}
}

func TestGetCurrent_UnsetVar(t *testing.T) {
	os.Unsetenv("ANTHROPIC_API_KEY")
	_, ok := GetCurrent("ANTHROPIC_API_KEY")
	if ok {
		t.Error("GetCurrent returned true for unset var")
	}
}

func TestGetAllCurrent_OnlyRegistered(t *testing.T) {
	// Set a registered var and an unregistered one
	t.Setenv("ANTHROPIC_API_KEY", "test-value")
	t.Setenv("MY_CUSTOM_UNREGISTERED_VAR_XYZ", "custom")

	result := GetAllCurrent()

	// Must contain the registered var
	if _, ok := result["ANTHROPIC_API_KEY"]; !ok {
		t.Error("GetAllCurrent missing ANTHROPIC_API_KEY")
	}
	// Must NOT contain unregistered var
	if _, ok := result["MY_CUSTOM_UNREGISTERED_VAR_XYZ"]; ok {
		t.Error("GetAllCurrent includes unregistered var MY_CUSTOM_UNREGISTERED_VAR_XYZ")
	}
	// All returned vars must be in the registry
	for name := range result {
		if _, ok := Get(name); !ok {
			t.Errorf("GetAllCurrent returned unregistered var %s", name)
		}
	}
}

func TestValidate_RequiresNotMet(t *testing.T) {
	// Set CLAUDE_CODE_OAUTH_REFRESH_TOKEN without CLAUDE_CODE_OAUTH_SCOPES
	t.Setenv("CLAUDE_CODE_OAUTH_REFRESH_TOKEN", "some-token")
	os.Unsetenv("CLAUDE_CODE_OAUTH_SCOPES")

	issues := validateVars(map[string]string{
		"CLAUDE_CODE_OAUTH_REFRESH_TOKEN": "some-token",
	})

	found := false
	for _, issue := range issues {
		if issue.Var == "CLAUDE_CODE_OAUTH_REFRESH_TOKEN" && strings.Contains(issue.Message, "CLAUDE_CODE_OAUTH_SCOPES") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected warning about CLAUDE_CODE_OAUTH_SCOPES not being set, got: %v", issues)
	}
}

func TestValidate_DeprecatedVar(t *testing.T) {
	issues := validateVars(map[string]string{
		"ANTHROPIC_SMALL_FAST_MODEL": "claude-haiku-3",
	})

	found := false
	for _, issue := range issues {
		if issue.Var == "ANTHROPIC_SMALL_FAST_MODEL" && issue.Level == "warning" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected deprecation warning for ANTHROPIC_SMALL_FAST_MODEL, got: %v", issues)
	}
}

func TestValidate_ConflictingProviders(t *testing.T) {
	issues := validateVars(map[string]string{
		"CLAUDE_CODE_USE_BEDROCK": "1",
		"CLAUDE_CODE_USE_VERTEX":  "1",
	})

	found := false
	for _, issue := range issues {
		if issue.Level == "error" && strings.Contains(issue.Message, "conflicting") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected conflict error for multiple providers, got: %v", issues)
	}
}

func TestGenerateEnvBlock_RoundTrip(t *testing.T) {
	vars := map[string]string{
		"ANTHROPIC_API_KEY":  "sk-test",
		"ANTHROPIC_BASE_URL": "http://localhost:9800",
	}
	block := GenerateEnvBlock(vars)

	// Verify format: each line is "export KEY=VALUE\n"
	if !strings.Contains(block, "export ANTHROPIC_API_KEY=sk-test\n") {
		t.Errorf("block missing expected line, got:\n%s", block)
	}
	if !strings.Contains(block, "export ANTHROPIC_BASE_URL=http://localhost:9800\n") {
		t.Errorf("block missing expected line, got:\n%s", block)
	}
	// Verify all lines have export prefix
	lines := strings.Split(strings.TrimSpace(block), "\n")
	if len(lines) != 2 {
		t.Errorf("expected 2 lines, got %d", len(lines))
	}
	for _, line := range lines {
		if !strings.HasPrefix(line, "export ") {
			t.Errorf("line missing export prefix: %q", line)
		}
	}
}

func TestResolveEffective_OSWins(t *testing.T) {
	// Set OS env for a known var
	t.Setenv("ANTHROPIC_API_KEY", "os-value")

	settings := map[string]string{
		"ANTHROPIC_API_KEY":  "settings-value",
		"ANTHROPIC_BASE_URL": "http://settings.example.com",
	}

	result := ResolveEffective(settings)

	// OS value should win
	if result["ANTHROPIC_API_KEY"] != "os-value" {
		t.Errorf("ANTHROPIC_API_KEY = %q, want os-value (OS should win)", result["ANTHROPIC_API_KEY"])
	}
	// Settings value retained when no OS override
	os.Unsetenv("ANTHROPIC_BASE_URL")
	result2 := ResolveEffective(settings)
	if result2["ANTHROPIC_BASE_URL"] != "http://settings.example.com" {
		t.Errorf("ANTHROPIC_BASE_URL = %q, want http://settings.example.com", result2["ANTHROPIC_BASE_URL"])
	}
}
