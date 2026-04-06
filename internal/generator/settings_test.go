package generator

import (
	"encoding/json"
	"testing"
)

// newEmptySettings returns an empty Settings for use in tests.
func newEmptySettings() *Settings {
	s, _ := ParseSettings([]byte(`{}`))
	return s
}

func TestParseSettingsEmpty(t *testing.T) {
	s, err := ParseSettings([]byte(`{}`))
	if err != nil {
		t.Fatalf("ParseSettings: %v", err)
	}
	if len(s.Hooks) != 0 {
		t.Errorf("hooks = %d, want 0", len(s.Hooks))
	}
}

func TestParseSettingsWithHooks(t *testing.T) {
	input := `{
		"hooks": {
			"Stop": [
				{
					"matcher": "",
					"hooks": [
						{"type": "command", "command": "echo done"}
					]
				}
			]
		},
		"permissions": {"allow": ["Bash"]}
	}`

	s, err := ParseSettings([]byte(input))
	if err != nil {
		t.Fatalf("ParseSettings: %v", err)
	}

	if len(s.Hooks) != 1 {
		t.Fatalf("hooks = %d, want 1", len(s.Hooks))
	}
	if len(s.Hooks["Stop"]) != 1 {
		t.Fatalf("Stop groups = %d", len(s.Hooks["Stop"]))
	}
}

func TestParseSettingsInvalidJSON(t *testing.T) {
	_, err := ParseSettings([]byte(`not json`))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestMergeHooksAddNew(t *testing.T) {
	s := newEmptySettings()
	manifest := &Manifest{Rules: make(map[string]ManifestEntry)}

	entries := []*HookEntryInfo{
		{
			Event:   "Stop",
			Matcher: "",
			Marker:  "# ht:rule-abc123",
			Entry:   HookEntry{Type: "command", Command: "ht hook exec abc123 # ht:rule-abc123"},
		},
	}

	MergeHooks(s, manifest, entries)

	if len(s.Hooks["Stop"]) != 1 {
		t.Fatalf("Stop groups = %d, want 1", len(s.Hooks["Stop"]))
	}
	if len(s.Hooks["Stop"][0].Hooks) != 1 {
		t.Fatalf("hooks = %d", len(s.Hooks["Stop"][0].Hooks))
	}
}

func TestMergeHooksPreservesNonHitch(t *testing.T) {
	input := `{
		"hooks": {
			"Stop": [
				{
					"matcher": "",
					"hooks": [
						{"type": "command", "command": "echo user-hook"}
					]
				}
			]
		}
	}`
	s, _ := ParseSettings([]byte(input))
	manifest := &Manifest{Rules: make(map[string]ManifestEntry)}

	entries := []*HookEntryInfo{
		{
			Event:   "Stop",
			Matcher: "",
			Marker:  "# ht:rule-abc",
			Entry:   HookEntry{Type: "command", Command: "ht hook exec abc # ht:rule-abc"},
		},
	}

	MergeHooks(s, manifest, entries)

	stopGroups := s.Hooks["Stop"]
	if len(stopGroups) != 1 {
		t.Fatalf("Stop groups = %d", len(stopGroups))
	}
	// Should have both user hook and hitch hook
	if len(stopGroups[0].Hooks) != 2 {
		t.Fatalf("hooks = %d, want 2", len(stopGroups[0].Hooks))
	}

	// Verify user hook is preserved
	found := false
	for _, h := range stopGroups[0].Hooks {
		if h.Command == "echo user-hook" {
			found = true
		}
	}
	if !found {
		t.Error("user hook was not preserved")
	}
}

func TestMergeHooksRemovesOld(t *testing.T) {
	input := `{
		"hooks": {
			"Stop": [
				{
					"matcher": "",
					"hooks": [
						{"type": "command", "command": "ht hook exec old123 # ht:rule-old123"},
						{"type": "command", "command": "echo keep-me"}
					]
				}
			]
		}
	}`
	s, _ := ParseSettings([]byte(input))

	manifest := &Manifest{
		Rules: map[string]ManifestEntry{
			"old123": {Marker: "# ht:rule-old123"},
		},
	}

	// New entries replace old
	entries := []*HookEntryInfo{
		{
			Event:   "Stop",
			Matcher: "",
			Marker:  "# ht:rule-new456",
			Entry:   HookEntry{Type: "command", Command: "ht hook exec new456 # ht:rule-new456"},
		},
	}

	MergeHooks(s, manifest, entries)

	hooks := s.Hooks["Stop"][0].Hooks
	for _, h := range hooks {
		if h.Command == "ht hook exec old123 # ht:rule-old123" {
			t.Error("old hitch entry was not removed")
		}
	}
	if len(hooks) != 2 { // keep-me + new
		t.Errorf("hooks = %d, want 2", len(hooks))
	}
}

func TestMergeHooksIdempotent(t *testing.T) {
	s := newEmptySettings()
	manifest := &Manifest{Rules: make(map[string]ManifestEntry)}

	entries := []*HookEntryInfo{
		{
			Event:   "Stop",
			Matcher: "",
			Marker:  "# ht:rule-abc",
			Entry:   HookEntry{Type: "command", Command: "ht hook exec abc # ht:rule-abc"},
		},
	}

	// First merge
	MergeHooks(s, manifest, entries)
	count1 := len(s.Hooks["Stop"][0].Hooks)

	// Update manifest to track existing entries
	UpdateManifest(manifest, entries, "global", "~/.claude/settings.json")

	// Second merge (same entries)
	MergeHooks(s, manifest, entries)
	count2 := len(s.Hooks["Stop"][0].Hooks)

	if count1 != count2 {
		t.Errorf("sync is not idempotent: first=%d, second=%d", count1, count2)
	}
}

func TestMarshalSettingsPreservesOtherFields(t *testing.T) {
	input := `{
		"permissions": {"allow": ["Bash"]},
		"hooks": {
			"Stop": [{"matcher": "", "hooks": [{"type": "command", "command": "echo hi"}]}]
		}
	}`

	s, _ := ParseSettings([]byte(input))
	data, err := MarshalSettings(s)
	if err != nil {
		t.Fatalf("MarshalSettings: %v", err)
	}

	var output map[string]json.RawMessage
	json.Unmarshal(data, &output)

	if _, ok := output["permissions"]; !ok {
		t.Error("permissions field was not preserved")
	}
	if _, ok := output["hooks"]; !ok {
		t.Error("hooks field missing")
	}
}
