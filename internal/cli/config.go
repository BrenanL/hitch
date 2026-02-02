package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/BrenanL/hitch/internal/adapters"
	"github.com/BrenanL/hitch/internal/state"
)

// Paths holds the resolved directory and file paths.
type Paths struct {
	GlobalDir      string // ~/.hitch/
	GlobalDB       string // ~/.hitch/state.db
	GlobalManifest string // ~/.hitch/manifest.json
	GlobalSettings string // ~/.claude/settings.json
	GlobalSyncLock string // ~/.hitch/sync.lock

	ProjectDir      string // .hitch/
	ProjectManifest string // .hitch/manifest.json
	ProjectSettings string // .claude/settings.json
}

func resolvePaths() (*Paths, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("resolving home directory: %w", err)
	}

	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("resolving working directory: %w", err)
	}

	return &Paths{
		GlobalDir:      filepath.Join(home, ".hitch"),
		GlobalDB:       filepath.Join(home, ".hitch", "state.db"),
		GlobalManifest: filepath.Join(home, ".hitch", "manifest.json"),
		GlobalSettings: filepath.Join(home, ".claude", "settings.json"),
		GlobalSyncLock: filepath.Join(home, ".hitch", "sync.lock"),

		ProjectDir:      filepath.Join(cwd, ".hitch"),
		ProjectManifest: filepath.Join(cwd, ".hitch", "manifest.json"),
		ProjectSettings: filepath.Join(cwd, ".claude", "settings.json"),
	}, nil
}

func openDB() (*state.DB, *Paths, error) {
	paths, err := resolvePaths()
	if err != nil {
		return nil, nil, err
	}
	db, err := state.Open(paths.GlobalDB)
	if err != nil {
		return nil, nil, fmt.Errorf("opening database: %w", err)
	}
	return db, paths, nil
}

// resolveAdapter looks up a channel in the DB and creates an adapter.
func resolveAdapter(db *state.DB, name string) (adapters.Adapter, error) {
	ch, err := db.ChannelGet(name)
	if err != nil {
		return nil, err
	}
	if ch == nil {
		return nil, fmt.Errorf("channel %q not found", name)
	}

	// Parse config JSON into map
	var config map[string]string
	if err := json.Unmarshal([]byte(ch.Config), &config); err != nil {
		config = make(map[string]string)
	}

	return adapters.NewAdapter(ch.Adapter, config)
}
