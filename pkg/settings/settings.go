package settings

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Scope identifies which settings file a value came from.
type Scope int

const (
	ScopeUser    Scope = iota // ~/.claude/settings.json
	ScopeProject              // {projectDir}/.claude/settings.json
	ScopeLocal                // {projectDir}/.claude/settings.local.json
	ScopeManaged              // {projectDir}/.claude/settings.managed.json
)

func (s Scope) String() string {
	switch s {
	case ScopeUser:
		return "user"
	case ScopeProject:
		return "project"
	case ScopeLocal:
		return "local"
	case ScopeManaged:
		return "managed"
	default:
		return "unknown"
	}
}

// HookEntry is a single hook command entry in settings.json.
type HookEntry struct {
	Type    string `json:"type"`
	Command string `json:"command"`
	Async   bool   `json:"async,omitempty"`
}

// MatcherGroup groups hooks under a matcher pattern.
type MatcherGroup struct {
	Matcher string      `json:"matcher"`
	Hooks   []HookEntry `json:"hooks"`
}

// Settings holds one parsed settings.json file.
// Preserves all unknown fields for round-trip fidelity.
type Settings struct {
	Hooks map[string][]MatcherGroup `json:"hooks,omitempty"`
	Env   map[string]string         `json:"env,omitempty"`
	raw   map[string]json.RawMessage
}

// scopePath returns the file path for the given scope.
func scopePath(scope Scope, projectDir string) (string, error) {
	switch scope {
	case ScopeUser:
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("getting home dir: %w", err)
		}
		return filepath.Join(home, ".claude", "settings.json"), nil
	case ScopeProject:
		return filepath.Join(projectDir, ".claude", "settings.json"), nil
	case ScopeLocal:
		return filepath.Join(projectDir, ".claude", "settings.local.json"), nil
	case ScopeManaged:
		return filepath.Join(projectDir, ".claude", "settings.managed.json"), nil
	default:
		return "", fmt.Errorf("unknown scope: %d", scope)
	}
}

// LoadScope loads settings from a specific scope.
// Returns empty Settings (not nil) if file doesn't exist.
func LoadScope(scope Scope, projectDir string) (*Settings, error) {
	path, err := scopePath(scope, projectDir)
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return &Settings{
			Hooks: make(map[string][]MatcherGroup),
			Env:   make(map[string]string),
			raw:   make(map[string]json.RawMessage),
		}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading settings: %w", err)
	}

	return ParseSettings(data)
}

// LoadAll loads settings from all scopes (User, Project, Local, Managed in priority order).
// Returns empty Settings for missing files.
func LoadAll(projectDir string) ([]*Settings, error) {
	scopes := []Scope{ScopeUser, ScopeProject, ScopeLocal, ScopeManaged}
	result := make([]*Settings, 0, len(scopes))
	for _, scope := range scopes {
		s, err := LoadScope(scope, projectDir)
		if err != nil {
			return nil, fmt.Errorf("loading scope %s: %w", scope, err)
		}
		result = append(result, s)
	}
	return result, nil
}

// ParseSettings parses JSON into Settings.
// Returns error for invalid JSON.
func ParseSettings(data []byte) (*Settings, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("settings.json has invalid JSON, refusing to modify: %w", err)
	}

	s := &Settings{
		Hooks: make(map[string][]MatcherGroup),
		Env:   make(map[string]string),
		raw:   raw,
	}

	if hooksData, ok := raw["hooks"]; ok {
		if err := json.Unmarshal(hooksData, &s.Hooks); err != nil {
			return nil, fmt.Errorf("parsing hooks: %w", err)
		}
	}

	if envData, ok := raw["env"]; ok {
		if err := json.Unmarshal(envData, &s.Env); err != nil {
			return nil, fmt.Errorf("parsing env: %w", err)
		}
	}

	return s, nil
}

// Write atomically writes settings to the given scope path.
// Creates parent directory if needed.
func Write(s *Settings, scope Scope, projectDir string) error {
	path, err := scopePath(scope, projectDir)
	if err != nil {
		return err
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating settings directory: %w", err)
	}

	data, err := MarshalSettings(s)
	if err != nil {
		return err
	}

	// Atomic write: write to temp file in same directory, then rename.
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("writing temp file: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("renaming temp file: %w", err)
	}
	return nil
}

// MarshalSettings serializes Settings to JSON preserving unknown fields.
func MarshalSettings(s *Settings) ([]byte, error) {
	output := make(map[string]json.RawMessage)
	for k, v := range s.raw {
		if k != "hooks" && k != "env" {
			output[k] = v
		}
	}

	if len(s.Hooks) > 0 {
		hooksData, err := json.Marshal(s.Hooks)
		if err != nil {
			return nil, fmt.Errorf("marshaling hooks: %w", err)
		}
		output["hooks"] = hooksData
	}

	if len(s.Env) > 0 {
		envData, err := json.Marshal(s.Env)
		if err != nil {
			return nil, fmt.Errorf("marshaling env: %w", err)
		}
		output["env"] = envData
	}

	return json.MarshalIndent(output, "", "  ")
}

// SetKey sets a top-level key in the settings raw map.
// Pass nil to delete the key.
func SetKey(s *Settings, key string, value any) error {
	if value == nil {
		delete(s.raw, key)
		return nil
	}
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("marshaling value for key %q: %w", key, err)
	}
	s.raw[key] = data
	return nil
}

// DeleteKey removes a top-level key from the settings.
func DeleteKey(s *Settings, key string) {
	delete(s.raw, key)
}

// GetRaw returns the raw JSON for a key.
// Returns (nil, false) if the key is not present.
func GetRaw(s *Settings, key string) (json.RawMessage, bool) {
	v, ok := s.raw[key]
	return v, ok
}

// SetEnv sets an env var in the settings env block.
func SetEnv(s *Settings, key, value string) error {
	if s.Env == nil {
		s.Env = make(map[string]string)
	}
	s.Env[key] = value

	data, err := json.Marshal(s.Env)
	if err != nil {
		return fmt.Errorf("marshaling env: %w", err)
	}
	s.raw["env"] = data
	return nil
}

// DeleteEnv removes an env var from the settings env block.
func DeleteEnv(s *Settings, key string) {
	if s.Env == nil {
		return
	}
	delete(s.Env, key)

	if len(s.Env) == 0 {
		delete(s.raw, "env")
	} else {
		data, _ := json.Marshal(s.Env)
		s.raw["env"] = data
	}
}
