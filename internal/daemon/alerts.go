package daemon

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/BrenanL/hitch/internal/adapters"
)

// AlertType identifies a kind of alert.
type AlertType string

const (
	AlertBurnRateSpike      AlertType = "burn_rate_spike"
	AlertRateLimitHit       AlertType = "rate_limit_hit"
	AlertBillingError       AlertType = "billing_error"
	AlertContextThrashing   AlertType = "context_thrashing"
	AlertProxyUnreachable   AlertType = "proxy_unreachable"
	AlertSessionCost        AlertType = "session_cost_threshold"
	AlertSubagentModelMismatch AlertType = "subagent_model_mismatch"
)

// Alert is a single fired alert.
type Alert struct {
	Timestamp string            `json:"ts"`
	Type      AlertType         `json:"type"`
	SessionID string            `json:"session_id,omitempty"`
	Level     string            `json:"level"` // "warning" or "error"
	Title     string            `json:"title"`
	Body      string            `json:"body"`
	Fields    map[string]string `json:"fields,omitempty"`
}

// AlertConfig holds thresholds for alert evaluation.
type AlertConfig struct {
	BurnRateThresholdTPM       float64
	CompactionStormCount       int
	CompactionStormWindowMin   int
	SessionCostThresholdUSD    float64
	CooldownMinutes            int
}

// DefaultAlertConfig returns sensible defaults.
func DefaultAlertConfig() AlertConfig {
	return AlertConfig{
		BurnRateThresholdTPM:     5000,
		CompactionStormCount:     3,
		CompactionStormWindowMin: 10,
		SessionCostThresholdUSD:  0, // disabled by default
		CooldownMinutes:          10,
	}
}

// AlertEvaluator checks session state against thresholds and fires alerts.
type AlertEvaluator struct {
	mu       sync.Mutex
	config   AlertConfig
	alerts   []Alert // ring buffer of recent alerts (max 100)
	cooldown map[string]time.Time // "type:session_id" -> last fired
	adapters []adapters.Adapter
}

// NewAlertEvaluator creates an evaluator with the given config and adapters.
func NewAlertEvaluator(config AlertConfig, adapters []adapters.Adapter) *AlertEvaluator {
	return &AlertEvaluator{
		config:   config,
		cooldown: make(map[string]time.Time),
		adapters: adapters,
	}
}

// Evaluate checks all active sessions against alert thresholds.
func (ae *AlertEvaluator) Evaluate(tracker *SessionTracker) {
	sessions := tracker.ActiveSessions(true, 0)
	for i := range sessions {
		ae.checkSession(&sessions[i])
	}
}

// RecentAlerts returns the last N alerts (most recent first).
func (ae *AlertEvaluator) RecentAlerts(limit int) []Alert {
	ae.mu.Lock()
	defer ae.mu.Unlock()

	if limit <= 0 || limit > len(ae.alerts) {
		limit = len(ae.alerts)
	}

	// Return in reverse order (most recent first)
	result := make([]Alert, limit)
	for i := 0; i < limit; i++ {
		result[i] = ae.alerts[len(ae.alerts)-1-i]
	}
	return result
}

func (ae *AlertEvaluator) checkSession(s *ActiveSession) {
	// Burn rate spike
	if ae.config.BurnRateThresholdTPM > 0 && s.BurnRateTPM > ae.config.BurnRateThresholdTPM {
		ae.fire(Alert{
			Type:      AlertBurnRateSpike,
			SessionID: s.SessionID,
			Level:     "warning",
			Title:     fmt.Sprintf("Burn rate spike: %s", s.SessionID),
			Body:      fmt.Sprintf("%.0f tok/min exceeds threshold of %.0f in %s", s.BurnRateTPM, ae.config.BurnRateThresholdTPM, s.ProjectDir),
			Fields:    map[string]string{"burn_rate": fmt.Sprintf("%.0f", s.BurnRateTPM), "threshold": fmt.Sprintf("%.0f", ae.config.BurnRateThresholdTPM)},
		})
	}

	// Context thrashing (compaction storm)
	if ae.config.CompactionStormCount > 0 && s.CompactionCount >= ae.config.CompactionStormCount {
		ae.fire(Alert{
			Type:      AlertContextThrashing,
			SessionID: s.SessionID,
			Level:     "warning",
			Title:     fmt.Sprintf("Context thrashing: %s", s.SessionID),
			Body:      fmt.Sprintf("%d compactions in session %s", s.CompactionCount, s.ProjectDir),
			Fields:    map[string]string{"compaction_count": fmt.Sprintf("%d", s.CompactionCount)},
		})
	}

	// Session cost threshold
	if ae.config.SessionCostThresholdUSD > 0 && s.TotalCostUSD > ae.config.SessionCostThresholdUSD {
		ae.fire(Alert{
			Type:      AlertSessionCost,
			SessionID: s.SessionID,
			Level:     "warning",
			Title:     fmt.Sprintf("Session cost threshold: %s", s.SessionID),
			Body:      fmt.Sprintf("$%.2f exceeds threshold of $%.2f in %s", s.TotalCostUSD, ae.config.SessionCostThresholdUSD, s.ProjectDir),
			Fields:    map[string]string{"cost": fmt.Sprintf("%.2f", s.TotalCostUSD), "threshold": fmt.Sprintf("%.2f", ae.config.SessionCostThresholdUSD)},
		})
	}
}

// FireFromEvent processes a hook event for event-triggered alerts.
func (ae *AlertEvaluator) FireFromEvent(sessionID, hookEvent, actionTaken string) {
	switch hookEvent {
	case "StopFailure":
		if actionTaken == "rate_limit" {
			ae.fire(Alert{
				Type:      AlertRateLimitHit,
				SessionID: sessionID,
				Level:     "error",
				Title:     fmt.Sprintf("Rate limit hit: %s", sessionID),
				Body:      actionTaken,
				Fields:    map[string]string{"error_message": actionTaken},
			})
		} else if actionTaken == "billing_error" {
			ae.fire(Alert{
				Type:      AlertBillingError,
				SessionID: sessionID,
				Level:     "error",
				Title:     fmt.Sprintf("Billing error: %s", sessionID),
				Body:      actionTaken,
				Fields:    map[string]string{"error_message": actionTaken},
			})
		}
	}
}

func (ae *AlertEvaluator) fire(alert Alert) {
	ae.mu.Lock()
	defer ae.mu.Unlock()

	// Deduplication check
	key := string(alert.Type) + ":" + alert.SessionID
	cooldown := time.Duration(ae.config.CooldownMinutes) * time.Minute
	if last, ok := ae.cooldown[key]; ok && time.Since(last) < cooldown {
		return // within cooldown window
	}

	alert.Timestamp = time.Now().Format(time.RFC3339)
	ae.cooldown[key] = time.Now()

	// Store in ring buffer (max 100)
	ae.alerts = append(ae.alerts, alert)
	if len(ae.alerts) > 100 {
		ae.alerts = ae.alerts[len(ae.alerts)-100:]
	}

	// Dispatch to adapters
	for _, adapter := range ae.adapters {
		msg := adapters.Message{
			Title: alert.Title,
			Body:  alert.Body,
			Event: string(alert.Type),
		}
		if alert.Level == "error" {
			msg.Level = adapters.Error
		} else {
			msg.Level = adapters.Warning
		}
		result := adapter.Send(context.Background(), msg)
		if !result.Success {
			log.Printf("[alerts] dispatch to %s failed: %v", adapter.Name(), result.Error)
		}
	}
}
