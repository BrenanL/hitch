package profiles

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/BrenanL/hitch/pkg/settings"
)

// activeProfileRecord is stored in .hitch/active-profile.json.
type activeProfileRecord struct {
	Name         string   `json:"name"`
	TrackedKeys  []string `json:"tracked_keys"`
}

func activeProfilePath(projectDir string) string {
	return filepath.Join(projectDir, ".hitch", "active-profile.json")
}

func readActiveProfile(projectDir string) (*activeProfileRecord, error) {
	data, err := os.ReadFile(activeProfilePath(projectDir))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading active-profile.json: %w", err)
	}
	var rec activeProfileRecord
	if err := json.Unmarshal(data, &rec); err != nil {
		return nil, fmt.Errorf("parsing active-profile.json: %w", err)
	}
	return &rec, nil
}

func writeActiveProfile(projectDir string, rec *activeProfileRecord) error {
	path := activeProfilePath(projectDir)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("creating .hitch dir: %w", err)
	}
	data, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling active-profile.json: %w", err)
	}
	return os.WriteFile(path, data, 0o644)
}

// ApplyProfile writes a profile's settings to settings.local.json.
// Returns the list of keys that were written (for tracking/rollback).
// Keys take the form "env:KEY" for env vars and "settings:KEY" for settings keys.
func ApplyProfile(p *Profile, projectDir string) ([]string, error) {
	s, err := settings.LoadScope(settings.ScopeLocal, projectDir)
	if err != nil {
		return nil, fmt.Errorf("loading settings.local.json: %w", err)
	}

	var written []string

	for k, v := range p.Env {
		if err := settings.SetEnv(s, k, v); err != nil {
			return nil, fmt.Errorf("setting env %q: %w", k, err)
		}
		written = append(written, "env:"+k)
	}

	for _, k := range p.EnvDeletes {
		settings.DeleteEnv(s, k)
		written = append(written, "env_delete:"+k)
	}

	for k, v := range p.Settings {
		if v == nil {
			settings.DeleteKey(s, k)
			written = append(written, "settings_delete:"+k)
		} else {
			if err := settings.SetKey(s, k, v); err != nil {
				return nil, fmt.Errorf("setting key %q: %w", k, err)
			}
			written = append(written, "settings:"+k)
		}
	}

	if err := settings.Write(s, settings.ScopeLocal, projectDir); err != nil {
		return nil, fmt.Errorf("writing settings.local.json: %w", err)
	}

	sort.Strings(written)

	rec := &activeProfileRecord{
		Name:        p.Name,
		TrackedKeys: written,
	}
	if err := writeActiveProfile(projectDir, rec); err != nil {
		return nil, fmt.Errorf("writing active-profile.json: %w", err)
	}

	return written, nil
}

// ResetProfile removes all keys previously written by a profile.
// Uses the tracked key list to know what to remove.
func ResetProfile(trackedKeys []string, projectDir string) error {
	s, err := settings.LoadScope(settings.ScopeLocal, projectDir)
	if err != nil {
		return fmt.Errorf("loading settings.local.json: %w", err)
	}

	for _, entry := range trackedKeys {
		if len(entry) > 4 && entry[:4] == "env:" {
			settings.DeleteEnv(s, entry[4:])
		} else if len(entry) > 9 && entry[:9] == "settings:" {
			settings.DeleteKey(s, entry[9:])
		}
	}

	if err := settings.Write(s, settings.ScopeLocal, projectDir); err != nil {
		return fmt.Errorf("writing settings.local.json: %w", err)
	}

	if err := os.Remove(activeProfilePath(projectDir)); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("removing active-profile.json: %w", err)
	}

	return nil
}

// CurrentProfile returns the name of the currently active profile, if any.
// Returns empty string and nil error if no profile is active.
func CurrentProfile(projectDir string) (string, error) {
	rec, err := readActiveProfile(projectDir)
	if err != nil {
		return "", err
	}
	if rec == nil {
		return "", nil
	}
	return rec.Name, nil
}
