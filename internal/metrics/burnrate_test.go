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
