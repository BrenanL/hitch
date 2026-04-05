# internal/pricing

Canonical cost estimation for Anthropic API requests. Extracted from `internal/proxy/cost.go` so that all components (proxy, session analysis, dashboard, `ht costs`) share a single pricing source.

## Types

### ModelPricing

Holds per-million-token costs for a single model.

```go
type ModelPricing struct {
    Input      float64 `json:"input"`
    Output     float64 `json:"output"`
    CacheWrite float64 `json:"cache_write"`
    CacheRead  float64 `json:"cache_read"`
}
```

All values are USD per million tokens. `CacheWrite` uses the 1-hour cache tier.

### Pricing

`type Pricing map[string]ModelPricing`

A map from model name strings to their pricing data.

## Key Functions

| Function | Description |
|---|---|
| `LoadPricing() Pricing` | Load pricing from `~/.hitch/pricing.json`; falls back to `DefaultPricing()` if the file is absent or unreadable. |
| `LoadPricingFromFile(path string) (Pricing, error)` | Load pricing from any JSON file at the given path. |
| `DefaultPricing() Pricing` | Returns the embedded pricing table for known Claude models. |
| `(p Pricing) EstimateCost(model string, input, output, cacheRead, cacheCreate int) float64` | Compute estimated USD cost for one request. Token counts are raw counts (not per-million). Returns 0 if the model is not found. Supports prefix matching (e.g. `claude-sonnet-4-6-20250101` matches `claude-sonnet-4-6`). |
| `PricingFilePath() string` | Returns `~/.hitch/pricing.json`. |
| `SeedPricingFile() error` | Write `DefaultPricing()` to `~/.hitch/pricing.json` if it does not exist. |
| `WritePricingFile(p Pricing) error` | Write a `Pricing` map to `~/.hitch/pricing.json` (with metadata comment). |
| `FetchLiteLLMPricing() (Pricing, error)` | Fetch current pricing from LiteLLM's GitHub model list and convert to `Pricing`. Claude-only; skips provider-prefixed keys. |

## Usage

```go
import "github.com/BrenanL/hitch/internal/pricing"

p := pricing.LoadPricing()
cost := p.EstimateCost("claude-sonnet-4-6", inputTokens, outputTokens, cacheReadTokens, cacheWriteTokens)
```

## Adding New Models

To add a model to the embedded defaults, add an entry to `DefaultPricing()` in `pricing.go`:

```go
"claude-new-model": {Input: 3.00, Output: 15.00, CacheWrite: 6.00, CacheRead: 0.30},
```

All values are USD per million tokens. Use the 1-hour cache tier for `CacheWrite`.

To override pricing at runtime without changing the source, write a `~/.hitch/pricing.json` file (see file format below) or call `FetchLiteLLMPricing()` and write the result with `WritePricingFile`.

## pricing.json File Format

`~/.hitch/pricing.json` is a JSON object where each key is a model name and each value is a `ModelPricing` object. Keys prefixed with `_` are treated as metadata and ignored.

```json
{
  "_comment": "Prices in USD per million tokens. cache_write uses 1h tier. Updated 2026-03-31.",
  "claude-sonnet-4-6": {
    "input": 3.00,
    "output": 15.00,
    "cache_write": 6.00,
    "cache_read": 0.30
  }
}
```

Entries with `input == 0` are skipped. The file is loaded by `LoadPricing()` at startup; if absent, `DefaultPricing()` is used instead.
