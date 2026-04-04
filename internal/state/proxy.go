package state

import "fmt"

// APIRequest represents a logged API request from the proxy.
type APIRequest struct {
	ID                  int64
	Timestamp           string
	RequestID           string
	SessionID           string
	Model               string
	InputTokens         int
	OutputTokens        int
	CacheReadTokens     int
	CacheCreationTokens int
	CostUSD             float64
	LatencyMS           int64
	StopReason          string
	Streaming           bool
	Endpoint            string
	Error               string
	MicrocompactCount   int
	TruncatedResults    int
	TotalToolResultSize int
}

// InsertAPIRequest logs an API request to the database.
func (s *DB) InsertAPIRequest(r APIRequest) error {
	_, err := s.db.Exec(`INSERT INTO api_requests (
		request_id, session_id, model, input_tokens, output_tokens,
		cache_read_tokens, cache_creation_tokens, cost_usd, latency_ms,
		stop_reason, streaming, endpoint, error,
		microcompact_count, truncated_results, total_tool_result_size
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		r.RequestID, r.SessionID, r.Model, r.InputTokens, r.OutputTokens,
		r.CacheReadTokens, r.CacheCreationTokens, r.CostUSD, r.LatencyMS,
		r.StopReason, r.Streaming, r.Endpoint, r.Error,
		r.MicrocompactCount, r.TruncatedResults, r.TotalToolResultSize,
	)
	if err != nil {
		return fmt.Errorf("inserting API request: %w", err)
	}
	return nil
}

// QueryRecentRequests returns the most recent N API requests.
func (s *DB) QueryRecentRequests(limit int) ([]APIRequest, error) {
	rows, err := s.db.Query(`SELECT
		id, timestamp, COALESCE(request_id, ''), COALESCE(session_id, ''),
		COALESCE(model, ''),
		input_tokens, output_tokens, cache_read_tokens, cache_creation_tokens,
		cost_usd, latency_ms, COALESCE(stop_reason, ''), streaming,
		COALESCE(endpoint, ''), COALESCE(error, ''),
		microcompact_count, truncated_results, total_tool_result_size
	FROM api_requests ORDER BY id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("querying recent requests: %w", err)
	}
	defer rows.Close()

	var results []APIRequest
	for rows.Next() {
		var r APIRequest
		if err := rows.Scan(
			&r.ID, &r.Timestamp, &r.RequestID, &r.SessionID, &r.Model,
			&r.InputTokens, &r.OutputTokens, &r.CacheReadTokens, &r.CacheCreationTokens,
			&r.CostUSD, &r.LatencyMS, &r.StopReason, &r.Streaming, &r.Endpoint,
			&r.Error, &r.MicrocompactCount, &r.TruncatedResults, &r.TotalToolResultSize,
		); err != nil {
			return nil, fmt.Errorf("scanning row: %w", err)
		}
		results = append(results, r)
	}
	return results, rows.Err()
}

// ProxyStats holds aggregate proxy statistics.
type ProxyStats struct {
	TotalRequests      int
	TotalInputTokens   int64
	TotalOutputTokens  int64
	TotalCacheRead     int64
	TotalCacheCreation int64
	TotalCostUSD       float64
	AvgLatencyMS       float64
	CacheHitRate       float64
	TotalMicrocompacts int
	TotalTruncated     int
}

// GetProxyStats returns aggregate statistics since the given timestamp.
// If sessionID is non-empty, results are filtered to that session.
func (s *DB) GetProxyStats(since, sessionID string) (*ProxyStats, error) {
	query := `SELECT
		COUNT(*),
		COALESCE(SUM(input_tokens), 0),
		COALESCE(SUM(output_tokens), 0),
		COALESCE(SUM(cache_read_tokens), 0),
		COALESCE(SUM(cache_creation_tokens), 0),
		COALESCE(SUM(cost_usd), 0),
		COALESCE(AVG(latency_ms), 0),
		COALESCE(SUM(microcompact_count), 0),
		COALESCE(SUM(truncated_results), 0)
	FROM api_requests WHERE timestamp >= ?`
	args := []any{since}

	if sessionID != "" {
		query += ` AND session_id = ?`
		args = append(args, sessionID)
	}

	var stats ProxyStats
	err := s.db.QueryRow(query, args...).Scan(
		&stats.TotalRequests,
		&stats.TotalInputTokens,
		&stats.TotalOutputTokens,
		&stats.TotalCacheRead,
		&stats.TotalCacheCreation,
		&stats.TotalCostUSD,
		&stats.AvgLatencyMS,
		&stats.TotalMicrocompacts,
		&stats.TotalTruncated,
	)
	if err != nil {
		return nil, fmt.Errorf("querying proxy stats: %w", err)
	}

	totalInput := stats.TotalInputTokens + stats.TotalCacheRead
	if totalInput > 0 {
		stats.CacheHitRate = float64(stats.TotalCacheRead) / float64(totalInput) * 100
	}

	return &stats, nil
}

// ModelStats holds per-model statistics.
type ModelStats struct {
	Model    string
	Requests int
	CostUSD  float64
	Tokens   int64
}

// GetProxyStatsByModel returns per-model statistics since the given timestamp.
func (s *DB) GetProxyStatsByModel(since, sessionID string) ([]ModelStats, error) {
	query := `SELECT
		COALESCE(model, 'unknown'), COUNT(*), COALESCE(SUM(cost_usd), 0),
		COALESCE(SUM(input_tokens + output_tokens + cache_read_tokens), 0)
	FROM api_requests WHERE timestamp >= ?`
	args := []any{since}

	if sessionID != "" {
		query += ` AND session_id = ?`
		args = append(args, sessionID)
	}

	query += ` GROUP BY model ORDER BY SUM(cost_usd) DESC`

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("querying model stats: %w", err)
	}
	defer rows.Close()

	var results []ModelStats
	for rows.Next() {
		var m ModelStats
		if err := rows.Scan(&m.Model, &m.Requests, &m.CostUSD, &m.Tokens); err != nil {
			return nil, fmt.Errorf("scanning model stats: %w", err)
		}
		results = append(results, m)
	}
	return results, rows.Err()
}
