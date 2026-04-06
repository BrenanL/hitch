package daemon

import (
	"sync"
	"time"

	"github.com/BrenanL/hitch/internal/metrics"
	"github.com/BrenanL/hitch/internal/pricing"
	"github.com/BrenanL/hitch/internal/state"
)

const burnRateWindow = 5 * time.Minute

// SubagentState tracks a spawned subagent within a session.
type SubagentState struct {
	AgentID      string
	AgentType    string // e.g. "Bash", "Explore", "Plan"
	Model        string
	StartedAt    time.Time
	StoppedAt    *time.Time
	InputTokens  int
	OutputTokens int
}

// ActiveSession holds aggregated metrics for a single Claude Code session.
type ActiveSession struct {
	SessionID    string
	ProjectDir   string
	Model        string
	StartedAt    time.Time
	LastActivity time.Time
	IsActive     bool

	TotalInputTokens    int
	TotalOutputTokens   int
	TotalCacheRead      int
	TotalCacheCreation  int
	TotalCostUSD        float64
	RequestCount        int
	CompactionCount     int

	BurnRateSamples []BurnSample
	BurnRateTPM     float64

	ActiveSubagents []SubagentState
	LastRequestID   string
	LastEventType   string
}

// BurnSample records cumulative token count at a point in time.
type BurnSample struct {
	Timestamp   time.Time
	TokensTotal int
}

// SessionTracker maintains a goroutine-safe map of active sessions.
type SessionTracker struct {
	mu       sync.RWMutex
	sessions map[string]*ActiveSession

	// Track the last-seen IDs to avoid reprocessing rows.
	lastRequestID int64
	lastEventID   int
}

// NewSessionTracker creates an empty tracker.
func NewSessionTracker() *SessionTracker {
	return &SessionTracker{
		sessions: make(map[string]*ActiveSession),
	}
}

// GetOrCreate returns the session for the given ID, creating it if needed.
func (t *SessionTracker) GetOrCreate(sessionID string) *ActiveSession {
	t.mu.Lock()
	defer t.mu.Unlock()

	s, ok := t.sessions[sessionID]
	if !ok {
		now := time.Now()
		s = &ActiveSession{
			SessionID:    sessionID,
			StartedAt:    now,
			LastActivity: now,
			IsActive:     true,
		}
		t.sessions[sessionID] = s
	}
	return s
}

// Get returns the session for the given ID, or nil if not found.
func (t *SessionTracker) Get(sessionID string) *ActiveSession {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.sessions[sessionID]
}

// ActiveSessions returns all sessions sorted by last activity (most recent first).
func (t *SessionTracker) ActiveSessions(activeOnly bool, limit int) []ActiveSession {
	t.mu.RLock()
	defer t.mu.RUnlock()

	result := make([]ActiveSession, 0, len(t.sessions))
	for _, s := range t.sessions {
		if activeOnly && !s.IsActive {
			continue
		}
		result = append(result, *s)
	}

	// Sort by last activity descending
	for i := 0; i < len(result); i++ {
		for j := i + 1; j < len(result); j++ {
			if result[j].LastActivity.After(result[i].LastActivity) {
				result[i], result[j] = result[j], result[i]
			}
		}
	}

	if limit > 0 && len(result) > limit {
		result = result[:limit]
	}
	return result
}

// Count returns total and active session counts.
func (t *SessionTracker) Count() (total, active int) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	for _, s := range t.sessions {
		total++
		if s.IsActive {
			active++
		}
	}
	return
}

