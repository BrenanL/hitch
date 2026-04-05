package proxy

import (
	"math"
	"os"
	"path/filepath"
	"testing"
)

func TestEstimateCostExactMatch(t *testing.T) {
	p := DefaultPricing()
	// claude-opus-4-6: Input=5.00, Output=25.00, CacheWrite=10.00, CacheRead=0.50
	// (1000*5 + 500*25 + 2000*0.50 + 100*10) / 1_000_000 = (5000+12500+1000+1000)/1e6 = 0.0195
	got := p.EstimateCost("claude-opus-4-6", 1000, 500, 2000, 100)
	want := 0.0195
	if math.Abs(got-want) > 0.000001 {
		t.Errorf("EstimateCost = %f, want %f", got, want)
	}
}

func TestEstimateCostPrefixMatch(t *testing.T) {
	// Use a single-model pricing map to ensure deterministic prefix matching
	p := Pricing{
		"claude-opus-4-6": {Input: 5.00, Output: 25.00, CacheWrite: 10.00, CacheRead: 0.50},
	}
	got := p.EstimateCost("claude-opus-4-6-20260205", 1000, 0, 0, 0)
	want := p.EstimateCost("claude-opus-4-6", 1000, 0, 0, 0)
	if got == 0 {
		t.Error("EstimateCost returned 0 for prefix-matchable model")
	}
	if math.Abs(got-want) > 0.000001 {
		t.Errorf("prefix match cost %f != exact match cost %f", got, want)
	}
}

func TestEstimateCostUnknownModel(t *testing.T) {
	p := DefaultPricing()
	got := p.EstimateCost("gpt-4", 1000, 500, 2000, 100)
	if got != 0 {
		t.Errorf("EstimateCost for unknown model = %f, want 0", got)
	}
}

func TestEstimateCostZeroTokens(t *testing.T) {
	p := DefaultPricing()
	got := p.EstimateCost("claude-opus-4-6", 0, 0, 0, 0)
	if got != 0 {
		t.Errorf("EstimateCost for zero tokens = %f, want 0", got)
	}
}

func TestLoadPricingFromFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "pricing.json")
	content := `{"claude-test": {"input": 1.0, "output": 2.0, "cache_write": 0.5, "cache_read": 0.1}}`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing test file: %v", err)
	}

	p, err := LoadPricingFromFile(path)
	if err != nil {
		t.Fatalf("LoadPricingFromFile: %v", err)
	}
	mp, ok := p["claude-test"]
	if !ok {
		t.Fatal("model claude-test not found in pricing")
	}
	if mp.Input != 1.0 || mp.Output != 2.0 || mp.CacheWrite != 0.5 || mp.CacheRead != 0.1 {
		t.Errorf("unexpected pricing values: %+v", mp)
	}
}

func TestLoadPricingFromFileWithComment(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "pricing.json")
	content := `{
		"_comment": "test comment",
		"claude-test": {"input": 1.0, "output": 2.0, "cache_write": 0.5, "cache_read": 0.1}
	}`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing test file: %v", err)
	}

	p, err := LoadPricingFromFile(path)
	if err != nil {
		t.Fatalf("LoadPricingFromFile: %v", err)
	}
	if _, ok := p["_comment"]; ok {
		t.Error("_comment key should be skipped")
	}
	if _, ok := p["claude-test"]; !ok {
		t.Error("claude-test model should be present")
	}
}

func TestLoadPricingFromMissingFile(t *testing.T) {
	_, err := LoadPricingFromFile("/nonexistent/pricing.json")
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestDefaultPricingNotEmpty(t *testing.T) {
	p := DefaultPricing()
	if len(p) == 0 {
		t.Fatal("DefaultPricing returned empty map")
	}
	if _, ok := p["claude-opus-4-6"]; !ok {
		t.Error("DefaultPricing missing claude-opus-4-6")
	}
}
