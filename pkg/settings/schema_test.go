package settings

import (
	"testing"
)

func TestSchema_AllKnownKeys(t *testing.T) {
	defs := Schema()
	if len(defs) == 0 {
		t.Fatal("Schema() returned empty list")
	}

	// Verify expected keys are present.
	expected := []string{
		"effortLevel",
		"model",
		"hooks",
		"env",
		"cleanupPeriodDays",
		"feedbackSurveyRate",
		"permissions",
		"defaultShell",
		"autoUpdatesChannel",
		"forceLoginMethod",
		"allowedMcpServers",
		"channelsEnabled",
		"blockedMarketplaces",
	}

	keyIndex := make(map[string]bool)
	for _, def := range defs {
		keyIndex[def.Name] = true
	}

	for _, key := range expected {
		if !keyIndex[key] {
			t.Errorf("expected key %q not found in Schema()", key)
		}
	}
}

func TestManagedOnlyKeys_Count(t *testing.T) {
	keys := ManagedOnlyKeys()
	if len(keys) < 13 {
		t.Errorf("ManagedOnlyKeys() returned %d keys, want at least 13; got: %v", len(keys), keys)
	}
}

func TestManagedOnlyKeys_Contents(t *testing.T) {
	keys := ManagedOnlyKeys()
	keySet := make(map[string]bool)
	for _, k := range keys {
		keySet[k] = true
	}

	expectedManaged := []string{
		"allowedChannelPlugins",
		"allowedMcpServers",
		"allowManagedHooksOnly",
		"allowManagedMcpServersOnly",
		"allowManagedPermissionRulesOnly",
		"blockedMarketplaces",
		"channelsEnabled",
		"deniedMcpServers",
		"forceLoginMethod",
		"forceLoginOrgUUID",
		"forceRemoteSettingsRefresh",
		"pluginTrustMessage",
		"strictKnownMarketplaces",
	}

	for _, key := range expectedManaged {
		if !keySet[key] {
			t.Errorf("expected managed-only key %q not found in ManagedOnlyKeys()", key)
		}
	}
}

func TestGlobalConfigKeys_Contents(t *testing.T) {
	keys := GlobalConfigKeys()
	keySet := make(map[string]bool)
	for _, k := range keys {
		keySet[k] = true
	}

	expectedGlobal := []string{
		"autoConnectIde",
		"autoInstallIdeExtension",
		"editorMode",
		"showTurnDuration",
		"terminalProgressBarEnabled",
		"teammateMode",
		"mcpServers",
		"projects",
		"theme",
	}

	for _, key := range expectedGlobal {
		if !keySet[key] {
			t.Errorf("expected global-config key %q not found in GlobalConfigKeys()", key)
		}
	}

	if len(keys) < 9 {
		t.Errorf("GlobalConfigKeys() returned %d keys, want at least 9", len(keys))
	}
}
