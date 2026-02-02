package state

import (
	"fmt"
	"time"
)

// Event represents a hook event log entry.
type Event struct {
	ID          int
	SessionID   string
	HookEvent   string
	RuleID      string
	ToolName    string
	ActionTaken string
	DurationMs  int
	Timestamp   string
}

// EventLog inserts a new event.
func (s *DB) EventLog(e Event) error {
	if e.Timestamp == "" {
		e.Timestamp = time.Now().UTC().Format(time.RFC3339)
	}
	_, err := s.db.Exec(
		`INSERT INTO events (session_id, hook_event, rule_id, tool_name, action_taken, duration_ms, timestamp) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		e.SessionID, e.HookEvent, nilIfEmpty(e.RuleID), nilIfEmpty(e.ToolName), nilIfEmpty(e.ActionTaken), e.DurationMs, e.Timestamp,
	)
	if err != nil {
		return fmt.Errorf("logging event: %w", err)
	}
	return nil
}

// EventQuery returns events matching the given filters.
type EventFilter struct {
	SessionID string
	HookEvent string
	Since     string // ISO timestamp
	Limit     int
}

// EventQuery returns events matching the given filter.
func (s *DB) EventQuery(f EventFilter) ([]Event, error) {
	query := `SELECT id, session_id, hook_event, COALESCE(rule_id, ''), COALESCE(tool_name, ''), COALESCE(action_taken, ''), COALESCE(duration_ms, 0), timestamp FROM events WHERE 1=1`
	var args []any

	if f.SessionID != "" {
		query += ` AND session_id = ?`
		args = append(args, f.SessionID)
	}
	if f.HookEvent != "" {
		query += ` AND hook_event = ?`
		args = append(args, f.HookEvent)
	}
	if f.Since != "" {
		query += ` AND timestamp >= ?`
		args = append(args, f.Since)
	}

	query += ` ORDER BY timestamp DESC`

	if f.Limit > 0 {
		query += fmt.Sprintf(` LIMIT %d`, f.Limit)
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("querying events: %w", err)
	}
	defer rows.Close()

	var events []Event
	for rows.Next() {
		var e Event
		if err := rows.Scan(&e.ID, &e.SessionID, &e.HookEvent, &e.RuleID, &e.ToolName, &e.ActionTaken, &e.DurationMs, &e.Timestamp); err != nil {
			return nil, fmt.Errorf("scanning event: %w", err)
		}
		events = append(events, e)
	}
	return events, rows.Err()
}

func nilIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}
