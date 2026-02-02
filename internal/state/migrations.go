package state

import "fmt"

// migrations is the ordered list of schema migrations.
// Each migration is a function that runs SQL statements.
var migrations = []func(*DB) error{
	migrateV1,
}

func (s *DB) migrate() error {
	// Create schema_version table if it doesn't exist.
	if _, err := s.db.Exec(`CREATE TABLE IF NOT EXISTS schema_version (version INTEGER PRIMARY KEY)`); err != nil {
		return fmt.Errorf("creating schema_version table: %w", err)
	}

	current := s.schemaVersion()

	for i := current; i < len(migrations); i++ {
		if err := migrations[i](s); err != nil {
			return fmt.Errorf("migration v%d: %w", i+1, err)
		}
		if _, err := s.db.Exec(`INSERT OR REPLACE INTO schema_version (version) VALUES (?)`, i+1); err != nil {
			return fmt.Errorf("updating schema version to %d: %w", i+1, err)
		}
	}

	return nil
}

func (s *DB) schemaVersion() int {
	var version int
	err := s.db.QueryRow(`SELECT COALESCE(MAX(version), 0) FROM schema_version`).Scan(&version)
	if err != nil {
		return 0
	}
	return version
}

// SchemaVersion returns the current schema version. Exported for testing.
func (s *DB) SchemaVersion() int {
	return s.schemaVersion()
}

func migrateV1(s *DB) error {
	stmts := []string{
		`CREATE TABLE channels (
			id TEXT PRIMARY KEY,
			adapter TEXT NOT NULL,
			name TEXT NOT NULL,
			config TEXT NOT NULL DEFAULT '{}',
			enabled BOOLEAN NOT NULL DEFAULT 1,
			created_at TEXT NOT NULL,
			last_used_at TEXT
		)`,
		`CREATE TABLE rules (
			id TEXT PRIMARY KEY,
			dsl TEXT NOT NULL,
			scope TEXT NOT NULL,
			enabled BOOLEAN NOT NULL DEFAULT 1,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		)`,
		`CREATE TABLE events (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			session_id TEXT NOT NULL,
			hook_event TEXT NOT NULL,
			rule_id TEXT,
			tool_name TEXT,
			action_taken TEXT,
			duration_ms INTEGER,
			timestamp TEXT NOT NULL DEFAULT (datetime('now'))
		)`,
		`CREATE TABLE sessions (
			session_id TEXT PRIMARY KEY,
			project_dir TEXT,
			started_at TEXT,
			ended_at TEXT,
			event_count INTEGER DEFAULT 0,
			files_modified TEXT,
			summary TEXT
		)`,
		`CREATE TABLE kv_state (
			key TEXT NOT NULL,
			value TEXT,
			session_id TEXT NOT NULL DEFAULT '',
			expires_at TEXT,
			PRIMARY KEY (key, session_id)
		)`,
		`CREATE TABLE mute (
			id INTEGER PRIMARY KEY CHECK (id = 1),
			muted_until TEXT
		)`,
		`CREATE INDEX idx_events_session ON events(session_id)`,
		`CREATE INDEX idx_events_timestamp ON events(timestamp)`,
		`CREATE INDEX idx_events_hook_event ON events(hook_event)`,
		`CREATE INDEX idx_rules_scope ON rules(scope)`,
		`CREATE INDEX idx_kv_session ON kv_state(session_id)`,
		`CREATE INDEX idx_kv_expires ON kv_state(expires_at)`,
	}

	for _, stmt := range stmts {
		if _, err := s.db.Exec(stmt); err != nil {
			return fmt.Errorf("executing %q: %w", stmt[:40], err)
		}
	}

	return nil
}
