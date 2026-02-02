package generator

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Manifest tracks which settings.json entries hitch owns.
type Manifest struct {
	Version      int                       `json:"version"`
	Scope        string                    `json:"scope"`
	SettingsPath string                    `json:"settings_path"`
	Rules        map[string]ManifestEntry  `json:"rules"`
	SystemHooks  map[string]ManifestEntry  `json:"system_hooks,omitempty"`
}

// ManifestEntry tracks a single hook entry owned by hitch.
type ManifestEntry struct {
	DSL         string `json:"dsl,omitempty"`
	Event       string `json:"event"`
	Matcher     string `json:"matcher"`
	Marker      string `json:"marker"`
	GeneratedAt string `json:"generated_at"`
}

// ReadManifest reads a manifest from disk. Returns empty manifest if not found.
func ReadManifest(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return &Manifest{
			Version: 1,
			Rules:   make(map[string]ManifestEntry),
		}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading manifest: %w", err)
	}

	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parsing manifest: %w", err)
	}
	if m.Rules == nil {
		m.Rules = make(map[string]ManifestEntry)
	}
	return &m, nil
}

// WriteManifest writes a manifest to disk.
func WriteManifest(path string, m *Manifest) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating manifest directory: %w", err)
	}

	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling manifest: %w", err)
	}

	return os.WriteFile(path, data, 0o644)
}

// UpdateManifest updates a manifest with the current set of hook entries.
func UpdateManifest(m *Manifest, entries []*HookEntryInfo, scope, settingsPath string) {
	m.Version = 1
	m.Scope = scope
	m.SettingsPath = settingsPath
	m.Rules = make(map[string]ManifestEntry)
	m.SystemHooks = make(map[string]ManifestEntry)

	now := time.Now().UTC().Format(time.RFC3339)

	for _, e := range entries {
		entry := ManifestEntry{
			Event:       e.Event,
			Matcher:     e.Matcher,
			Marker:      e.Marker,
			GeneratedAt: now,
		}

		// System hooks go in a separate map
		if len(e.Marker) > 12 && e.Marker[:12] == "# ht:system:" {
			name := e.Marker[12:]
			m.SystemHooks[name] = entry
		} else {
			// Extract rule ID from marker "# ht:rule-<id>"
			if len(e.Marker) > 10 {
				ruleID := e.Marker[10:] // skip "# ht:rule-"
				m.Rules[ruleID] = entry
			}
		}
	}
}

// AllMarkers returns all ownership markers from a manifest.
func (m *Manifest) AllMarkers() []string {
	markers := make([]string, 0, len(m.Rules)+len(m.SystemHooks))
	for _, e := range m.Rules {
		markers = append(markers, e.Marker)
	}
	for _, e := range m.SystemHooks {
		markers = append(markers, e.Marker)
	}
	return markers
}
