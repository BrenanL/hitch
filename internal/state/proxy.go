package state

import "fmt"

// APIRequest represents a logged API request from the proxy.
type APIRequest struct {
	ID                  int64
	Timestamp           string
	RequestID           string
	SessionID           string
	Model               string
	HTTPMethod          string
	HTTPStatus          int
	InputTokens         int
	OutputTokens        int
	CacheReadTokens     int
	CacheCreationTokens int
	LatencyMS           int64
	StopReason          string
	Streaming           bool
	Endpoint            string
	Error               string
	MicrocompactCount   int
	TruncatedResults    int
	TotalToolResultSize int
	RequestHeaders      string
	ResponseHeaders     string
	RequestBodySize     int64
	ResponseBodySize    int64
	RequestLogPath      string
	ResponseLogPath     string
	MessageCount        int
}

// InsertAPIRequest logs an API request to the database.
func (s *DB) InsertAPIRequest(r APIRequest) error {
	_, err := s.db.Exec(`INSERT INTO api_requests (
		request_id, session_id, model, http_method, http_status,
		input_tokens, output_tokens, cache_read_tokens, cache_creation_tokens,
		latency_ms, stop_reason, streaming, endpoint, error,
		microcompact_count, truncated_results, total_tool_result_size,
		request_headers, response_headers,
		request_body_size, response_body_size,
		request_log_path, response_log_path, message_count
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		r.RequestID, r.SessionID, r.Model, r.HTTPMethod, r.HTTPStatus,
		r.InputTokens, r.OutputTokens, r.CacheReadTokens, r.CacheCreationTokens,
		r.LatencyMS, r.StopReason, r.Streaming, r.Endpoint, r.Error,
		r.MicrocompactCount, r.TruncatedResults, r.TotalToolResultSize,
		r.RequestHeaders, r.ResponseHeaders,
		r.RequestBodySize, r.ResponseBodySize,
		r.RequestLogPath, r.ResponseLogPath, r.MessageCount,
	)
	if err != nil {
		return fmt.Errorf("inserting API request: %w", err)
	}
	return nil
}

// QueryRecentRequests returns the most recent N API requests.
// If sessionID is non-empty, filters to that session.
func (s *DB) QueryRecentRequests(limit int, sessionID string) ([]APIRequest, error) {
	query := `SELECT
		id, timestamp, COALESCE(request_id, ''), COALESCE(session_id, ''),
		COALESCE(model, ''), COALESCE(http_method, ''), http_status,
		input_tokens, output_tokens, cache_read_tokens, cache_creation_tokens,
		latency_ms, COALESCE(stop_reason, ''), streaming,
		COALESCE(endpoint, ''), COALESCE(error, ''),
		microcompact_count, truncated_results, total_tool_result_size,
		COALESCE(request_headers, ''), COALESCE(response_headers, ''),
		request_body_size, response_body_size,
		COALESCE(request_log_path, ''), COALESCE(response_log_path, ''),
		message_count
	FROM api_requests`
	args := []any{}

	if sessionID != "" {
		query += ` WHERE session_id = ?`
		args = append(args, sessionID)
	}
	query += ` ORDER BY id DESC LIMIT ?`
	args = append(args, limit)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("querying recent requests: %w", err)
	}
	defer rows.Close()

	var results []APIRequest
	for rows.Next() {
		var r APIRequest
		if err := rows.Scan(
			&r.ID, &r.Timestamp, &r.RequestID, &r.SessionID,
			&r.Model, &r.HTTPMethod, &r.HTTPStatus,
			&r.InputTokens, &r.OutputTokens, &r.CacheReadTokens, &r.CacheCreationTokens,
			&r.LatencyMS, &r.StopReason, &r.Streaming, &r.Endpoint, &r.Error,
			&r.MicrocompactCount, &r.TruncatedResults, &r.TotalToolResultSize,
			&r.RequestHeaders, &r.ResponseHeaders,
			&r.RequestBodySize, &r.ResponseBodySize,
			&r.RequestLogPath, &r.ResponseLogPath,
			&r.MessageCount,
		); err != nil {
			return nil, fmt.Errorf("scanning row: %w", err)
		}
		results = append(results, r)
	}
	return results, rows.Err()
}

// GetRequest returns a single request by ID.
func (s *DB) GetRequest(id int64) (*APIRequest, error) {
	var r APIRequest
	err := s.db.QueryRow(`SELECT
		id, timestamp, COALESCE(request_id, ''), COALESCE(session_id, ''),
		COALESCE(model, ''), COALESCE(http_method, ''), http_status,
		input_tokens, output_tokens, cache_read_tokens, cache_creation_tokens,
		latency_ms, COALESCE(stop_reason, ''), streaming,
		COALESCE(endpoint, ''), COALESCE(error, ''),
		microcompact_count, truncated_results, total_tool_result_size,
		COALESCE(request_headers, ''), COALESCE(response_headers, ''),
		request_body_size, response_body_size,
		COALESCE(request_log_path, ''), COALESCE(response_log_path, ''),
		message_count
	FROM api_requests WHERE id = ?`, id).Scan(
		&r.ID, &r.Timestamp, &r.RequestID, &r.SessionID,
		&r.Model, &r.HTTPMethod, &r.HTTPStatus,
		&r.InputTokens, &r.OutputTokens, &r.CacheReadTokens, &r.CacheCreationTokens,
		&r.LatencyMS, &r.StopReason, &r.Streaming, &r.Endpoint, &r.Error,
		&r.MicrocompactCount, &r.TruncatedResults, &r.TotalToolResultSize,
		&r.RequestHeaders, &r.ResponseHeaders,
		&r.RequestBodySize, &r.ResponseBodySize,
		&r.RequestLogPath, &r.ResponseLogPath,
		&r.MessageCount,
	)
	if err != nil {
		return nil, fmt.Errorf("getting request %d: %w", id, err)
	}
	return &r, nil
}

// ProxyStats holds aggregate proxy statistics (no cost — computed at runtime).
type ProxyStats struct {
	TotalRequests      int
	TotalInputTokens   int64
	TotalOutputTokens  int64
	TotalCacheRead     int64
	TotalCacheCreation int64
	AvgLatencyMS       float64
	CacheHitRate       float64
	TotalMicrocompacts int
	TotalTruncated     int
}

// GetProxyStats returns aggregate statistics since the given timestamp.
func (s *DB) GetProxyStats(since, sessionID string) (*ProxyStats, error) {
	query := `SELECT
		COUNT(*),
		COALESCE(SUM(input_tokens), 0),
		COALESCE(SUM(output_tokens), 0),
		COALESCE(SUM(cache_read_tokens), 0),
		COALESCE(SUM(cache_creation_tokens), 0),
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

// ModelStats holds per-model token breakdowns for runtime cost computation.
type ModelStats struct {
	Model               string
	Requests            int
	InputTokens         int64
	OutputTokens        int64
	CacheReadTokens     int64
	CacheCreationTokens int64
}

// GetProxyStatsByModel returns per-model statistics since the given timestamp.
func (s *DB) GetProxyStatsByModel(since, sessionID string) ([]ModelStats, error) {
	query := `SELECT
		COALESCE(model, 'unknown'), COUNT(*),
		COALESCE(SUM(input_tokens), 0), COALESCE(SUM(output_tokens), 0),
		COALESCE(SUM(cache_read_tokens), 0), COALESCE(SUM(cache_creation_tokens), 0)
	FROM api_requests WHERE timestamp >= ?`
	args := []any{since}

	if sessionID != "" {
		query += ` AND session_id = ?`
		args = append(args, sessionID)
	}
	query += ` GROUP BY model ORDER BY COUNT(*) DESC`

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("querying model stats: %w", err)
	}
	defer rows.Close()

	var results []ModelStats
	for rows.Next() {
		var m ModelStats
		if err := rows.Scan(&m.Model, &m.Requests,
			&m.InputTokens, &m.OutputTokens,
			&m.CacheReadTokens, &m.CacheCreationTokens); err != nil {
			return nil, fmt.Errorf("scanning model stats: %w", err)
		}
		results = append(results, m)
	}
	return results, rows.Err()
}

// SessionInfo holds per-session aggregate data.
type SessionInfo struct {
	SessionID           string
	RequestCount        int
	FirstTimestamp      string
	LastTimestamp        string
	TotalInput          int64
	TotalOutput         int64
	TotalCacheRead      int64
	TotalCacheCreation  int64
}

// ListSessions returns sessions with aggregate stats, most recent first.
func (s *DB) ListSessions(limit int) ([]SessionInfo, error) {
	rows, err := s.db.Query(`SELECT
		session_id, COUNT(*), MIN(timestamp), MAX(timestamp),
		COALESCE(SUM(input_tokens), 0), COALESCE(SUM(output_tokens), 0),
		COALESCE(SUM(cache_read_tokens), 0), COALESCE(SUM(cache_creation_tokens), 0)
	FROM api_requests
	WHERE session_id != ''
	GROUP BY session_id
	ORDER BY MAX(timestamp) DESC
	LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("listing sessions: %w", err)
	}
	defer rows.Close()

	var results []SessionInfo
	for rows.Next() {
		var si SessionInfo
		if err := rows.Scan(&si.SessionID, &si.RequestCount,
			&si.FirstTimestamp, &si.LastTimestamp,
			&si.TotalInput, &si.TotalOutput,
			&si.TotalCacheRead, &si.TotalCacheCreation); err != nil {
			return nil, fmt.Errorf("scanning session: %w", err)
		}
		results = append(results, si)
	}
	return results, rows.Err()
}
