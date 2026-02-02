package state

import (
	"database/sql"
	"fmt"
	"time"
)

// MuteGet returns the muted_until timestamp, or empty string if not muted.
func (s *DB) MuteGet() (string, error) {
	var mutedUntil sql.NullString
	err := s.db.QueryRow(`SELECT muted_until FROM mute WHERE id = 1`).Scan(&mutedUntil)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("getting mute state: %w", err)
	}
	if !mutedUntil.Valid {
		return "", nil
	}
	return mutedUntil.String, nil
}

// MuteSet sets the muted_until timestamp.
func (s *DB) MuteSet(until string) error {
	_, err := s.db.Exec(
		`INSERT OR REPLACE INTO mute (id, muted_until) VALUES (1, ?)`, until,
	)
	if err != nil {
		return fmt.Errorf("setting mute: %w", err)
	}
	return nil
}

// MuteClear clears the mute state.
func (s *DB) MuteClear() error {
	_, err := s.db.Exec(`DELETE FROM mute WHERE id = 1`)
	if err != nil {
		return fmt.Errorf("clearing mute: %w", err)
	}
	return nil
}

// IsMuted returns true if notifications are currently muted.
func (s *DB) IsMuted() (bool, error) {
	until, err := s.MuteGet()
	if err != nil {
		return false, err
	}
	if until == "" {
		return false, nil
	}
	t, err := time.Parse(time.RFC3339, until)
	if err != nil {
		return false, nil
	}
	return time.Now().UTC().Before(t), nil
}
