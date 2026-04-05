package profiles

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

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
