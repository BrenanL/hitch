package settings

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// GlobalConfig represents the global config file (~/.claude/config.json).
// These keys must NOT appear in settings.json.
type GlobalConfig struct {
	Theme                      string `json:"theme,omitempty"`
	PreferredNotify            string `json:"preferredNotify,omitempty"`
	AutoUpdateStatus           string `json:"autoUpdateStatus,omitempty"`
	AutoConnectIde             *bool  `json:"autoConnectIde,omitempty"`
	AutoInstallIdeExtension    *bool  `json:"autoInstallIdeExtension,omitempty"`
	EditorMode                 string `json:"editorMode,omitempty"`
	ShowTurnDuration           *bool  `json:"showTurnDuration,omitempty"`
	TerminalProgressBarEnabled *bool  `json:"terminalProgressBarEnabled,omitempty"`
	TeammateMode               string `json:"teammateMode,omitempty"`
}

// globalConfigPath returns the path to the global config file.
func globalConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("getting home dir: %w", err)
	}
	return filepath.Join(home, ".claude", "config.json"), nil
}

// LoadGlobalConfig loads the global config from ~/.claude/config.json.
// Returns an empty GlobalConfig (not an error) if the file does not exist.
func LoadGlobalConfig() (*GlobalConfig, error) {
	path, err := globalConfigPath()
	if err != nil {
		return nil, err
	}
	return loadGlobalConfigFromPath(path)
}

// loadGlobalConfigFromPath reads a GlobalConfig from the given path.
// Returns an empty GlobalConfig if the file does not exist.
func loadGlobalConfigFromPath(path string) (*GlobalConfig, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return &GlobalConfig{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading global config: %w", err)
	}

	var gc GlobalConfig
	if err := json.Unmarshal(data, &gc); err != nil {
		return nil, fmt.Errorf("parsing global config: %w", err)
	}
	return &gc, nil
}

// WriteGlobalConfig atomically writes the global config to ~/.claude/config.json.
func WriteGlobalConfig(gc *GlobalConfig) error {
	path, err := globalConfigPath()
	if err != nil {
		return err
	}
	return writeGlobalConfigToPath(path, gc)
}

// writeGlobalConfigToPath atomically writes a GlobalConfig to the given path.
func writeGlobalConfigToPath(path string, gc *GlobalConfig) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}

	data, err := json.MarshalIndent(gc, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling global config: %w", err)
	}

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
