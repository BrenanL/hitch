package adapters

import (
	"testing"
)

func TestNewAdapterNtfy(t *testing.T) {
	a, err := NewAdapter("ntfy", map[string]string{
		"topic":  "test-topic",
		"server": "https://ntfy.example.com",
	})
	if err != nil {
		t.Fatalf("NewAdapter ntfy: %v", err)
	}
	if a.Name() != "ntfy" {
		t.Errorf("Name = %q, want %q", a.Name(), "ntfy")
	}
}

func TestNewAdapterNtfyMissingTopic(t *testing.T) {
	_, err := NewAdapter("ntfy", map[string]string{})
	if err == nil {
		t.Error("expected error for ntfy without topic")
	}
}

func TestNewAdapterNtfyDefaultServer(t *testing.T) {
	a, err := NewAdapter("ntfy", map[string]string{"topic": "t"})
	if err != nil {
		t.Fatalf("NewAdapter: %v", err)
	}
	// Should use default server — just verify it was created
	if a == nil {
		t.Error("adapter should not be nil")
	}
}

func TestNewAdapterDiscord(t *testing.T) {
	a, err := NewAdapter("discord", map[string]string{
		"webhook_url": "https://discord.com/api/webhooks/123/abc",
	})
	if err != nil {
		t.Fatalf("NewAdapter discord: %v", err)
	}
	if a.Name() != "discord" {
		t.Errorf("Name = %q", a.Name())
	}
}

func TestNewAdapterSlack(t *testing.T) {
	a, err := NewAdapter("slack", map[string]string{
		"webhook_url": "https://hooks.slack.com/services/T/B/X",
	})
	if err != nil {
		t.Fatalf("NewAdapter slack: %v", err)
	}
	if a.Name() != "slack" {
		t.Errorf("Name = %q", a.Name())
	}
}

func TestNewAdapterDesktop(t *testing.T) {
	a, err := NewAdapter("desktop", map[string]string{})
	if err != nil {
		t.Fatalf("NewAdapter desktop: %v", err)
	}
	if a.Name() != "desktop" {
		t.Errorf("Name = %q", a.Name())
	}
}

func TestNewAdapterUnknown(t *testing.T) {
	_, err := NewAdapter("carrier-pigeon", map[string]string{})
	if err == nil {
		t.Error("expected error for unknown adapter")
	}
}

func TestAvailableAdapters(t *testing.T) {
	available := AvailableAdapters()
	if len(available) == 0 {
		t.Fatal("no adapters registered")
	}

	expected := map[string]bool{"ntfy": false, "discord": false, "slack": false, "desktop": false}
	for _, name := range available {
		if _, ok := expected[name]; ok {
			expected[name] = true
		}
	}
	for name, found := range expected {
		if !found {
			t.Errorf("adapter %q not found in AvailableAdapters()", name)
		}
	}
}

// TestAdapterConfigRoundTrip verifies that config stored as JSON in DB
// can be used to reconstruct an adapter via the registry.
func TestAdapterConfigRoundTrip(t *testing.T) {
	configs := []struct {
		adapter string
		config  map[string]string
	}{
		{"ntfy", map[string]string{"topic": "my-topic", "server": "https://ntfy.example.com"}},
		{"discord", map[string]string{"webhook_url": "https://discord.com/api/webhooks/1/a"}},
		{"slack", map[string]string{"webhook_url": "https://hooks.slack.com/services/T/B/X"}},
		{"desktop", map[string]string{}},
	}

	for _, tc := range configs {
		t.Run(tc.adapter, func(t *testing.T) {
			a, err := NewAdapter(tc.adapter, tc.config)
			if err != nil {
				t.Fatalf("NewAdapter(%s): %v", tc.adapter, err)
			}
			if err := a.ValidateConfig(); err != nil {
				t.Fatalf("ValidateConfig: %v", err)
			}
		})
	}
}
