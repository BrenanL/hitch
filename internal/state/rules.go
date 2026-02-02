package state

import (
	"database/sql"
	"fmt"
	"time"
)

// Rule represents a DSL rule stored in the database.
type Rule struct {
	ID        string
	DSL       string
	Scope     string
	Enabled   bool
	CreatedAt string
	UpdatedAt string
}

// RuleAdd adds a new rule.
func (s *DB) RuleAdd(r Rule) error {
	now := time.Now().UTC().Format(time.RFC3339)
	if r.CreatedAt == "" {
		r.CreatedAt = now
	}
	if r.UpdatedAt == "" {
		r.UpdatedAt = now
	}
	_, err := s.db.Exec(
		`INSERT INTO rules (id, dsl, scope, enabled, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`,
		r.ID, r.DSL, r.Scope, r.Enabled, r.CreatedAt, r.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("adding rule: %w", err)
	}
	return nil
}

// RuleList returns all rules.
func (s *DB) RuleList() ([]Rule, error) {
	rows, err := s.db.Query(`SELECT id, dsl, scope, enabled, created_at, updated_at FROM rules ORDER BY created_at`)
	if err != nil {
		return nil, fmt.Errorf("listing rules: %w", err)
	}
	defer rows.Close()

	var rules []Rule
	for rows.Next() {
		var r Rule
		if err := rows.Scan(&r.ID, &r.DSL, &r.Scope, &r.Enabled, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning rule: %w", err)
		}
		rules = append(rules, r)
	}
	return rules, rows.Err()
}

// RuleGet returns a rule by ID.
func (s *DB) RuleGet(id string) (*Rule, error) {
	var r Rule
	err := s.db.QueryRow(
		`SELECT id, dsl, scope, enabled, created_at, updated_at FROM rules WHERE id = ?`, id,
	).Scan(&r.ID, &r.DSL, &r.Scope, &r.Enabled, &r.CreatedAt, &r.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("getting rule: %w", err)
	}
	return &r, nil
}

// RuleRemove deletes a rule by ID.
func (s *DB) RuleRemove(id string) error {
	res, err := s.db.Exec(`DELETE FROM rules WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("removing rule: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("rule %q not found", id)
	}
	return nil
}

// RuleEnable enables a rule.
func (s *DB) RuleEnable(id string) error {
	return s.ruleSetEnabled(id, true)
}

// RuleDisable disables a rule.
func (s *DB) RuleDisable(id string) error {
	return s.ruleSetEnabled(id, false)
}

func (s *DB) ruleSetEnabled(id string, enabled bool) error {
	res, err := s.db.Exec(
		`UPDATE rules SET enabled = ?, updated_at = ? WHERE id = ?`,
		enabled, time.Now().UTC().Format(time.RFC3339), id,
	)
	if err != nil {
		return fmt.Errorf("updating rule: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("rule %q not found", id)
	}
	return nil
}

// RuleListByScope returns rules matching a scope.
func (s *DB) RuleListByScope(scope string) ([]Rule, error) {
	rows, err := s.db.Query(
		`SELECT id, dsl, scope, enabled, created_at, updated_at FROM rules WHERE scope = ? ORDER BY created_at`, scope,
	)
	if err != nil {
		return nil, fmt.Errorf("listing rules by scope: %w", err)
	}
	defer rows.Close()

	var rules []Rule
	for rows.Next() {
		var r Rule
		if err := rows.Scan(&r.ID, &r.DSL, &r.Scope, &r.Enabled, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning rule: %w", err)
		}
		rules = append(rules, r)
	}
	return rules, rows.Err()
}
