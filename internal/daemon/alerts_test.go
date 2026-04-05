package daemon

import (
	"testing"
	"time"
)

func TestAlertEvaluatorBurnRateSpike(t *testing.T) {
	config := DefaultAlertConfig()
	config.BurnRateThresholdTPM = 1000
	ae := NewAlertEvaluator(config, nil)

	tracker := NewSessionTracker()
	s := tracker.GetOrCreate("s1")
	s.BurnRateTPM = 2000
	s.ProjectDir = "~/dev/test"

	ae.Evaluate(tracker)

	alerts := ae.RecentAlerts(10)
	if len(alerts) != 1 {
		t.Fatalf("alerts = %d, want 1", len(alerts))
	}
	if alerts[0].Type != AlertBurnRateSpike {
		t.Errorf("Type = %q", alerts[0].Type)
	}
	if alerts[0].Level != "warning" {
		t.Errorf("Level = %q", alerts[0].Level)
	}
}

func TestAlertEvaluatorBelowThreshold(t *testing.T) {
	config := DefaultAlertConfig()
	config.BurnRateThresholdTPM = 5000
	ae := NewAlertEvaluator(config, nil)

	tracker := NewSessionTracker()
	s := tracker.GetOrCreate("s1")
	s.BurnRateTPM = 2000

	ae.Evaluate(tracker)

	alerts := ae.RecentAlerts(10)
	if len(alerts) != 0 {
		t.Errorf("alerts = %d, want 0 (below threshold)", len(alerts))
	}
}

func TestAlertDeduplication(t *testing.T) {
	config := DefaultAlertConfig()
	config.BurnRateThresholdTPM = 1000
	config.CooldownMinutes = 10
	ae := NewAlertEvaluator(config, nil)

	tracker := NewSessionTracker()
	s := tracker.GetOrCreate("s1")
	s.BurnRateTPM = 2000

	ae.Evaluate(tracker)
	ae.Evaluate(tracker)
	ae.Evaluate(tracker)

	alerts := ae.RecentAlerts(10)
	if len(alerts) != 1 {
		t.Errorf("alerts = %d, want 1 (deduplication should prevent repeats)", len(alerts))
	}
}

func TestAlertDeduplicationExpires(t *testing.T) {
	config := DefaultAlertConfig()
	config.BurnRateThresholdTPM = 1000
	config.CooldownMinutes = 0 // no cooldown
	ae := NewAlertEvaluator(config, nil)

	tracker := NewSessionTracker()
	s := tracker.GetOrCreate("s1")
	s.BurnRateTPM = 2000

	ae.Evaluate(tracker)
	// With zero cooldown, next evaluation should fire again
	ae.Evaluate(tracker)

	alerts := ae.RecentAlerts(10)
	if len(alerts) != 2 {
		t.Errorf("alerts = %d, want 2 (no cooldown)", len(alerts))
	}
}

func TestAlertContextThrashing(t *testing.T) {
	config := DefaultAlertConfig()
	config.CompactionStormCount = 3
	ae := NewAlertEvaluator(config, nil)

	tracker := NewSessionTracker()
	s := tracker.GetOrCreate("s1")
	s.CompactionCount = 5

	ae.Evaluate(tracker)

	alerts := ae.RecentAlerts(10)
	found := false
	for _, a := range alerts {
		if a.Type == AlertContextThrashing {
			found = true
		}
	}
	if !found {
		t.Error("expected context_thrashing alert")
	}
}

func TestAlertSessionCostThreshold(t *testing.T) {
	config := DefaultAlertConfig()
	config.SessionCostThresholdUSD = 10.0
	ae := NewAlertEvaluator(config, nil)

	tracker := NewSessionTracker()
	s := tracker.GetOrCreate("s1")
	s.TotalCostUSD = 15.50

	ae.Evaluate(tracker)

	alerts := ae.RecentAlerts(10)
	found := false
	for _, a := range alerts {
		if a.Type == AlertSessionCost {
			found = true
		}
	}
	if !found {
		t.Error("expected session_cost_threshold alert")
	}
}

func TestAlertFireFromEventRateLimit(t *testing.T) {
	ae := NewAlertEvaluator(DefaultAlertConfig(), nil)
	ae.FireFromEvent("s1", "StopFailure", "rate_limit")

	alerts := ae.RecentAlerts(10)
	if len(alerts) != 1 {
		t.Fatalf("alerts = %d, want 1", len(alerts))
	}
	if alerts[0].Type != AlertRateLimitHit {
		t.Errorf("Type = %q", alerts[0].Type)
	}
	if alerts[0].Level != "error" {
		t.Errorf("Level = %q", alerts[0].Level)
	}
}

func TestAlertFireFromEventBillingError(t *testing.T) {
	ae := NewAlertEvaluator(DefaultAlertConfig(), nil)
	ae.FireFromEvent("s1", "StopFailure", "billing_error")

	alerts := ae.RecentAlerts(10)
	if len(alerts) != 1 {
		t.Fatalf("alerts = %d, want 1", len(alerts))
	}
	if alerts[0].Type != AlertBillingError {
		t.Errorf("Type = %q", alerts[0].Type)
	}
}

func TestAlertRingBuffer(t *testing.T) {
	config := DefaultAlertConfig()
	config.CooldownMinutes = 0
	config.BurnRateThresholdTPM = 1
	ae := NewAlertEvaluator(config, nil)

	tracker := NewSessionTracker()
	for i := 0; i < 150; i++ {
		s := tracker.GetOrCreate("s1")
		s.BurnRateTPM = 100
		ae.Evaluate(tracker)
	}

	alerts := ae.RecentAlerts(0)
	if len(alerts) != 100 {
		t.Errorf("ring buffer = %d, want max 100", len(alerts))
	}
}

func TestRecentAlertsOrder(t *testing.T) {
	config := DefaultAlertConfig()
	config.CooldownMinutes = 0
	ae := NewAlertEvaluator(config, nil)

	ae.FireFromEvent("s1", "StopFailure", "rate_limit")
	time.Sleep(1 * time.Millisecond)
	ae.FireFromEvent("s2", "StopFailure", "billing_error")

	alerts := ae.RecentAlerts(10)
	if len(alerts) != 2 {
		t.Fatalf("alerts = %d, want 2", len(alerts))
	}
	// Most recent first
	if alerts[0].Type != AlertBillingError {
		t.Errorf("first alert should be most recent (billing_error), got %q", alerts[0].Type)
	}
}

func TestAlertInactiveSessions(t *testing.T) {
	config := DefaultAlertConfig()
	config.BurnRateThresholdTPM = 1000
	ae := NewAlertEvaluator(config, nil)

	tracker := NewSessionTracker()
	s := tracker.GetOrCreate("s1")
	s.BurnRateTPM = 2000
	s.IsActive = false // inactive sessions should not trigger alerts

	ae.Evaluate(tracker)

	alerts := ae.RecentAlerts(10)
	if len(alerts) != 0 {
		t.Errorf("alerts = %d, want 0 (inactive session)", len(alerts))
	}
}
