package state

import (
	"testing"
)

func TestRuleCRUD(t *testing.T) {
	db, err := OpenInMemory()
	if err != nil {
		t.Fatalf("OpenInMemory: %v", err)
	}
	defer db.Close()

	// Add
	r := Rule{
		ID:      "a1b2c3",
		DSL:     "on stop -> notify discord if elapsed > 30s",
		Scope:   "global",
		Enabled: true,
	}
	if err := db.RuleAdd(r); err != nil {
		t.Fatalf("RuleAdd: %v", err)
	}

	// List
	rules, err := db.RuleList()
	if err != nil {
		t.Fatalf("RuleList: %v", err)
	}
	if len(rules) != 1 {
		t.Fatalf("RuleList: got %d, want 1", len(rules))
	}
	if rules[0].DSL != r.DSL {
		t.Errorf("DSL = %q, want %q", rules[0].DSL, r.DSL)
	}

	// Get
	got, err := db.RuleGet("a1b2c3")
	if err != nil {
		t.Fatalf("RuleGet: %v", err)
	}
	if got == nil {
		t.Fatal("RuleGet: got nil")
	}
	if got.Scope != "global" {
		t.Errorf("Scope = %q, want %q", got.Scope, "global")
	}

	// Get not found
	got, err = db.RuleGet("nonexistent")
	if err != nil {
		t.Fatalf("RuleGet not found: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil for nonexistent rule")
	}

	// Disable
	if err := db.RuleDisable("a1b2c3"); err != nil {
		t.Fatalf("RuleDisable: %v", err)
	}
	got, _ = db.RuleGet("a1b2c3")
	if got.Enabled {
		t.Error("Enabled should be false after disable")
	}

	// Enable
	if err := db.RuleEnable("a1b2c3"); err != nil {
		t.Fatalf("RuleEnable: %v", err)
	}
	got, _ = db.RuleGet("a1b2c3")
	if !got.Enabled {
		t.Error("Enabled should be true after enable")
	}

	// ListByScope
	r2 := Rule{ID: "d4e5f6", DSL: "on pre-bash -> deny", Scope: "project:/tmp", Enabled: true}
	db.RuleAdd(r2)
	scoped, err := db.RuleListByScope("global")
	if err != nil {
		t.Fatalf("RuleListByScope: %v", err)
	}
	if len(scoped) != 1 {
		t.Errorf("RuleListByScope global: got %d, want 1", len(scoped))
	}

	// Remove
	if err := db.RuleRemove("a1b2c3"); err != nil {
		t.Fatalf("RuleRemove: %v", err)
	}
	rules, _ = db.RuleList()
	if len(rules) != 1 {
		t.Errorf("after remove: got %d rules, want 1", len(rules))
	}

	// Remove not found
	if err := db.RuleRemove("nonexistent"); err == nil {
		t.Error("RuleRemove of nonexistent should return error")
	}
}

func TestRuleDuplicateID(t *testing.T) {
	db, err := OpenInMemory()
	if err != nil {
		t.Fatalf("OpenInMemory: %v", err)
	}
	defer db.Close()

	r := Rule{ID: "dup1", DSL: "on stop -> log", Scope: "global", Enabled: true}
	if err := db.RuleAdd(r); err != nil {
		t.Fatalf("first add: %v", err)
	}

	// Adding same ID again should fail
	r2 := Rule{ID: "dup1", DSL: "on stop -> deny", Scope: "global", Enabled: true}
	if err := db.RuleAdd(r2); err == nil {
		t.Error("expected error when adding duplicate rule ID")
	}
}

func TestRuleEnableNonexistent(t *testing.T) {
	db, err := OpenInMemory()
	if err != nil {
		t.Fatalf("OpenInMemory: %v", err)
	}
	defer db.Close()

	if err := db.RuleEnable("ghost"); err == nil {
		t.Error("expected error enabling nonexistent rule")
	}
	if err := db.RuleDisable("ghost"); err == nil {
		t.Error("expected error disabling nonexistent rule")
	}
}

func TestRuleListEmpty(t *testing.T) {
	db, err := OpenInMemory()
	if err != nil {
		t.Fatalf("OpenInMemory: %v", err)
	}
	defer db.Close()

	rules, err := db.RuleList()
	if err != nil {
		t.Fatalf("RuleList: %v", err)
	}
	if rules != nil && len(rules) != 0 {
		t.Errorf("empty DB should return empty list, got %d", len(rules))
	}
}

func TestRuleTimestampsAutoFilled(t *testing.T) {
	db, err := OpenInMemory()
	if err != nil {
		t.Fatalf("OpenInMemory: %v", err)
	}
	defer db.Close()

	r := Rule{ID: "ts1", DSL: "on stop -> log", Scope: "global", Enabled: true}
	if err := db.RuleAdd(r); err != nil {
		t.Fatalf("RuleAdd: %v", err)
	}

	got, _ := db.RuleGet("ts1")
	if got.CreatedAt == "" {
		t.Error("CreatedAt should be auto-filled")
	}
	if got.UpdatedAt == "" {
		t.Error("UpdatedAt should be auto-filled")
	}
}
