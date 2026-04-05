package daemon

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/BrenanL/hitch/internal/proxy"
	"github.com/BrenanL/hitch/internal/state"
)

// HealthResponse is returned by GET /health.
type HealthResponse struct {
	Status          string `json:"status"`
	UptimeSeconds   int64  `json:"uptime_seconds"`
	Port            int    `json:"port"`
	TrackedSessions int    `json:"tracked_sessions"`
	ActiveSessions  int    `json:"active_sessions"`
	PollCount       int64  `json:"poll_count"`
}

// SessionSummary is the JSON representation for session list endpoints.
type SessionSummary struct {
	SessionID    string  `json:"session_id"`
	ProjectDir   string  `json:"project_dir,omitempty"`
	Model        string  `json:"model"`
	IsActive     bool    `json:"is_active"`
	BurnRateTPM  float64 `json:"burn_rate_tpm"`
	TotalTokens  int     `json:"total_tokens"`
	TotalCostUSD float64 `json:"total_cost_usd"`
	LastActivity string  `json:"last_activity"`
	RequestCount int     `json:"request_count"`
}

// SessionDetail extends SessionSummary with full metrics.
type SessionDetail struct {
	SessionSummary
	TotalInputTokens  int              `json:"total_input_tokens"`
	TotalOutputTokens int              `json:"total_output_tokens"`
	TotalCacheRead    int              `json:"total_cache_read"`
	CompactionCount   int              `json:"compaction_count"`
	ActiveSubagents   []SubagentInfo   `json:"active_subagents,omitempty"`
	BurnRateSamples   []BurnPointJSON  `json:"burn_rate_samples,omitempty"`
}

// SubagentInfo is the JSON representation of a subagent.
type SubagentInfo struct {
	AgentID   string  `json:"agent_id"`
	AgentType string  `json:"agent_type,omitempty"`
	Model     string  `json:"model,omitempty"`
	StartedAt string  `json:"started_at"`
	StoppedAt *string `json:"stopped_at,omitempty"`
}

// BurnPointJSON is a single burn rate data point for sparkline rendering.
type BurnPointJSON struct {
	Timestamp string  `json:"ts"`
	TokensTPM float64 `json:"tpm"`
}

// StatsResponse is returned by GET /api/stats.
type StatsResponse struct {
	UptimeSeconds        int64 `json:"uptime_seconds"`
	ActiveSessions       int   `json:"active_sessions"`
	TotalTrackedSessions int   `json:"total_tracked_sessions"`
	PollCount            int64 `json:"poll_count"`
}

func (d *Daemon) handleHealth(w http.ResponseWriter, r *http.Request) {
	total, active := d.Tracker.Count()
	writeJSON(w, HealthResponse{
		Status:          "ok",
		UptimeSeconds:   int64(time.Since(d.startTime).Seconds()),
		Port:            d.port,
		TrackedSessions: total,
		ActiveSessions:  active,
		PollCount:       d.pollCount.Load(),
	})
}

func (d *Daemon) handleSessions(w http.ResponseWriter, r *http.Request) {
	activeOnly := r.URL.Query().Get("active") == "true"
	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = n
		}
	}

	sessions := d.Tracker.ActiveSessions(activeOnly, limit)
	summaries := make([]SessionSummary, len(sessions))
	for i, s := range sessions {
		summaries[i] = toSummary(&s)
	}
	writeJSON(w, summaries)
}

func (d *Daemon) handleSessionByID(w http.ResponseWriter, r *http.Request) {
	// Extract session ID and sub-path from: /api/sessions/<id>[/<sub>]
	path := strings.TrimPrefix(r.URL.Path, "/api/sessions/")
	parts := strings.SplitN(path, "/", 2)
	id := parts[0]
	if id == "" {
		http.Error(w, "session ID required", http.StatusBadRequest)
		return
	}

	// Route to sub-endpoints
	if len(parts) > 1 {
		switch parts[1] {
		case "events":
			d.handleSessionEvents(w, r, id)
			return
		case "stream":
			d.handleSessionStream(w, r, id)
			return
		case "last-request":
			d.handleSessionLastRequest(w, r, id)
			return
		}
	}

	s := d.Tracker.Get(id)
	if s == nil {
		http.Error(w, "session not found", http.StatusNotFound)
		return
	}

	detail := toDetail(s)
	writeJSON(w, detail)
}

