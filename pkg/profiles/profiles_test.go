package profiles

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// TestBuiltinDefaultProfileFields verifies the "default" built-in has expected exact field values.
func TestBuiltinDefaultProfileFields(t *testing.T) {
	origHome := os.Getenv("HOME")
	tmp := t.TempDir()
	os.Setenv("HOME", tmp)
	defer os.Setenv("HOME", origHome)

	p, err := Load("default")
	if err != nil {
		t.Fatalf("Load('default'): %v", err)
	}
	if p.Env["CLAUDE_CODE_EFFORT_LEVEL"] != "medium" {
		t.Errorf("default CLAUDE_CODE_EFFORT_LEVEL = %q, want medium", p.Env["CLAUDE_CODE_EFFORT_LEVEL"])
	}
	effortRaw, ok := p.Settings["effortLevel"]
	if !ok {
		t.Fatal("default settings.effortLevel not set")
	}
	if effortRaw != "medium" {
		t.Errorf("default settings.effortLevel = %v, want medium", effortRaw)
	}
}

// TestBuiltinResearchProfileFields verifies the "research" built-in has MAX_THINKING_TOKENS=16000
// and the expected settings keys.
func TestBuiltinResearchProfileFields(t *testing.T) {
	origHome := os.Getenv("HOME")
	tmp := t.TempDir()
	os.Setenv("HOME", tmp)
	defer os.Setenv("HOME", origHome)

	p, err := Load("research")
	if err != nil {
		t.Fatalf("Load('research'): %v", err)
	}
	if p.Env["MAX_THINKING_TOKENS"] != "16000" {
		t.Errorf("research MAX_THINKING_TOKENS = %q, want 16000", p.Env["MAX_THINKING_TOKENS"])
	}
	if p.Env["CLAUDE_CODE_EFFORT_LEVEL"] != "high" {
		t.Errorf("research CLAUDE_CODE_EFFORT_LEVEL = %q, want high", p.Env["CLAUDE_CODE_EFFORT_LEVEL"])
	}
	if v, ok := p.Settings["alwaysThinkingEnabled"]; !ok || v != true {
		t.Errorf("research settings.alwaysThinkingEnabled = %v (ok=%v), want true", v, ok)
	}
	if v, ok := p.Settings["showThinkingSummaries"]; !ok || v != true {
		t.Errorf("research settings.showThinkingSummaries = %v (ok=%v), want true", v, ok)
	}
}

// TestBuiltinConservativeProfileHasHooks verifies the "conservative" built-in declares a hooks block.
func TestBuiltinConservativeProfileHasHooks(t *testing.T) {
	origHome := os.Getenv("HOME")
	tmp := t.TempDir()
	os.Setenv("HOME", tmp)
	defer os.Setenv("HOME", origHome)

	p, err := Load("conservative")
	if err != nil {
		t.Fatalf("Load('conservative'): %v", err)
	}
	if len(p.Hooks) == 0 {
		t.Error("conservative profile: expected hooks, got empty map")
	}
}

// TestBuiltinMinimalProfileHasNoEnvOrSettings verifies the "minimal" built-in has no env or settings.
func TestBuiltinMinimalProfileHasNoEnvOrSettings(t *testing.T) {
	origHome := os.Getenv("HOME")
	tmp := t.TempDir()
	os.Setenv("HOME", tmp)
	defer os.Setenv("HOME", origHome)

	p, err := Load("minimal")
	if err != nil {
		t.Fatalf("Load('minimal'): %v", err)
	}
	if len(p.Env) != 0 {
		t.Errorf("minimal profile: expected no env, got %v", p.Env)
	}
	if len(p.Settings) != 0 {
		t.Errorf("minimal profile: expected no settings, got %v", p.Settings)
	}
}

// TestBuiltinTagsPresent verifies all 5 built-in profiles include the "builtin" tag.
func TestBuiltinTagsPresent(t *testing.T) {
	origHome := os.Getenv("HOME")
	tmp := t.TempDir()
	os.Setenv("HOME", tmp)
	defer os.Setenv("HOME", origHome)

	profiles, err := LoadAll()
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}
	for _, p := range profiles {
		hasBuiltin := false
		for _, tag := range p.Tags {
			if tag == "builtin" {
				hasBuiltin = true
				break
			}
		}
		if !hasBuiltin {
			t.Errorf("profile %q missing 'builtin' tag, tags=%v", p.Name, p.Tags)
		}
	}
}

