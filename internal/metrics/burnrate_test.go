package metrics

import (
	"math"
	"testing"
	"time"
)

const epsilon = 0.01

func TestBurnRateEmpty(t *testing.T) {
	got := BurnRate(nil, 5*time.Minute)
	if got != 0 {
		t.Errorf("BurnRate(nil) = %f, want 0", got)
	}

	got = BurnRate([]BurnSample{}, 5*time.Minute)
	if got != 0 {
		t.Errorf("BurnRate([]) = %f, want 0", got)
	}
}

func TestBurnRateSingleSample(t *testing.T) {
	now := time.Now()
	samples := []BurnSample{
		{Timestamp: now, CumulativeTokens: 500},
	}
	got := BurnRate(samples, 5*time.Minute)
	if got != 0 {
		t.Errorf("BurnRate(single) = %f, want 0", got)
	}
}

func TestBurnRateBasic(t *testing.T) {
	now := time.Now()
	samples := []BurnSample{
		{Timestamp: now, CumulativeTokens: 0},
		{Timestamp: now.Add(1 * time.Minute), CumulativeTokens: 1000},
	}
	want := 1000.0
	got := BurnRate(samples, 5*time.Minute)
	if math.Abs(got-want) >= epsilon {
		t.Errorf("BurnRate = %f, want %f", got, want)
	}
}

func TestBurnRateMultipleSamples(t *testing.T) {
	now := time.Now()
	samples := []BurnSample{
		{Timestamp: now, CumulativeTokens: 0},
		{Timestamp: now.Add(1 * time.Minute), CumulativeTokens: 500},
		{Timestamp: now.Add(2 * time.Minute), CumulativeTokens: 1200},
		{Timestamp: now.Add(3 * time.Minute), CumulativeTokens: 1800},
		{Timestamp: now.Add(4 * time.Minute), CumulativeTokens: 2500},
		{Timestamp: now.Add(5 * time.Minute), CumulativeTokens: 3000},
	}
	// 3000 tokens over 5 minutes = 600 tokens/min
	want := 600.0
	got := BurnRate(samples, 10*time.Minute)
	if math.Abs(got-want) >= epsilon {
		t.Errorf("BurnRate = %f, want %f", got, want)
	}
}

func TestBurnRateWindowTrimming(t *testing.T) {
	now := time.Now()
	samples := []BurnSample{
		{Timestamp: now, CumulativeTokens: 0},
		{Timestamp: now.Add(1 * time.Minute), CumulativeTokens: 200},
		{Timestamp: now.Add(2 * time.Minute), CumulativeTokens: 400},
		{Timestamp: now.Add(3 * time.Minute), CumulativeTokens: 600},
		{Timestamp: now.Add(4 * time.Minute), CumulativeTokens: 800},
		{Timestamp: now.Add(5 * time.Minute), CumulativeTokens: 1000},
		{Timestamp: now.Add(6 * time.Minute), CumulativeTokens: 1300},
		{Timestamp: now.Add(7 * time.Minute), CumulativeTokens: 1600},
		{Timestamp: now.Add(8 * time.Minute), CumulativeTokens: 1900},
		{Timestamp: now.Add(9 * time.Minute), CumulativeTokens: 2200},
		{Timestamp: now.Add(10 * time.Minute), CumulativeTokens: 2500},
	}
	// With a 5-minute window, latest is at 10min. Cutoff is at 5min.
	// The last sample strictly before cutoff is at 4min (tokens=800).
	// Windowed range: 4min (800) to 10min (2500) = 1700 tokens over 6 minutes ≈ 283.33 tokens/min.
	want := 283.33
	got := BurnRate(samples, 5*time.Minute)
	if math.Abs(got-want) >= epsilon {
		t.Errorf("BurnRate with window = %f, want %f", got, want)
	}
}

func TestBurnRateZeroElapsed(t *testing.T) {
	now := time.Now()
	samples := []BurnSample{
		{Timestamp: now, CumulativeTokens: 0},
		{Timestamp: now, CumulativeTokens: 1000},
	}
	got := BurnRate(samples, 5*time.Minute)
	if got != 0 {
		t.Errorf("BurnRate(zero elapsed) = %f, want 0", got)
	}
}

func TestBurnRateWindowLargerThanSpan(t *testing.T) {
	// When window is larger than total sample span, all samples are included.
	now := time.Now()
	samples := []BurnSample{
		{Timestamp: now, CumulativeTokens: 0},
		{Timestamp: now.Add(1 * time.Minute), CumulativeTokens: 300},
		{Timestamp: now.Add(2 * time.Minute), CumulativeTokens: 600},
	}
	// 600 tokens over 2 minutes = 300 tokens/min
	want := 300.0
	got := BurnRate(samples, 60*time.Minute)
	if math.Abs(got-want) >= epsilon {
		t.Errorf("BurnRate(window > span) = %f, want %f", got, want)
	}
}

func TestBurnRateAllSamplesOutsideWindow(t *testing.T) {
	// The boundary sample is included when it falls exactly at the cutoff or
	// is the last sample strictly before the cutoff. This ensures there is always
	// a start anchor for the rate calculation.
	now := time.Now()
	samples := []BurnSample{
		{Timestamp: now, CumulativeTokens: 0},
		{Timestamp: now.Add(1 * time.Minute), CumulativeTokens: 100},
		{Timestamp: now.Add(10 * time.Minute), CumulativeTokens: 700},
	}
	// window = 9 minutes, latest = 10min, cutoff = 1min
	// samples[1] at exactly 1min is NOT strictly before cutoff → loop breaks at i=1
	// start stays 0; windowed = all 3 samples
	// 700 tokens over 10 minutes = 70 tokens/min
	want := 70.0
	got := BurnRate(samples, 9*time.Minute)
	if math.Abs(got-want) >= epsilon {
		t.Errorf("BurnRate(boundary at cutoff) = %f, want %f", got, want)
	}
}
