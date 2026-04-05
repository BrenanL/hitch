package metrics

import "time"

// BurnSample is a timestamped cumulative token count.
type BurnSample struct {
	Timestamp        time.Time
	CumulativeTokens int
}

// BurnRate computes tokens/minute from a slice of timestamped cumulative token counts
// over a rolling window. Samples outside the window are trimmed.
// Returns 0 for empty or single-sample inputs.
func BurnRate(samples []BurnSample, windowDuration time.Duration) float64 {
	if len(samples) < 2 {
		return 0
	}

	latest := samples[len(samples)-1].Timestamp
	cutoff := latest.Add(-windowDuration)

	// Find the first index within or just before the window cutoff.
	// Keep the last sample before the cutoff as the first sample (boundary).
	start := 0
	for i := 1; i < len(samples); i++ {
		if samples[i].Timestamp.Before(cutoff) {
			start = i
		} else {
			break
		}
	}

	windowed := samples[start:]
	if len(windowed) < 2 {
		return 0
	}

	first := windowed[0]
	last := windowed[len(windowed)-1]

	elapsed := last.Timestamp.Sub(first.Timestamp).Minutes()
	if elapsed <= 0 {
		return 0
	}

	return float64(last.CumulativeTokens-first.CumulativeTokens) / elapsed
}
