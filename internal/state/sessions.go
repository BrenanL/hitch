package state

import (
	"database/sql"
	"fmt"
	"time"
)

// Session represents a Claude Code session.
type Session struct {
	SessionID     string
	ProjectDir    string
	StartedAt     string
	EndedAt       string
	EventCount    int
	FilesModified string
	Summary       string
}

// SessionCreate creates or updates a session record.
func (s *DB) SessionCreate(sess Session) error {
	if sess.StartedAt == "" {
		sess.StartedAt = time.Now().UTC().Format(time.RFC3339)
	}
	_, err := s.db.Exec(
		`INSERT OR REPLACE INTO sessions (session_id, project_dir, started_at, ended_at, event_count, files_modified, summary)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		sess.SessionID, nilIfEmpty(sess.ProjectDir), sess.StartedAt,
		nilIfEmpty(sess.EndedAt), sess.EventCount,
		nilIfEmpty(sess.FilesModified), nilIfEmpty(sess.Summary),
	)
	if err != nil {
		return fmt.Errorf("creating session: %w", err)
	}
	return nil
}

// SessionUpdate updates session fields.
func (s *DB) SessionUpdate(sess Session) error {
	_, err := s.db.Exec(
		`UPDATE sessions SET ended_at = ?, event_count = ?, files_modified = ?, summary = ? WHERE session_id = ?`,
		nilIfEmpty(sess.EndedAt), sess.EventCount,
		nilIfEmpty(sess.FilesModified), nilIfEmpty(sess.Summary), sess.SessionID,
	)
	if err != nil {
		return fmt.Errorf("updating session: %w", err)
	}
	return nil
}

// SessionGet returns a session by ID.
func (s *DB) SessionGet(sessionID string) (*Session, error) {
	var sess Session
	err := s.db.QueryRow(
		`SELECT session_id, COALESCE(project_dir, ''), COALESCE(started_at, ''), COALESCE(ended_at, ''), COALESCE(event_count, 0), COALESCE(files_modified, ''), COALESCE(summary, '') FROM sessions WHERE session_id = ?`,
		sessionID,
	).Scan(&sess.SessionID, &sess.ProjectDir, &sess.StartedAt, &sess.EndedAt, &sess.EventCount, &sess.FilesModified, &sess.Summary)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("getting session: %w", err)
	}
	return &sess, nil
}

// SessionIncrementEventCount increments the event count for a session.
func (s *DB) SessionIncrementEventCount(sessionID string) error {
	_, err := s.db.Exec(
		`UPDATE sessions SET event_count = event_count + 1 WHERE session_id = ?`,
		sessionID,
	)
	return err
}