// UpdateFromRequest updates session state from an API request row.
func (t *SessionTracker) UpdateFromRequest(req state.APIRequest) {
	s := t.GetOrCreate(req.SessionID)

	t.mu.Lock()
	defer t.mu.Unlock()

	s.LastActivity = time.Now()
	s.IsActive = true
	s.RequestCount++
	s.TotalInputTokens += req.InputTokens
	s.TotalOutputTokens += req.OutputTokens
	s.TotalCacheRead += req.CacheReadTokens
	s.TotalCacheCreation += req.CacheCreationTokens
	s.LastRequestID = req.RequestID

	if req.Model != "" {
		s.Model = req.Model
	}

	// Compute cost using shared pricing package
	p := pricing.LoadPricing()
	s.TotalCostUSD += p.EstimateCost(req.Model, req.InputTokens, req.OutputTokens,
		req.CacheReadTokens, req.CacheCreationTokens)

	// Add burn rate sample and recompute
	cumulative := s.TotalInputTokens + s.TotalOutputTokens + s.TotalCacheRead
	s.BurnRateSamples = append(s.BurnRateSamples, BurnSample{
		Timestamp:   time.Now(),
		TokensTotal: cumulative,
	})
	s.BurnRateTPM = computeBurnRate(s.BurnRateSamples)

	// Track the max request ID we've seen
	if req.ID > t.lastRequestID {
		t.lastRequestID = req.ID
	}
}

// UpdateFromEvent processes a hook event (SubagentStart/Stop, PostCompact, etc.)
func (t *SessionTracker) UpdateFromEvent(evt state.Event) {
	if evt.SessionID == "" {
		return
	}

	s := t.GetOrCreate(evt.SessionID)

	t.mu.Lock()
	defer t.mu.Unlock()

	s.LastActivity = time.Now()
	s.LastEventType = evt.HookEvent

	switch evt.HookEvent {
	case "SubagentStart":
		s.ActiveSubagents = append(s.ActiveSubagents, SubagentState{
			AgentID:   evt.ToolName,
			AgentType: evt.ActionTaken,
			StartedAt: time.Now(),
		})

	case "SubagentStop":
		// Mark the matching subagent as stopped
		now := time.Now()
		for i := range s.ActiveSubagents {
			if s.ActiveSubagents[i].AgentID == evt.ToolName && s.ActiveSubagents[i].StoppedAt == nil {
				s.ActiveSubagents[i].StoppedAt = &now
				break
			}
		}

	case "PostCompact":
		s.CompactionCount++

	case "SessionStart":
		s.IsActive = true
		if evt.ActionTaken != "" {
			s.ProjectDir = evt.ActionTaken
		}

	case "SessionEnd", "Stop":
		s.IsActive = false
	}

	if evt.ID > t.lastEventID {
		t.lastEventID = evt.ID
	}
}

// LastRequestID returns the highest api_requests.id seen by the tracker.
func (t *SessionTracker) LastRequestID() int64 {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.lastRequestID
}

// LastEventID returns the highest events.id seen by the tracker.
func (t *SessionTracker) LastEventID() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.lastEventID
}

// MarkInactive marks sessions as inactive if they haven't had activity
// within the given timeout.
func (t *SessionTracker) MarkInactive(timeout time.Duration) {
	t.mu.Lock()
	defer t.mu.Unlock()

	cutoff := time.Now().Add(-timeout)
	for _, s := range t.sessions {
		if s.IsActive && s.LastActivity.Before(cutoff) {
			s.IsActive = false
		}
	}
}

// Prune removes sessions that have been inactive for longer than the
// given duration.
func (t *SessionTracker) Prune(maxAge time.Duration) {
	t.mu.Lock()
	defer t.mu.Unlock()

	cutoff := time.Now().Add(-maxAge)
	for id, s := range t.sessions {
		if !s.IsActive && s.LastActivity.Before(cutoff) {
			delete(t.sessions, id)
		}
	}
}

// computeBurnRate converts our BurnSamples to metrics.BurnSample and delegates
// to the shared metrics package.
func computeBurnRate(samples []BurnSample) float64 {
	ms := make([]metrics.BurnSample, len(samples))
	for i, s := range samples {
		ms[i] = metrics.BurnSample{
			Timestamp:        s.Timestamp,
			CumulativeTokens: s.TokensTotal,
		}
	}
	return metrics.BurnRate(ms, burnRateWindow)
}