// TestLoadAllUserOnlyProfileAppears verifies a user profile whose name doesn't match any built-in
// still appears in LoadAll results.
func TestLoadAllUserOnlyProfileAppears(t *testing.T) {
	tmp := t.TempDir()
	os.Setenv("HOME", tmp)
	defer os.Setenv("HOME", os.Getenv("HOME"))

	profilesDir := filepath.Join(tmp, ".hitch", "profiles")
	if err := os.MkdirAll(profilesDir, 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	userProfile := Profile{
		Name:        "custom-user-profile",
		Description: "A user-defined profile",
		Tags:        []string{"custom"},
	}
	data, err := json.Marshal(userProfile)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := os.WriteFile(filepath.Join(profilesDir, "custom-user-profile.json"), data, 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	profiles, err := LoadAll()
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}

	found := false
	for _, p := range profiles {
		if p.Name == "custom-user-profile" {
			found = true
			if p.Description != "A user-defined profile" {
				t.Errorf("description = %q, want 'A user-defined profile'", p.Description)
			}
		}
	}
	if !found {
		t.Error("user-only profile 'custom-user-profile' not found in LoadAll results")
	}
	// Total should be 5 built-ins + 1 user-only
	if len(profiles) != 6 {
		t.Errorf("len(profiles) = %d, want 6 (5 builtins + 1 user-only)", len(profiles))
	}
}

// TestValidateEmptyEnvKey verifies Validate rejects a profile with an empty env key.
func TestValidateEmptyEnvKey(t *testing.T) {
	p := &Profile{
		Name:        "bad-env-key",
		Description: "has empty env key",
		Env: map[string]string{
			"": "some-value",
		},
	}
	if err := Validate(p); err == nil {
		t.Error("Validate with empty env key: expected error, got nil")
	}
}

// TestValidateEmptyEnvValue verifies Validate rejects a profile with an empty env value.
func TestValidateEmptyEnvValue(t *testing.T) {
	p := &Profile{
		Name:        "bad-env-value",
		Description: "has empty env value",
		Env: map[string]string{
			"SOME_KEY": "",
		},
	}
	if err := Validate(p); err == nil {
		t.Error("Validate with empty env value: expected error, got nil")
	}
}

// TestLoadAllBuiltins verifies all 5 built-in profiles are loaded.
func TestLoadAllBuiltins(t *testing.T) {
	// Override user dir to a temp dir with no user profiles.
	origHome := os.Getenv("HOME")
	tmp := t.TempDir()
	os.Setenv("HOME", tmp)
	defer os.Setenv("HOME", origHome)

	profiles, err := LoadAll()
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}

	want := []string{"default", "conservative", "autonomous", "research", "minimal"}
	if len(profiles) != len(want) {
		t.Errorf("len(profiles) = %d, want %d", len(profiles), len(want))
	}

	byName := make(map[string]Profile, len(profiles))
	for _, p := range profiles {
		byName[p.Name] = p
	}

	for _, name := range want {
		if _, ok := byName[name]; !ok {
			t.Errorf("built-in profile %q not found", name)
		}
	}
}

// TestLoadAllBuiltinsHaveDescriptions verifies each built-in has a description.
func TestLoadAllBuiltinsHaveDescriptions(t *testing.T) {
	origHome := os.Getenv("HOME")
	tmp := t.TempDir()
	os.Setenv("HOME", tmp)
	defer os.Setenv("HOME", origHome)

	profiles, err := LoadAll()
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}
	for _, p := range profiles {
		if p.Description == "" {
			t.Errorf("profile %q has empty description", p.Name)
		}
	}
}

