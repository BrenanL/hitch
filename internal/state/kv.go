package state

import (
	"database/sql"
	"fmt"
	"time"
)

// KVGet retrieves a value by key and optional session ID.
// Pass empty string for sessionID to get global keys.
// Returns empty string and nil error if not found.
func (s *DB) KVGet(key, sessionID string) (string, error) {
	var value string
	err := s.db.QueryRow(
		`SELECT COALESCE(value, '') FROM kv_state WHERE key = ? AND session_id = ? AND (expires_at IS NULL OR expires_at > ?)`,
		key, sessionID, time.Now().UTC().Format(time.RFC3339),
	).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("getting kv %q: %w", key, err)
	}
	return value, nil
}

// KVSet sets a key-value pair with optional session scope and expiry.
// Pass empty string for sessionID for global keys.
// Pass empty string for expiresAt for no expiry.
func (s *DB) KVSet(key, value, sessionID string, expiresAt string) error {
	var expPtr any
	if expiresAt != "" {
		expPtr = expiresAt
	}

	_, err := s.db.Exec(
		`INSERT OR REPLACE INTO kv_state (key, value, session_id, expires_at) VALUES (?, ?, ?, ?)`,
		key, value, sessionID, expPtr,
	)
	if err != nil {
		return fmt.Errorf("setting kv %q: %w", key, err)
	}
	return nil
}

// KVDelete deletes a key-value pair.
func (s *DB) KVDelete(key, sessionID string) error {
	_, err := s.db.Exec(`DELETE FROM kv_state WHERE key = ? AND session_id = ?`, key, sessionID)
	if err != nil {
		return fmt.Errorf("deleting kv %q: %w", key, err)
	}
	return nil
}

// KVCleanupExpired removes all expired key-value pairs.
func (s *DB) KVCleanupExpired() (int64, error) {
	res, err := s.db.Exec(`DELETE FROM kv_state WHERE expires_at IS NOT NULL AND expires_at <= ?`,
		time.Now().UTC().Format(time.RFC3339))
	if err != nil {
		return 0, fmt.Errorf("cleaning up expired kv: %w", err)
	}
	return res.RowsAffected()
}
