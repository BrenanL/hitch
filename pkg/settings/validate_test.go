package settings

import (
	"os"
	"path/filepath"
	"testing"
)

// findIssue returns the first ValidationIssue with the given key and level, or nil.
func findIssue(issues []ValidationIssue, key, level string) *ValidationIssue {
	for i := range issues {
		if issues[i].Key == key && issues[i].Level == level {
			return &issues[i]
		}
	}
	return nil
}

func TestValidate_ManagedOnlyInProject(t *testing.T) {
	s, err := ParseSettings([]byte(`{"forceLoginMethod": "claudeai"}`))
	if err != nil {
		t.Fatalf("ParseSettings: %v", err)
	}

	issues := Validate(s, ScopeProject)
	if issue := findIssue(issues, "forceLoginMethod", "error"); issue == nil {
		t.Errorf("expected error for managed-only key in project scope, got issues: %v", issues)
	}
}

func TestValidate_ManagedOnlyInManaged(t *testing.T) {
	// Managed-only keys are allowed in managed scope — no error.
	s, err := ParseSettings([]byte(`{"forceLoginMethod": "claudeai"}`))
	if err != nil {
		t.Fatalf("ParseSettings: %v", err)
	}

	issues := Validate(s, ScopeManaged)
	for _, issue := range issues {
		if issue.Key == "forceLoginMethod" && issue.Level == "error" {
			t.Errorf("unexpected error for managed-only key in managed scope: %s", issue.Message)
		}
	}
}

func TestValidate_GlobalConfigInSettings(t *testing.T) {
	s, err := ParseSettings([]byte(`{"theme": "dark"}`))
	if err != nil {
		t.Fatalf("ParseSettings: %v", err)
	}

	issues := Validate(s, ScopeUser)
	if issue := findIssue(issues, "theme", "warning"); issue == nil {
		t.Errorf("expected warning for global-config key in settings.json, got issues: %v", issues)
	}
}

func TestValidate_InvalidEnumValue(t *testing.T) {
	s, err := ParseSettings([]byte(`{"effortLevel": "extreme"}`))
	if err != nil {
		t.Fatalf("ParseSettings: %v", err)
	}

	issues := Validate(s, ScopeUser)
	if issue := findIssue(issues, "effortLevel", "error"); issue == nil {
		t.Errorf("expected error for invalid enum value, got issues: %v", issues)
	}
}

func TestValidate_ValidEnumValue(t *testing.T) {
	s, err := ParseSettings([]byte(`{"effortLevel": "high"}`))
	if err != nil {
		t.Fatalf("ParseSettings: %v", err)
	}

	issues := Validate(s, ScopeUser)
	for _, issue := range issues {
		if issue.Key == "effortLevel" && issue.Level == "error" {
			t.Errorf("unexpected error for valid enum value: %s", issue.Message)
		}
	}
}

func TestValidate_InvalidDefaultMode(t *testing.T) {
	s, err := ParseSettings([]byte(`{"permissions": {"defaultMode": "invalid"}}`))
	if err != nil {
		t.Fatalf("ParseSettings: %v", err)
	}

	issues := Validate(s, ScopeUser)
	if issue := findIssue(issues, "permissions.defaultMode", "error"); issue == nil {
		t.Errorf("expected error for invalid permissions.defaultMode, got issues: %v", issues)
	}
}

func TestValidate_ValidDefaultMode(t *testing.T) {
	s, err := ParseSettings([]byte(`{"permissions": {"defaultMode": "auto"}}`))
	if err != nil {
		t.Fatalf("ParseSettings: %v", err)
	}

	issues := Validate(s, ScopeUser)
	for _, issue := range issues {
		if issue.Key == "permissions.defaultMode" && issue.Level == "error" {
			t.Errorf("unexpected error for valid defaultMode: %s", issue.Message)
		}
	}
}

