package generator

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BrenanL/hitch/pkg/settings"
)

// Settings is an alias for settings.Settings.
type Settings = settings.Settings

// ReadSettings reads and parses a settings.json file.
func ReadSettings(path string) (*Settings, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return settings.ParseSettings([]byte(`{}`))
	}
	if err != nil {
		return nil, fmt.Errorf("reading settings: %w", err)
	}

	return settings.ParseSettings(data)
}

// ParseSettings parses settings JSON data.
func ParseSettings(data []byte) (*Settings, error) {
	return settings.ParseSettings(data)
}

// WriteSettings writes settings back to disk, preserving non-hook fields.
func WriteSettings(path string, s *Settings) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating settings directory: %w", err)
	}

	data, err := settings.MarshalSettings(s)
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0o644)
}

// MarshalSettings serializes settings to JSON.
func MarshalSettings(s *Settings) ([]byte, error) {
	return settings.MarshalSettings(s)
}

// MergeHooks performs the sync algorithm: removes old hitch entries, adds new ones.
func MergeHooks(s *Settings, manifest *Manifest, entries []*HookEntryInfo) {
	// Step 1: Remove old hitch entries (identified by markers from manifest)
	markers := manifest.AllMarkers()
	removeMarkedEntries(s, markers)

	// Step 2: Also remove entries matching any new markers (in case of re-sync)
	for _, e := range entries {
		removeMarkedEntries(s, []string{e.Marker})
	}

	// Step 3: Add new entries
	for _, e := range entries {
		addEntry(s, e)
	}

	// Step 4: Prune empty matcher groups and event types
	pruneEmpty(s)
}

// removeMarkedEntries removes hooks whose command contains any of the given markers.
func removeMarkedEntries(s *Settings, markers []string) {
	if len(markers) == 0 {
		return
	}

	for event, groups := range s.Hooks {
		var newGroups []MatcherGroup
		for _, group := range groups {
			var newHooks []HookEntry
			for _, hook := range group.Hooks {
				owned := false
				for _, marker := range markers {
					if strings.Contains(hook.Command, marker) {
						owned = true
						break
					}
				}
				if !owned {
					newHooks = append(newHooks, hook)
				}
			}
			group.Hooks = newHooks
			newGroups = append(newGroups, group)
		}
		s.Hooks[event] = newGroups
	}
}

// addEntry adds a hook entry to the correct event and matcher group.
func addEntry(s *Settings, e *HookEntryInfo) {
	groups := s.Hooks[e.Event]

	// Find existing matcher group
	for i, group := range groups {
		if group.Matcher == e.Matcher {
			groups[i].Hooks = append(groups[i].Hooks, e.Entry)
			s.Hooks[e.Event] = groups
			return
		}
	}

	// Create new matcher group
	s.Hooks[e.Event] = append(groups, MatcherGroup{
		Matcher: e.Matcher,
		Hooks:   []HookEntry{e.Entry},
	})
}

// pruneEmpty removes empty matcher groups and event types.
func pruneEmpty(s *Settings) {
	for event, groups := range s.Hooks {
		var nonEmpty []MatcherGroup
		for _, group := range groups {
			if len(group.Hooks) > 0 {
				nonEmpty = append(nonEmpty, group)
			}
		}
		if len(nonEmpty) == 0 {
			delete(s.Hooks, event)
		} else {
			s.Hooks[event] = nonEmpty
		}
	}
}
