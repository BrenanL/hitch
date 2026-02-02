package state

import (
	"database/sql"
	"fmt"
	"time"
)

// Channel represents a configured notification channel.
type Channel struct {
	ID         string
	Adapter    string
	Name       string
	Config     string
	Enabled    bool
	CreatedAt  string
	LastUsedAt string
}

// ChannelAdd adds a new channel.
func (s *DB) ChannelAdd(ch Channel) error {
	if ch.CreatedAt == "" {
		ch.CreatedAt = time.Now().UTC().Format(time.RFC3339)
	}
	_, err := s.db.Exec(
		`INSERT INTO channels (id, adapter, name, config, enabled, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
		ch.ID, ch.Adapter, ch.Name, ch.Config, ch.Enabled, ch.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("adding channel: %w", err)
	}
	return nil
}

// ChannelList returns all channels.
func (s *DB) ChannelList() ([]Channel, error) {
	rows, err := s.db.Query(`SELECT id, adapter, name, config, enabled, created_at, COALESCE(last_used_at, '') FROM channels ORDER BY created_at`)
	if err != nil {
		return nil, fmt.Errorf("listing channels: %w", err)
	}
	defer rows.Close()

	var channels []Channel
	for rows.Next() {
		var ch Channel
		if err := rows.Scan(&ch.ID, &ch.Adapter, &ch.Name, &ch.Config, &ch.Enabled, &ch.CreatedAt, &ch.LastUsedAt); err != nil {
			return nil, fmt.Errorf("scanning channel: %w", err)
		}
		channels = append(channels, ch)
	}
	return channels, rows.Err()
}

// ChannelGet returns a channel by ID.
func (s *DB) ChannelGet(id string) (*Channel, error) {
	var ch Channel
	err := s.db.QueryRow(
		`SELECT id, adapter, name, config, enabled, created_at, COALESCE(last_used_at, '') FROM channels WHERE id = ?`, id,
	).Scan(&ch.ID, &ch.Adapter, &ch.Name, &ch.Config, &ch.Enabled, &ch.CreatedAt, &ch.LastUsedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("getting channel: %w", err)
	}
	return &ch, nil
}

// ChannelRemove deletes a channel by ID.
func (s *DB) ChannelRemove(id string) error {
	res, err := s.db.Exec(`DELETE FROM channels WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("removing channel: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("channel %q not found", id)
	}
	return nil
}

// ChannelUpdateLastUsed sets the last_used_at timestamp for a channel.
func (s *DB) ChannelUpdateLastUsed(id string) error {
	_, err := s.db.Exec(
		`UPDATE channels SET last_used_at = ? WHERE id = ?`,
		time.Now().UTC().Format(time.RFC3339), id,
	)
	return err
}