func TestValidate_CleanupPeriodDays_Zero(t *testing.T) {
	s, err := ParseSettings([]byte(`{"cleanupPeriodDays": 0}`))
	if err != nil {
		t.Fatalf("ParseSettings: %v", err)
	}

	issues := Validate(s, ScopeUser)
	if issue := findIssue(issues, "cleanupPeriodDays", "error"); issue == nil {
		t.Errorf("expected error for cleanupPeriodDays=0, got issues: %v", issues)
	}
}

func TestValidate_CleanupPeriodDays_Negative(t *testing.T) {
	s, err := ParseSettings([]byte(`{"cleanupPeriodDays": -5}`))
	if err != nil {
		t.Fatalf("ParseSettings: %v", err)
	}

	issues := Validate(s, ScopeUser)
	if issue := findIssue(issues, "cleanupPeriodDays", "error"); issue == nil {
		t.Errorf("expected error for cleanupPeriodDays=-5, got issues: %v", issues)
	}
}

func TestValidate_CleanupPeriodDays_Valid(t *testing.T) {
	s, err := ParseSettings([]byte(`{"cleanupPeriodDays": 30}`))
	if err != nil {
		t.Fatalf("ParseSettings: %v", err)
	}

	issues := Validate(s, ScopeUser)
	for _, issue := range issues {
		if issue.Key == "cleanupPeriodDays" && issue.Level == "error" {
			t.Errorf("unexpected error for valid cleanupPeriodDays: %s", issue.Message)
		}
	}
}

func TestValidate_FeedbackSurveyRate_TooHigh(t *testing.T) {
	s, err := ParseSettings([]byte(`{"feedbackSurveyRate": 1.5}`))
	if err != nil {
		t.Fatalf("ParseSettings: %v", err)
	}

	issues := Validate(s, ScopeUser)
	if issue := findIssue(issues, "feedbackSurveyRate", "error"); issue == nil {
		t.Errorf("expected error for feedbackSurveyRate=1.5, got issues: %v", issues)
	}
}

func TestValidate_FeedbackSurveyRate_TooLow(t *testing.T) {
	s, err := ParseSettings([]byte(`{"feedbackSurveyRate": -0.1}`))
	if err != nil {
		t.Fatalf("ParseSettings: %v", err)
	}

	issues := Validate(s, ScopeUser)
	if issue := findIssue(issues, "feedbackSurveyRate", "error"); issue == nil {
		t.Errorf("expected error for feedbackSurveyRate=-0.1, got issues: %v", issues)
	}
}

func TestValidate_FeedbackSurveyRate_Valid(t *testing.T) {
	s, err := ParseSettings([]byte(`{"feedbackSurveyRate": 0.5}`))
	if err != nil {
		t.Fatalf("ParseSettings: %v", err)
	}

	issues := Validate(s, ScopeUser)
	for _, issue := range issues {
		if issue.Key == "feedbackSurveyRate" && issue.Level == "error" {
			t.Errorf("unexpected error for valid feedbackSurveyRate: %s", issue.Message)
		}
	}
}

