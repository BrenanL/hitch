package hookio

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestAllow(t *testing.T) {
	out := Allow()
	data, err := out.JSON()
	if err != nil {
		t.Fatalf("JSON: %v", err)
	}
	// Allow produces empty JSON object
	var m map[string]any
	json.Unmarshal(data, &m)
	if len(m) != 0 {
		t.Errorf("Allow should produce empty JSON, got %s", data)
	}
}

func TestDeny(t *testing.T) {
	out := Deny("blocked by safety guard")
	data, _ := out.JSON()
	var m map[string]any
	json.Unmarshal(data, &m)

	if m["decision"] != "deny" {
		t.Errorf("decision = %v, want deny", m["decision"])
	}
	if m["reason"] != "blocked by safety guard" {
		t.Errorf("reason = %v", m["reason"])
	}
}

func TestBlock(t *testing.T) {
	out := Block("dangerous command")
	data, _ := out.JSON()
	var m map[string]any
	json.Unmarshal(data, &m)

	if m["decision"] != "deny" {
		t.Errorf("Block should produce deny decision, got %v", m["decision"])
	}
}

func TestAsk(t *testing.T) {
	out := Ask("this command looks risky")
	data, _ := out.JSON()
	var m map[string]any
	json.Unmarshal(data, &m)

	if m["decision"] != "ask" {
		t.Errorf("decision = %v, want ask", m["decision"])
	}
}

func TestInjectContext(t *testing.T) {
	out := InjectContext("Remember to run tests")
	data, _ := out.JSON()
	var m map[string]any
	json.Unmarshal(data, &m)

	if m["additionalContext"] != "Remember to run tests" {
		t.Errorf("additionalContext = %v", m["additionalContext"])
	}
}

func TestContinueWorking(t *testing.T) {
	out := ContinueWorking("tests not passing")
	data, _ := out.JSON()
	var m map[string]any
	json.Unmarshal(data, &m)

	if m["continue"] != true {
		t.Errorf("continue = %v, want true", m["continue"])
	}
	if m["stopReason"] != "tests not passing" {
		t.Errorf("stopReason = %v", m["stopReason"])
	}
}

func TestStopWorking(t *testing.T) {
	out := StopWorking()
	data, _ := out.JSON()
	var m map[string]any
	json.Unmarshal(data, &m)

	if m["continue"] != false {
		t.Errorf("continue = %v, want false", m["continue"])
	}
}

func TestAllowPermission(t *testing.T) {
	out := AllowPermission()
	data, _ := out.JSON()
	var m map[string]any
	json.Unmarshal(data, &m)

	if m["permissionDecision"] != "allow" {
		t.Errorf("permissionDecision = %v, want allow", m["permissionDecision"])
	}
}

func TestDenyPermission(t *testing.T) {
	out := DenyPermission("not allowed")
	data, _ := out.JSON()
	var m map[string]any
	json.Unmarshal(data, &m)

	if m["permissionDecision"] != "deny" {
		t.Errorf("permissionDecision = %v, want deny", m["permissionDecision"])
	}
	if m["reason"] != "not allowed" {
		t.Errorf("reason = %v", m["reason"])
	}
}

func TestSuppressNotification(t *testing.T) {
	out := SuppressNotification()
	data, _ := out.JSON()
	var m map[string]any
	json.Unmarshal(data, &m)

	if m["suppressNotification"] != true {
		t.Errorf("suppressNotification = %v, want true", m["suppressNotification"])
	}
}

func TestWriteOutput(t *testing.T) {
	var buf bytes.Buffer
	out := Deny("blocked")
	if err := WriteOutput(&buf, out); err != nil {
		t.Fatalf("WriteOutput: %v", err)
	}

	var m map[string]any
	if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
		t.Fatalf("parsing output: %v", err)
	}
	if m["decision"] != "deny" {
		t.Errorf("decision = %v, want deny", m["decision"])
	}
}

func TestOutputOmitsEmptyFields(t *testing.T) {
	// Allow() should produce {} with no extra fields
	out := Allow()
	data, _ := out.JSON()
	if string(data) != "{}" {
		t.Errorf("Allow JSON = %s, want {}", data)
	}

	// Deny should only have decision and reason
	out = Deny("no")
	data, _ = out.JSON()
	var m map[string]any
	json.Unmarshal(data, &m)
	if _, ok := m["continue"]; ok {
		t.Error("Deny should not include continue field")
	}
	if _, ok := m["additionalContext"]; ok {
		t.Error("Deny should not include additionalContext field")
	}
}
