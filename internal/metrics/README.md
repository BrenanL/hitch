# internal/metrics

Canonical burn rate and token velocity definitions for Hitch. Components that display or act on token consumption rates (daemon, dashboard, hook conditions, alerts) should use this package to ensure consistent calculations.

## Types

### BurnSample

```go
type BurnSample struct {
    Timestamp        time.Time
    CumulativeTokens int
}
```

A timestamped snapshot of cumulative token usage. Samples are expected to be in ascending timestamp order.

## Functions

### BurnRate

```go
func BurnRate(samples []BurnSample, windowDuration time.Duration) float64
```

Computes tokens per minute over a rolling window.

**Algorithm:**

1. Determine the window boundary: `latest.Timestamp - windowDuration`.
2. Walk backward through samples to find the last sample that falls before the cutoff. That sample is used as the window's starting point (boundary anchoring).
3. Compute `(last.CumulativeTokens - first.CumulativeTokens) / elapsed_minutes` over the windowed slice.

**Returns 0 when:**
- Fewer than 2 samples are provided.
- The windowed slice contains fewer than 2 samples.
- Elapsed time within the window is zero or negative.

## Cache Read Weighting

`BurnRate` treats all tokens equally. If callers want to weight cache read tokens differently (e.g. at a lower cost equivalent), they should adjust `CumulativeTokens` in their `BurnSample` values before passing them to `BurnRate`. This package does not make weighting decisions.

## Usage

```go
import (
    "time"
    "github.com/BrenanL/hitch/internal/metrics"
)

samples := []metrics.BurnSample{
    {Timestamp: t0, CumulativeTokens: 1000},
    {Timestamp: t1, CumulativeTokens: 3500},
    {Timestamp: t2, CumulativeTokens: 6000},
}

rate := metrics.BurnRate(samples, 5*time.Minute) // tokens/minute over last 5 minutes
```
