package proxy

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const litellmPricingURL = "https://raw.githubusercontent.com/BerriAI/litellm/main/model_prices_and_context_window.json"

// ModelPricing holds per-million-token costs for a model.
type ModelPricing struct {
	Input      float64 `json:"input"`
	Output     float64 `json:"output"`
	CacheWrite float64 `json:"cache_write"`
	CacheRead  float64 `json:"cache_read"`
}

// Pricing maps model names to their pricing.
type Pricing map[string]ModelPricing

// PricingFilePath returns the path to the pricing JSON file.
func PricingFilePath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".hitch", "pricing.json")
}

// LoadPricing loads pricing from ~/.hitch/pricing.json, falling back to embedded defaults.
func LoadPricing() Pricing {
	p, err := LoadPricingFromFile(PricingFilePath())
	if err != nil {
		return DefaultPricing()
	}
	return p
}

// LoadPricingFromFile loads pricing from a JSON file.
func LoadPricingFromFile(path string) (Pricing, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parsing pricing file: %w", err)
	}

	pricing := make(Pricing)
	for key, val := range raw {
		if strings.HasPrefix(key, "_") {
			continue
		}
		var mp ModelPricing
		if json.Unmarshal(val, &mp) == nil && mp.Input > 0 {
			pricing[key] = mp
		}
	}
	return pricing, nil
}

// DefaultPricing returns embedded pricing (1h cache tier, updated 2026-03-31).
func DefaultPricing() Pricing {
	return Pricing{
		"claude-opus-4-6":            {Input: 5.00, Output: 25.00, CacheWrite: 10.00, CacheRead: 0.50},
		"claude-opus-4-5":            {Input: 5.00, Output: 25.00, CacheWrite: 10.00, CacheRead: 0.50},
		"claude-opus-4-1":            {Input: 15.00, Output: 75.00, CacheWrite: 30.00, CacheRead: 1.50},
		"claude-opus-4":              {Input: 15.00, Output: 75.00, CacheWrite: 30.00, CacheRead: 1.50},
		"claude-sonnet-4-6":          {Input: 3.00, Output: 15.00, CacheWrite: 6.00, CacheRead: 0.30},
		"claude-sonnet-4-5":          {Input: 3.00, Output: 15.00, CacheWrite: 6.00, CacheRead: 0.30},
		"claude-sonnet-4":            {Input: 3.00, Output: 15.00, CacheWrite: 6.00, CacheRead: 0.30},
		"claude-sonnet-3-7":          {Input: 3.00, Output: 15.00, CacheWrite: 6.00, CacheRead: 0.30},
		"claude-haiku-4-5-20251001":  {Input: 1.00, Output: 5.00, CacheWrite: 2.00, CacheRead: 0.10},
		"claude-haiku-3-5":           {Input: 0.80, Output: 4.00, CacheWrite: 1.60, CacheRead: 0.08},
		"claude-haiku-3":             {Input: 0.25, Output: 1.25, CacheWrite: 0.50, CacheRead: 0.03},
	}
}

// EstimateCost computes the estimated USD cost for a single request.
func (p Pricing) EstimateCost(model string, input, output, cacheRead, cacheCreate int) float64 {
	mp, ok := p[model]
	if !ok {
		for name, m := range p {
			if len(model) > len(name) && model[:len(name)] == name {
				mp = m
				ok = true
				break
			}
		}
		if !ok {
			return 0
		}
	}
	return (float64(input)*mp.Input + float64(output)*mp.Output +
		float64(cacheRead)*mp.CacheRead + float64(cacheCreate)*mp.CacheWrite) / 1_000_000
}

// SeedPricingFile writes default pricing to ~/.hitch/pricing.json if it doesn't exist.
func SeedPricingFile() error {
	path := PricingFilePath()
	if _, err := os.Stat(path); err == nil {
		return nil // already exists
	}
	return WritePricingFile(DefaultPricing())
}

// WritePricingFile writes pricing to ~/.hitch/pricing.json.
func WritePricingFile(p Pricing) error {
	path := PricingFilePath()
	os.MkdirAll(filepath.Dir(path), 0o755)

	// Wrap with metadata comment
	wrapper := make(map[string]any)
	wrapper["_comment"] = fmt.Sprintf("Prices in USD per million tokens. cache_write uses 1h tier. Updated %s.", time.Now().Format("2006-01-02"))
	for k, v := range p {
		wrapper[k] = v
	}

	data, err := json.MarshalIndent(wrapper, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

// FetchLiteLLMPricing fetches pricing from LiteLLM's GitHub repo and converts to our format.
func FetchLiteLLMPricing() (Pricing, error) {
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get(litellmPricingURL)
	if err != nil {
		return nil, fmt.Errorf("fetching LiteLLM pricing: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("LiteLLM returned HTTP %d", resp.StatusCode)
	}

	var raw map[string]json.RawMessage
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("parsing LiteLLM pricing: %w", err)
	}

	type litellmModel struct {
		InputCost      float64 `json:"input_cost_per_token"`
		OutputCost     float64 `json:"output_cost_per_token"`
		CacheWriteCost float64 `json:"cache_creation_input_token_cost"`
		CacheReadCost  float64 `json:"cache_read_input_token_cost"`
	}

	pricing := make(Pricing)
	for key, val := range raw {
		if !strings.HasPrefix(key, "claude-") {
			continue
		}
		// Skip provider-prefixed entries like "anthropic/claude-..."
		if strings.Contains(key, "/") {
			continue
		}
		var lm litellmModel
		if json.Unmarshal(val, &lm) != nil || lm.InputCost == 0 {
			continue
		}
		pricing[key] = ModelPricing{
			Input:      lm.InputCost * 1_000_000,
			Output:     lm.OutputCost * 1_000_000,
			CacheWrite: lm.CacheWriteCost * 1_000_000,
			CacheRead:  lm.CacheReadCost * 1_000_000,
		}
	}

	if len(pricing) == 0 {
		return nil, fmt.Errorf("no Claude models found in LiteLLM pricing data")
	}
	return pricing, nil
}