// TestUserProfileShadowsBuiltin verifies user profiles shadow built-ins by name.
func TestUserProfileShadowsBuiltin(t *testing.T) {
	tmp := t.TempDir()
	os.Setenv("HOME", tmp)
	defer os.Setenv("HOME", os.Getenv("HOME"))

	// Write a user profile with the name "default" to shadow the built-in.
	profilesDir := filepath.Join(tmp, ".hitch", "profiles")
	if err := os.MkdirAll(profilesDir, 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	userProfile := Profile{
		Name:        "default",
		Description: "User override of default",
		Tags:        []string{"user"},
	}
	data, err := json.Marshal(userProfile)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := os.WriteFile(filepath.Join(profilesDir, "default.json"), data, 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	profiles, err := LoadAll()
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}

	var found *Profile
	for i := range profiles {
		if profiles[i].Name == "default" {
			found = &profiles[i]
			break
		}
	}
	if found == nil {
		t.Fatal("profile 'default' not found")
	}
	if found.Description != "User override of default" {
		t.Errorf("description = %q, want user override", found.Description)
	}
}

// TestInvalidProfileJSONProducesError verifies that invalid JSON in a user profile returns an error.
func TestInvalidProfileJSONProducesError(t *testing.T) {
	tmp := t.TempDir()
	os.Setenv("HOME", tmp)
	defer os.Setenv("HOME", os.Getenv("HOME"))

	profilesDir := filepath.Join(tmp, ".hitch", "profiles")
	if err := os.MkdirAll(profilesDir, 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	// Write invalid JSON.
	if err := os.WriteFile(filepath.Join(profilesDir, "bad.json"), []byte("{not valid json}"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	_, err := LoadAll()
	if err == nil {
		t.Error("LoadAll with invalid JSON: expected error, got nil")
	}
}

// TestValidateValid verifies Validate accepts a valid profile.
func TestValidateValid(t *testing.T) {
	p := &Profile{
		Name:        "myprofile",
		Description: "A valid profile",
		Env: map[string]string{
			"CLAUDE_CODE_EFFORT_LEVEL": "high",
		},
	}
	if err := Validate(p); err != nil {
		t.Errorf("Validate valid profile: %v", err)
	}
}

// TestValidateMissingName verifies Validate catches a missing name.
func TestValidateMissingName(t *testing.T) {
	p := &Profile{Description: "no name"}
	if err := Validate(p); err == nil {
		t.Error("Validate with empty name: expected error, got nil")
	}
}

// TestValidateMissingDescription verifies Validate catches a missing description.
func TestValidateMissingDescription(t *testing.T) {
	p := &Profile{Name: "nodesc"}
	if err := Validate(p); err == nil {
		t.Error("Validate with empty description: expected error, got nil")
	}
}

// TestValidateNil verifies Validate catches nil input.
func TestValidateNil(t *testing.T) {
	if err := Validate(nil); err == nil {
		t.Error("Validate(nil): expected error, got nil")
	}
}

// TestValidateSelfExtends verifies Validate catches self-referential extends.
func TestValidateSelfExtends(t *testing.T) {
	p := &Profile{
		Name:        "loop",
		Description: "self-referential",
		Extends:     "loop",
	}
	if err := Validate(p); err == nil {
		t.Error("Validate with self-extends: expected error, got nil")
	}
}

// TestLoadByName verifies Load returns the correct profile by name.
func TestLoadByName(t *testing.T) {
	origHome := os.Getenv("HOME")
	tmp := t.TempDir()
	os.Setenv("HOME", tmp)
	defer os.Setenv("HOME", origHome)

	p, err := Load("default")
	if err != nil {
		t.Fatalf("Load('default'): %v", err)
	}
	if p.Name != "default" {
		t.Errorf("p.Name = %q, want 'default'", p.Name)
	}
	if p.Description == "" {
		t.Error("p.Description is empty")
	}
}

// TestLoadNotFound verifies Load returns an error for unknown profiles.
func TestLoadNotFound(t *testing.T) {
	origHome := os.Getenv("HOME")
	tmp := t.TempDir()
	os.Setenv("HOME", tmp)
	defer os.Setenv("HOME", origHome)

	_, err := Load("nonexistent-profile-xyz")
	if err == nil {
		t.Error("Load('nonexistent-profile-xyz'): expected error, got nil")
	}
}

// TestLoadUserShadowsBuiltin verifies Load returns user profile over built-in when names collide.
func TestLoadUserShadowsBuiltin(t *testing.T) {
	tmp := t.TempDir()
	os.Setenv("HOME", tmp)
	defer os.Setenv("HOME", os.Getenv("HOME"))

	profilesDir := filepath.Join(tmp, ".hitch", "profiles")
	if err := os.MkdirAll(profilesDir, 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	userProfile := Profile{
		Name:        "minimal",
		Description: "User minimal override",
	}
	data, err := json.Marshal(userProfile)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := os.WriteFile(filepath.Join(profilesDir, "minimal.json"), data, 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	p, err := Load("minimal")
	if err != nil {
		t.Fatalf("Load('minimal'): %v", err)
	}
	if p.Description != "User minimal override" {
		t.Errorf("description = %q, want user override", p.Description)
	}
}