func (d *Daemon) handleSessionEvents(w http.ResponseWriter, r *http.Request, sessionID string) {
	limit := 100
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = n
		}
	}

	filter := state.EventFilter{
		SessionID: sessionID,
		Since:     r.URL.Query().Get("since"),
		Limit:     limit,
	}

	if hookEvent := r.URL.Query().Get("source"); hookEvent != "" {
		filter.HookEvent = hookEvent
	}

	if d.db == nil {
		writeJSON(w, []DaemonEvent{})
		return
	}

	events, err := d.db.EventQuery(filter)
	if err != nil {
		log.Printf("[daemon] event query error: %v", err)
		writeJSON(w, []DaemonEvent{})
		return
	}

	result := make([]DaemonEvent, len(events))
	for i, e := range events {
		result[i] = DaemonEvent{
			Timestamp:   e.Timestamp,
			Source:      "hooks",
			EventType:   e.HookEvent,
			SessionID:   e.SessionID,
			Description: formatEventDescription(e),
		}
	}
	writeJSON(w, result)
}

func (d *Daemon) handleSessionStream(w http.ResponseWriter, r *http.Request, sessionID string) {
	d.serveSSE(w, r, sessionID)
}

func (d *Daemon) handleSessionLastRequest(w http.ResponseWriter, r *http.Request, sessionID string) {
	s := d.Tracker.Get(sessionID)
	if s == nil {
		http.Error(w, "session not found", http.StatusNotFound)
		return
	}

	if s.LastRequestID == "" {
		http.Error(w, "no requests for session", http.StatusNotFound)
		return
	}

	// Find the request log path from the database
	if d.db == nil {
		http.Error(w, "database not available", http.StatusServiceUnavailable)
		return
	}

	requests, err := d.db.QueryRecentRequests(1, sessionID)
	if err != nil || len(requests) == 0 {
		http.Error(w, "request not found", http.StatusNotFound)
		return
	}

	reqLogPath := requests[0].RequestLogPath
	if reqLogPath == "" {
		http.Error(w, "no request log available", http.StatusNotFound)
		return
	}

	analysis, err := proxy.AnalyzeRequestBody(reqLogPath)
	if err != nil {
		http.Error(w, "analysis failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, analysis)
}

func (d *Daemon) handleStats(w http.ResponseWriter, r *http.Request) {
	total, active := d.Tracker.Count()
	writeJSON(w, StatsResponse{
		UptimeSeconds:        int64(time.Since(d.startTime).Seconds()),
		ActiveSessions:       active,
		TotalTrackedSessions: total,
		PollCount:            d.pollCount.Load(),
	})
}

func (d *Daemon) handleAlerts(w http.ResponseWriter, r *http.Request) {
	alerts := d.Alerts.RecentAlerts(100)
	if alerts == nil {
		alerts = []Alert{}
	}
	writeJSON(w, alerts)
}

func toSummary(s *ActiveSession) SessionSummary {
	return SessionSummary{
		SessionID:    s.SessionID,
		ProjectDir:   s.ProjectDir,
		Model:        s.Model,
		IsActive:     s.IsActive,
		BurnRateTPM:  s.BurnRateTPM,
		TotalTokens:  s.TotalInputTokens + s.TotalOutputTokens + s.TotalCacheRead,
		TotalCostUSD: s.TotalCostUSD,
		LastActivity: s.LastActivity.Format(time.RFC3339),
		RequestCount: s.RequestCount,
	}
}

func toDetail(s *ActiveSession) SessionDetail {
	detail := SessionDetail{
		SessionSummary:    toSummary(s),
		TotalInputTokens:  s.TotalInputTokens,
		TotalOutputTokens: s.TotalOutputTokens,
		TotalCacheRead:    s.TotalCacheRead,
		CompactionCount:   s.CompactionCount,
	}

	// Subagents
	for _, sa := range s.ActiveSubagents {
		info := SubagentInfo{
			AgentID:   sa.AgentID,
			AgentType: sa.AgentType,
			Model:     sa.Model,
			StartedAt: sa.StartedAt.Format(time.RFC3339),
		}
		if sa.StoppedAt != nil {
			stopped := sa.StoppedAt.Format(time.RFC3339)
			info.StoppedAt = &stopped
		}
		detail.ActiveSubagents = append(detail.ActiveSubagents, info)
	}

	// Burn rate samples for sparkline
	for _, bs := range s.BurnRateSamples {
		detail.BurnRateSamples = append(detail.BurnRateSamples, BurnPointJSON{
			Timestamp: bs.Timestamp.Format(time.RFC3339),
			TokensTPM: float64(bs.TokensTotal),
		})
	}

	return detail
}

func formatEventDescription(e state.Event) string {
	desc := e.HookEvent
	if e.ToolName != "" {
		desc += ": " + e.ToolName
	}
	if e.ActionTaken != "" {
		desc += " (" + e.ActionTaken + ")"
	}
	return desc
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}
