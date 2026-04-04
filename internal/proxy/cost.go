package proxy

type modelPricing struct {
	Input      float64 // per 1M tokens
	Output     float64
	CacheRead  float64
	CacheWrite float64
}

// Pricing as of 2026-04. Update as needed.
var pricing = map[string]modelPricing{
	"claude-opus-4-6":            {15.0, 75.0, 1.50, 18.75},
	"claude-sonnet-4-6":          {3.0, 15.0, 0.30, 3.75},
	"claude-haiku-4-5":           {0.80, 4.0, 0.08, 1.0},
	"claude-haiku-4-5-20251001":  {0.80, 4.0, 0.08, 1.0},
	"claude-sonnet-4-5-20250514": {3.0, 15.0, 0.30, 3.75},
}

// estimateCost calculates the estimated USD cost for a request.
func estimateCost(model string, input, output, cacheRead, cacheCreate int) float64 {
	p, ok := pricing[model]
	if !ok {
		// Try prefix match for versioned model IDs (e.g. "claude-opus-4-6-20260301")
		for name, mp := range pricing {
			if len(model) > len(name) && model[:len(name)] == name {
				p = mp
				ok = true
				break
			}
		}
		if !ok {
			return 0
		}
	}
	return (float64(input)*p.Input + float64(output)*p.Output +
		float64(cacheRead)*p.CacheRead + float64(cacheCreate)*p.CacheWrite) / 1_000_000
}