func TestLoadGlobalConfig_AllKeys(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	content := `{
		"theme": "dark",
		"preferredNotify": "email",
		"autoUpdateStatus": "stable",
		"autoConnectIde": true,
		"autoInstallIdeExtension": false,
		"editorMode": "vim",
		"showTurnDuration": false,
		"terminalProgressBarEnabled": true,
		"teammateMode": "tmux"
	}`

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing config: %v", err)
	}

	gc, err := loadGlobalConfigFromPath(path)
	if err != nil {
		t.Fatalf("loadGlobalConfigFromPath: %v", err)
	}

	if gc.Theme != "dark" {
		t.Errorf("Theme = %q, want %q", gc.Theme, "dark")
	}
	if gc.PreferredNotify != "email" {
		t.Errorf("PreferredNotify = %q, want %q", gc.PreferredNotify, "email")
	}
	if gc.AutoUpdateStatus != "stable" {
		t.Errorf("AutoUpdateStatus = %q, want %q", gc.AutoUpdateStatus, "stable")
	}
	if gc.AutoConnectIde == nil || *gc.AutoConnectIde != true {
		t.Error("AutoConnectIde should be true")
	}
	if gc.AutoInstallIdeExtension == nil || *gc.AutoInstallIdeExtension != false {
		t.Error("AutoInstallIdeExtension should be false")
	}
	if gc.EditorMode != "vim" {
		t.Errorf("EditorMode = %q, want %q", gc.EditorMode, "vim")
	}
	if gc.ShowTurnDuration == nil || *gc.ShowTurnDuration != false {
		t.Error("ShowTurnDuration should be false")
	}
	if gc.TerminalProgressBarEnabled == nil || *gc.TerminalProgressBarEnabled != true {
		t.Error("TerminalProgressBarEnabled should be true")
	}
	if gc.TeammateMode != "tmux" {
		t.Errorf("TeammateMode = %q, want %q", gc.TeammateMode, "tmux")
	}
}

func TestGlobalConfig_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	boolTrue := true
	boolFalse := false
	original := &GlobalConfig{
		Theme:                      "dark",
		PreferredNotify:            "slack",
		AutoUpdateStatus:           "latest",
		AutoConnectIde:             &boolTrue,
		AutoInstallIdeExtension:    &boolFalse,
		EditorMode:                 "vim",
		ShowTurnDuration:           &boolTrue,
		TerminalProgressBarEnabled: &boolFalse,
		TeammateMode:               "auto",
	}

	if err := writeGlobalConfigToPath(path, original); err != nil {
		t.Fatalf("writeGlobalConfigToPath: %v", err)
	}

	loaded, err := loadGlobalConfigFromPath(path)
	if err != nil {
		t.Fatalf("loadGlobalConfigFromPath: %v", err)
	}

	if loaded.Theme != original.Theme {
		t.Errorf("Theme = %q, want %q", loaded.Theme, original.Theme)
	}
	if loaded.PreferredNotify != original.PreferredNotify {
		t.Errorf("PreferredNotify = %q, want %q", loaded.PreferredNotify, original.PreferredNotify)
	}
	if loaded.AutoUpdateStatus != original.AutoUpdateStatus {
		t.Errorf("AutoUpdateStatus = %q, want %q", loaded.AutoUpdateStatus, original.AutoUpdateStatus)
	}
	if loaded.AutoConnectIde == nil || *loaded.AutoConnectIde != *original.AutoConnectIde {
		t.Errorf("AutoConnectIde mismatch")
	}
	if loaded.AutoInstallIdeExtension == nil || *loaded.AutoInstallIdeExtension != *original.AutoInstallIdeExtension {
		t.Errorf("AutoInstallIdeExtension mismatch")
	}
	if loaded.EditorMode != original.EditorMode {
		t.Errorf("EditorMode = %q, want %q", loaded.EditorMode, original.EditorMode)
	}
	if loaded.ShowTurnDuration == nil || *loaded.ShowTurnDuration != *original.ShowTurnDuration {
		t.Errorf("ShowTurnDuration mismatch")
	}
	if loaded.TerminalProgressBarEnabled == nil || *loaded.TerminalProgressBarEnabled != *original.TerminalProgressBarEnabled {
		t.Errorf("TerminalProgressBarEnabled mismatch")
	}
	if loaded.TeammateMode != original.TeammateMode {
		t.Errorf("TeammateMode = %q, want %q", loaded.TeammateMode, original.TeammateMode)
	}
}

func TestLoadGlobalConfig_MissingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nonexistent.json")

	gc, err := loadGlobalConfigFromPath(path)
	if err != nil {
		t.Fatalf("expected no error for missing file, got: %v", err)
	}
	if gc == nil {
		t.Fatal("expected non-nil GlobalConfig for missing file")
	}
}
