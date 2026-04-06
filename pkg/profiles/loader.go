package profiles

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// userProfilesDir returns the path to the user's profile directory.
func userProfilesDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("finding home dir: %w", err)
	}
	return filepath.Join(home, ".hitch", "profiles"), nil
}

// LoadAll loads all profiles: built-ins plus user profiles from ~/.hitch/profiles/.
// User profiles with the same name as a built-in shadow the built-in.
func LoadAll() ([]Profile, error) {
	builtins, err := builtinProfiles()
	if err != nil {
		return nil, err
	}

	// Index built-ins by name.
	byName := make(map[string]Profile, len(builtins))
	for _, p := range builtins {
		byName[p.Name] = p
	}

	// Load user profiles; they shadow built-ins by name.
	dir, err := userProfilesDir()
	if err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(dir)
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("reading user profiles dir: %w", err)
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", e.Name(), err)
		}
		var p Profile
		if err := json.Unmarshal(data, &p); err != nil {
			return nil, fmt.Errorf("parsing %s: %w", e.Name(), err)
		}
		byName[p.Name] = p
	}

	// Rebuild ordered list: built-ins first (in order), then user-only profiles.
	builtinNames := make(map[string]bool, len(builtins))
	result := make([]Profile, 0, len(byName))
	for _, b := range builtins {
		builtinNames[b.Name] = true
		result = append(result, byName[b.Name])
	}
	for name, p := range byName {
		if !builtinNames[name] {
			result = append(result, p)
		}
	}
	return result, nil
}

// Load loads a specific profile by name.
// User profiles shadow built-ins.
func Load(name string) (*Profile, error) {
	// Check user profiles first (higher priority).
	dir, err := userProfilesDir()
	if err != nil {
		return nil, err
	}

	userPath := filepath.Join(dir, name+".json")
	data, err := os.ReadFile(userPath)
	if err == nil {
		var p Profile
		if err := json.Unmarshal(data, &p); err != nil {
			return nil, fmt.Errorf("parsing user profile %q: %w", name, err)
		}
		return &p, nil
	}
	if !os.IsNotExist(err) {
		return nil, fmt.Errorf("reading user profile %q: %w", name, err)
	}

	// Fall back to built-ins.
	builtins, err := builtinProfiles()
	if err != nil {
		return nil, err
	}
	for i := range builtins {
		if builtins[i].Name == name {
			return &builtins[i], nil
		}
	}

	return nil, fmt.Errorf("profile %q not found", name)
}

// Validate checks that a profile is structurally valid.
func Validate(p *Profile) error {
	if p == nil {
		return fmt.Errorf("profile is nil")
	}
	if p.Name == "" {
		return fmt.Errorf("profile name is required")
	}
	if p.Description == "" {
		return fmt.Errorf("profile %q: description is required", p.Name)
	}
	for k, v := range p.Env {
		if k == "" {
			return fmt.Errorf("profile %q: env key must not be empty", p.Name)
		}
		if v == "" {
			return fmt.Errorf("profile %q: env value for %q must not be empty", p.Name, k)
		}
	}
	if p.Extends != "" && p.Extends == p.Name {
		return fmt.Errorf("profile %q: extends cannot reference itself", p.Name)
	}
	return nil
}
