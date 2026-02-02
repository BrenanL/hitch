package generator

import (
	"path/filepath"
	"testing"
)

func TestManifestReadWriteRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "manifest.json")

	m := &Manifest{
		Version:      1,
		Scope:        "global",
		SettingsPath: "~/.claude/settings.json",
		Rules: map[string]ManifestEntry{
			"abc123": {
				DSL:     "on stop -> notify discord",
				Event:   "Stop",
				Marker:  "# ht:rule-abc123",
			},
		},
	}

	if err := WriteManifest(path, m); err != nil {
		t.Fatalf("WriteManifest: %v", err)
	}

	got, err := ReadManifest(path)
	if err != nil {
		t.Fatalf("ReadManifest: %v", err)
	}

	if got.Version != 1 {
		t.Errorf("version = %d", got.Version)
	}
	if got.Scope != "global" {
		t.Errorf("scope = %q", got.Scope)
	}
	if _, ok := got.Rules["abc123"]; !ok {
		t.Error("rule abc123 not found")
	}
}

func TestManifestReadNonexistent(t *testing.T) {
	m, err := ReadManifest("/nonexistent/manifest.json")
	if err != nil {
		t.Fatalf("ReadManifest nonexistent: %v", err)
	}
	if m.Rules == nil {
		t.Error("rules should be initialized")
	}
	if len(m.Rules) != 0 {
		t.Errorf("rules = %d, want 0", len(m.Rules))
	}
}

func TestManifestAllMarkers(t *testing.T) {
	m := &Manifest{
		Rules: map[string]ManifestEntry{
			"abc": {Marker: "# ht:rule-abc"},
			"def": {Marker: "# ht:rule-def"},
		},
		SystemHooks: map[string]ManifestEntry{
			"session-start": {Marker: "# ht:system:session-start"},
		},
	}

	markers := m.AllMarkers()
	if len(markers) != 3 {
		t.Errorf("markers = %d, want 3", len(markers))
	}
}

func TestUpdateManifest(t *testing.T) {
	m := &Manifest{Rules: make(map[string]ManifestEntry)}

	entries := []*HookEntryInfo{
		{Event: "Stop", Matcher: "", Marker: "# ht:rule-abc"},
		{Event: "SessionStart", Matcher: "*", Marker: "# ht:system:session-start"},
	}

	UpdateManifest(m, entries, "global", "~/.claude/settings.json")

	if len(m.Rules) != 1 {
		t.Errorf("rules = %d, want 1", len(m.Rules))
	}
	if len(m.SystemHooks) != 1 {
		t.Errorf("system hooks = %d, want 1", len(m.SystemHooks))
	}
}
