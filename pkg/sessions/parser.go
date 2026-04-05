package sessions

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// ParseSession fully parses a session JSONL file and returns a ParsedSession.
// Malformed lines are skipped with a warning written to log (pass nil to suppress).
// cost is optional; pass nil to skip cost computation (EstimatedCost will be 0).
func ParseSession(transcriptPath string, log Logger, cost CostEstimator) (*ParsedSession, error) {
	seenIDs := make(map[string]bool)
	return parseSessionInternal(transcriptPath, log, cost, seenIDs, 0)
}

// ParseSessionWithCost parses a session JSONL file with a cost estimation callback.
func ParseSessionWithCost(path string, estimator func(model string, in, out, cacheRead, cacheCreate int) float64) (*ParsedSession, error) {
	return ParseSession(path, nil, estimator)
}

func parseSessionInternal(transcriptPath string, log Logger, cost CostEstimator, seenIDs map[string]bool, depth int) (*ParsedSession, error) {
	f, err := os.Open(transcriptPath)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", transcriptPath, err)
	}
	defer f.Close()

	session := &ParsedSession{
		TranscriptPath: transcriptPath,
		FileReadCounts: make(map[string]int),
	}

	// Extract session ID from file name (UUID before .jsonl)
	base := filepath.Base(transcriptPath)
	session.ID = strings.TrimSuffix(base, ".jsonl")
	session.ProjectDir = ParseProjectDir(transcriptPath)

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 4*1024*1024), 4*1024*1024)

	var offset int64
	var pendingToolUses []pendingTool // tool_use blocks awaiting matching tool_result

	for scanner.Scan() {
		rawBytes := scanner.Bytes()
		offset += int64(len(rawBytes)) + 1

		trimmed := bytes.TrimSpace(rawBytes)
		if len(trimmed) == 0 {
			continue
		}

		var raw rawLine
		if err := json.Unmarshal(trimmed, &raw); err != nil {
			if log != nil {
				log.Warnf("skipping malformed line at byte offset %d: %v", offset, err)
			}
			continue
		}

		ts, err := parseTimestamp(raw.Timestamp)
		if err != nil {
			ts = time.Time{}
		}

		switch raw.Type {
		case "assistant":
			if raw.Message == nil {
				continue
			}
			msg := raw.Message

			// Skip streaming partial chunks (no stop_reason)
			if msg.StopReason == nil || *msg.StopReason == "" {
				// Still need to track for deduplication — but skip empty stop_reason
				continue
			}

			// Deduplication by message ID
			if msg.ID != "" {
				if seenIDs[msg.ID] {
					continue
				}
				seenIDs[msg.ID] = true
			} else {
				// Fallback: content hash
				h := sha256hex(trimmed)
				if seenIDs[h] {
					continue
				}
				seenIDs[h] = true
			}

			// Parse content blocks
			blocks, toolUses, err := parseContentBlocks(msg.Content)
			if err != nil {
				if log != nil {
					log.Warnf("error parsing content blocks: %v", err)
				}
			}

			usage := TokenUsage{}
			if msg.Usage != nil {
				usage.InputTokens = msg.Usage.InputTokens
				usage.OutputTokens = msg.Usage.OutputTokens
				usage.CacheReadTokens = msg.Usage.CacheReadInputTokens
				usage.CacheCreationTokens = msg.Usage.CacheCreationInputTokens
			}

			isCompacted := raw.IsSidechain || isCompactionMessage(blocks)

			m := Message{
				UUID:        raw.UUID,
				Role:        "assistant",
				Timestamp:   ts,
				Model:       msg.Model,
				MessageID:   msg.ID,
				StopReason:  *msg.StopReason,
				Content:     blocks,
				Usage:       usage,
				IsCompacted: isCompacted,
			}

			// Detect compaction
			if isCompacted && len(session.Messages) > 0 {
				tokensBefore := 0
				for _, prev := range session.Messages {
					tokensBefore += prev.Usage.InputTokens
				}
				ce := CompactionEvent{
					Timestamp:      ts,
					MessagesBefore: len(session.Messages),
					MessagesAfter:  1,
					TokensBefore:   tokensBefore,
					TokensAfter:    usage.InputTokens,
				}
				session.Compactions = append(session.Compactions, ce)
			}

			session.Messages = append(session.Messages, m)

			// Queue tool_use blocks for later matching with tool_result
			for _, tu := range toolUses {
				tc := ToolCall{
					ToolName:  tu.Name,
					Timestamp: ts,
					MessageID: msg.ID,
				}
				extractToolCallFields(&tc, tu)
				pendingToolUses = append(pendingToolUses, pendingTool{toolUseID: tu.ID, call: tc})
			}

			// Track timing
			if session.StartedAt.IsZero() || ts.Before(session.StartedAt) {
				session.StartedAt = ts
			}
			if ts.After(session.EndedAt) {
				session.EndedAt = ts
			}
			if msg.Model != "" && session.Model == "" {
				session.Model = msg.Model
			}

			// Accumulate token usage
			session.TokenUsage.InputTokens += usage.InputTokens
			session.TokenUsage.OutputTokens += usage.OutputTokens
			session.TokenUsage.CacheReadTokens += usage.CacheReadTokens
			session.TokenUsage.CacheCreationTokens += usage.CacheCreationTokens

		case "user":
			if raw.Message == nil {
				continue
			}

			// Deduplication by uuid
			dedupKey := raw.UUID
			if dedupKey == "" {
				dedupKey = sha256hex(trimmed)
			}
			if seenIDs[dedupKey] {
				continue
			}
			seenIDs[dedupKey] = true

			blocks, err := parseUserContent(raw.Message.Content)
			if err != nil {
				if log != nil {
					log.Warnf("error parsing user content: %v", err)
				}
			}

			// Match tool_result blocks with pending tool_use
			for i := range blocks {
				if blocks[i].Type == "tool_result" && blocks[i].ToolResult != nil {
					tr := blocks[i].ToolResult
					// Find and finalize matching pending tool use
					for j := range pendingToolUses {
						if pendingToolUses[j].toolUseID == tr.ToolUseID {
							pendingToolUses[j].call.ResultSize = tr.SizeBytes
							pendingToolUses[j].call.IsError = tr.IsError
							finalizeToolCall(pendingToolUses[j].call, session)
							pendingToolUses = append(pendingToolUses[:j], pendingToolUses[j+1:]...)
							break
						}
					}
				}
			}

			m := Message{
				UUID:      raw.UUID,
				Role:      "user",
				Timestamp: ts,
				Content:   blocks,
			}
			session.Messages = append(session.Messages, m)

			if session.StartedAt.IsZero() || (!ts.IsZero() && ts.Before(session.StartedAt)) {
				session.StartedAt = ts
			}
			if ts.After(session.EndedAt) {
				session.EndedAt = ts
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scanning %s: %w", transcriptPath, err)
	}

	// Finalize any unmatched tool calls (no result seen)
	for _, pt := range pendingToolUses {
		finalizeToolCall(pt.call, session)
	}

	// Build FileReadCounts
	for _, tc := range session.ToolCalls {
		if tc.FilePath != "" {
			session.FileReadCounts[tc.FilePath]++
		}
	}

	// Compute cost
	if cost != nil {
		session.TokenUsage.EstimatedCost = cost(
			session.Model,
			session.TokenUsage.InputTokens,
			session.TokenUsage.OutputTokens,
			session.TokenUsage.CacheReadTokens,
			session.TokenUsage.CacheCreationTokens,
		)
	}

	// Load subagents (depth limited)
	if depth < 5 {
		session.Subagents = loadSubagents(transcriptPath, log, cost, seenIDs, depth+1)
	}

	return session, nil
}

type pendingTool struct {
	toolUseID string
	call      ToolCall
}

func finalizeToolCall(tc ToolCall, session *ParsedSession) {
	session.ToolCalls = append(session.ToolCalls, tc)
}

func extractToolCallFields(tc *ToolCall, tu *ToolUseBlock) {
	input := tu.Input
	switch tu.Name {
	case "Read":
		if v, ok := input["file_path"].(string); ok {
			tc.FilePath = v
		}
	case "Edit":
		if v, ok := input["file_path"].(string); ok {
			tc.FilePath = v
		}
	case "Write":
		if v, ok := input["file_path"].(string); ok {
			tc.FilePath = v
		}
	case "NotebookEdit":
		if v, ok := input["notebook_path"].(string); ok {
			tc.FilePath = v
		}
	case "Glob":
		if v, ok := input["path"].(string); ok {
			tc.FilePath = v
		}
		if v, ok := input["pattern"].(string); ok {
			tc.Pattern = v
		}
	case "Grep":
		if v, ok := input["path"].(string); ok {
			tc.FilePath = v
		}
		if v, ok := input["pattern"].(string); ok {
			tc.Pattern = v
		}
	case "Bash":
		if v, ok := input["command"].(string); ok {
			tc.Command = v
		}
	}
}

func parseContentBlocks(raw json.RawMessage) ([]ContentBlock, []*ToolUseBlock, error) {
	if len(raw) == 0 {
		return nil, nil, nil
	}

	var rawBlocks []rawContentBlock
	if err := json.Unmarshal(raw, &rawBlocks); err != nil {
		// Content might be a plain string
		var s string
		if err2 := json.Unmarshal(raw, &s); err2 == nil {
			return []ContentBlock{{Type: "text", Text: s}}, nil, nil
		}
		return nil, nil, err
	}

	var blocks []ContentBlock
	var toolUses []*ToolUseBlock

	for _, rb := range rawBlocks {
		switch rb.Type {
		case "text", "thinking":
			blocks = append(blocks, ContentBlock{Type: rb.Type, Text: rb.Text})
		case "tool_use":
			var input map[string]interface{}
			if len(rb.Input) > 0 {
				_ = json.Unmarshal(rb.Input, &input)
			}
			tu := &ToolUseBlock{
				ID:    rb.ID,
				Name:  rb.Name,
				Input: input,
			}
			toolUses = append(toolUses, tu)
			blocks = append(blocks, ContentBlock{Type: "tool_use", ToolUse: tu})
		}
	}

	return blocks, toolUses, nil
}

func parseUserContent(raw json.RawMessage) ([]ContentBlock, error) {
	if len(raw) == 0 {
		return nil, nil
	}

	// Content can be a string or array
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return []ContentBlock{{Type: "text", Text: s}}, nil
	}

	var rawBlocks []rawContentBlock
	if err := json.Unmarshal(raw, &rawBlocks); err != nil {
		return nil, err
	}

	var blocks []ContentBlock
	for _, rb := range rawBlocks {
		switch rb.Type {
		case "text":
			blocks = append(blocks, ContentBlock{Type: "text", Text: rb.Text})
		case "tool_result":
			content := extractToolResultContent(rb.Content)
			sizeBytes := len(rb.Content)
			if sizeBytes == 0 {
				// content might be a string directly
				sizeBytes = len(content)
			}
			tr := &ToolResultBlock{
				ToolUseID: rb.ToolUseID,
				Content:   content,
				IsError:   rb.IsError,
				SizeBytes: sizeBytes,
			}
			blocks = append(blocks, ContentBlock{Type: "tool_result", ToolResult: tr})
		}
	}

	return blocks, nil
}

func extractToolResultContent(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	// Could be a string
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	// Could be an array of content blocks
	var blocks []rawContentBlock
	if err := json.Unmarshal(raw, &blocks); err == nil {
		var parts []string
		for _, b := range blocks {
			if b.Text != "" {
				parts = append(parts, b.Text)
			}
		}
		return strings.Join(parts, "\n")
	}
	return string(raw)
}

func isCompactionMessage(blocks []ContentBlock) bool {
	for _, b := range blocks {
		if b.Type == "text" && (strings.HasPrefix(b.Text, "<compacted_summary>") ||
			(strings.Contains(b.Text, "previous conversation") && len(b.Text) > 100)) {
			return true
		}
	}
	return false
}

func parseTimestamp(raw json.RawMessage) (time.Time, error) {
	if len(raw) == 0 {
		return time.Time{}, nil
	}

	// Try numeric (epoch milliseconds)
	var f float64
	if err := json.Unmarshal(raw, &f); err == nil {
		sec := int64(f / 1000)
		ms := int64(math.Mod(f, 1000))
		return time.Unix(sec, ms*int64(time.Millisecond)).UTC(), nil
	}

	// Try string (ISO 8601)
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		// Try RFC3339
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			return t, nil
		}
		// Try RFC3339Nano
		if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
			return t, nil
		}
		// Try with trailing Z stripped
		s2 := strings.TrimSuffix(s, "Z")
		if t, err := time.Parse("2006-01-02T15:04:05.000", s2); err == nil {
			return t.UTC(), nil
		}
		if t, err := time.Parse("2006-01-02T15:04:05", s2); err == nil {
			return t.UTC(), nil
		}
		return time.Time{}, fmt.Errorf("unknown timestamp format: %s", s)
	}

	return time.Time{}, fmt.Errorf("unrecognized timestamp: %s", string(raw))
}

func sha256hex(data []byte) string {
	h := sha256.Sum256(data)
	return fmt.Sprintf("%x", h)
}

func loadSubagents(parentTranscriptPath string, log Logger, cost CostEstimator, seenIDs map[string]bool, depth int) []SubagentInfo {
	stem := strings.TrimSuffix(parentTranscriptPath, ".jsonl")
	subagentDir := stem + "/subagents/"

	entries, err := os.ReadDir(subagentDir)
	if err != nil {
		// No subagents directory is normal
		return nil
	}

	var infos []SubagentInfo
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasPrefix(name, "agent-") || !strings.HasSuffix(name, ".jsonl") {
			continue
		}
		agentPath := filepath.Join(subagentDir, name)
		ps, err := parseSessionInternal(agentPath, log, cost, seenIDs, depth)
		if err != nil {
			if log != nil {
				log.Warnf("failed to parse subagent %s: %v", agentPath, err)
			}
			continue
		}

		info := SubagentInfo{
			SessionID:   ps.ID,
			Model:       ps.Model,
			StartedAt:   ps.StartedAt,
			EndedAt:     ps.EndedAt,
			TokenUsage:  ps.TokenUsage,
			ToolCalls:   ps.ToolCalls,
			Compactions: ps.Compactions,
			FileReadCounts: ps.FileReadCounts,
		}

		// Build sorted FileReads list
		for path := range ps.FileReadCounts {
			info.FileReads = append(info.FileReads, path)
		}
		sort.Strings(info.FileReads)

		// Try to read metadata
		agentID := strings.TrimSuffix(strings.TrimPrefix(name, "agent-"), ".jsonl")
		metaPath := filepath.Join(subagentDir, "agent-"+agentID+".meta.json")
		if metaBytes, err := os.ReadFile(metaPath); err == nil {
			var meta struct {
				AgentName string `json:"agent_name"`
				AgentType string `json:"agent_type"`
			}
			if json.Unmarshal(metaBytes, &meta) == nil {
				info.AgentName = meta.AgentName
				info.AgentType = meta.AgentType
			}
		}

		infos = append(infos, info)
	}

	return infos
}

// ParseSessionSummary reads only enough of the JSONL to populate a SessionSummary.
func ParseSessionSummary(transcriptPath string) (*SessionSummary, error) {
	fi, err := os.Stat(transcriptPath)
	if err != nil {
		return nil, fmt.Errorf("stat %s: %w", transcriptPath, err)
	}

	base := filepath.Base(transcriptPath)
	sessionID := strings.TrimSuffix(base, ".jsonl")

	summary := &SessionSummary{
		ID:             sessionID,
		TranscriptPath: transcriptPath,
		ProjectDir:     ParseProjectDir(transcriptPath),
		FileSize:       fi.Size(),
		LastModified:   fi.ModTime(),
		IsActive:       time.Since(fi.ModTime()) < 5*time.Minute,
	}

	f, err := os.Open(transcriptPath)
	if err != nil {
		return summary, nil
	}
	defer f.Close()

	// Read first 20 lines for start time
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 4*1024*1024), 4*1024*1024)
	lineCount := 0
	for scanner.Scan() && lineCount < 20 {
		line := scanner.Bytes()
		lineCount++
		var raw rawLine
		if err := json.Unmarshal(bytes.TrimSpace(line), &raw); err != nil {
			continue
		}
		if raw.Type == "user" || raw.Type == "assistant" {
			summary.MessageCount++
		}
		if summary.StartedAt.IsZero() {
			if ts, err := parseTimestamp(raw.Timestamp); err == nil && !ts.IsZero() {
				summary.StartedAt = ts
			}
		}
	}

	return summary, nil
}

// ParseProjectDir extracts the decoded project directory path from a transcript file path.
func ParseProjectDir(transcriptPath string) string {
	projectDir := filepath.Dir(transcriptPath)
	encodedName := filepath.Base(projectDir)

	// Try sessions-index.json
	indexPath := filepath.Join(projectDir, "sessions-index.json")
	if data, err := os.ReadFile(indexPath); err == nil {
		var idx struct {
			OriginalPath string `json:"originalPath"`
		}
		if json.Unmarshal(data, &idx) == nil && idx.OriginalPath != "" {
			return idx.OriginalPath
		}
	}

	// Heuristic: strip leading - then replace - with /
	// Common prefixes: -home-user-dev-, -home-user-exp-, etc.
	decoded := strings.ReplaceAll(encodedName, "-", "/")
	if !strings.HasPrefix(decoded, "/") {
		decoded = "/" + decoded
	}

	// Clean up double slashes
	for strings.Contains(decoded, "//") {
		decoded = strings.ReplaceAll(decoded, "//", "/")
	}

	return decoded
}

// DiscoverProjects scans ~/.claude/projects/ and returns one ProjectInfo per
// subdirectory, sorted by last activity descending.
func DiscoverProjects() ([]ProjectInfo, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	base := filepath.Join(home, ".claude", "projects")

	entries, err := os.ReadDir(base)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", base, err)
	}

	var projects []ProjectInfo
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		dirPath := filepath.Join(base, entry.Name())

		pi := ProjectInfo{
			EncodedName: entry.Name(),
			DirPath:     dirPath,
		}

		// Try sessions-index.json
		indexPath := filepath.Join(dirPath, "sessions-index.json")
		if data, err := os.ReadFile(indexPath); err == nil {
			var idx struct {
				OriginalPath string `json:"originalPath"`
			}
			if json.Unmarshal(data, &idx) == nil && idx.OriginalPath != "" {
				pi.OriginalPath = idx.OriginalPath
			}
		}
		if pi.OriginalPath == "" {
			pi.OriginalPath = ParseProjectDir(filepath.Join(dirPath, "fake.jsonl"))
		}

		// Count JSONL files and find latest mtime
		jsonlEntries, err := filepath.Glob(filepath.Join(dirPath, "*.jsonl"))
		if err == nil {
			pi.SessionCount = len(jsonlEntries)
			for _, jf := range jsonlEntries {
				if fi, err := os.Stat(jf); err == nil {
					if fi.ModTime().After(pi.LastActivity) {
						pi.LastActivity = fi.ModTime()
					}
				}
			}
		}

		projects = append(projects, pi)
	}

	sort.Slice(projects, func(i, j int) bool {
		return projects[i].LastActivity.After(projects[j].LastActivity)
	})

	return projects, nil
}

// DiscoverSessions returns lightweight summaries for all session JSONL files
// found in projectDir, sorted by last-modified descending.
func DiscoverSessions(projectDir string) ([]SessionSummary, error) {
	entries, err := filepath.Glob(filepath.Join(projectDir, "*.jsonl"))
	if err != nil {
		return nil, err
	}

	var summaries []SessionSummary
	for _, path := range entries {
		base := filepath.Base(path)
		sessionID := strings.TrimSuffix(base, ".jsonl")
		if !isValidUUID(sessionID) {
			continue
		}
		s, err := ParseSessionSummary(path)
		if err != nil {
			continue
		}
		summaries = append(summaries, *s)
	}

	sort.Slice(summaries, func(i, j int) bool {
		return summaries[i].LastModified.After(summaries[j].LastModified)
	})

	return summaries, nil
}

// isValidUUID checks if s matches the UUID format.
func isValidUUID(s string) bool {
	if len(s) != 36 {
		return false
	}
	for i, c := range s {
		switch i {
		case 8, 13, 18, 23:
			if c != '-' {
				return false
			}
		default:
			if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
				return false
			}
		}
	}
	return true
}

// DetectProblems runs all heuristics against a fully-parsed session.
func DetectProblems(s *ParsedSession, cfg ProblemConfig) Problems {
	var p Problems

	// Repeated reads
	for path, count := range s.FileReadCounts {
		if count >= cfg.RepeatedReadThreshold {
			p.RepeatedReads = append(p.RepeatedReads, RepeatedReadProblem{
				FilePath:  path,
				ReadCount: count,
			})
		}
	}
	for _, sub := range s.Subagents {
		for path, count := range sub.FileReadCounts {
			if count >= cfg.RepeatedReadThreshold {
				p.RepeatedReads = append(p.RepeatedReads, RepeatedReadProblem{
					FilePath:   path,
					ReadCount:  count,
					SubagentID: sub.SessionID,
				})
			}
		}
	}

	// Compaction loops
	window := time.Duration(cfg.CompactionLoopWindowMin) * time.Minute
	for _, ce := range s.Compactions {
		var reread []string
		readBefore := make(map[string]bool)
		for _, tc := range s.ToolCalls {
			if tc.FilePath != "" && tc.Timestamp.Before(ce.Timestamp) {
				readBefore[tc.FilePath] = true
			}
		}
		for _, tc := range s.ToolCalls {
			if tc.FilePath != "" && !tc.Timestamp.Before(ce.Timestamp) && tc.Timestamp.Before(ce.Timestamp.Add(window)) {
				if readBefore[tc.FilePath] {
					reread = append(reread, tc.FilePath)
				}
			}
		}
		if len(reread) > 0 {
			p.CompactionLoops = append(p.CompactionLoops, CompactionLoopProblem{
				CompactionAt:  ce.Timestamp,
				RereadFiles:   reread,
				WindowMinutes: cfg.CompactionLoopWindowMin,
			})
		}
	}

	// Excessive subagents
	if len(s.Subagents) >= cfg.ExcessiveSubagentCount {
		p.ExcessiveSubagents = &ExcessiveSubagentsProblem{
			SubagentCount: len(s.Subagents),
			Threshold:     cfg.ExcessiveSubagentCount,
		}
	}

	// Context fill
	checkContextFill := func(id string, usage TokenUsage) {
		total := usage.InputTokens + usage.OutputTokens
		if total > 0 {
			ratio := float64(usage.OutputTokens) / float64(total)
			if ratio < cfg.ContextFillOutputRatio && usage.InputTokens > 10000 {
				p.ContextFillNoProgress = append(p.ContextFillNoProgress, ContextFillProblem{
					SubagentID:   id,
					InputTokens:  usage.InputTokens,
					OutputTokens: usage.OutputTokens,
					OutputRatio:  ratio,
				})
			}
		}
	}
	checkContextFill("", s.TokenUsage)
	for _, sub := range s.Subagents {
		checkContextFill(sub.SessionID, sub.TokenUsage)
	}

	// Model mismatches
	for _, sub := range s.Subagents {
		if strings.Contains(strings.ToLower(sub.Model), "opus") &&
			strings.Contains(strings.ToLower(s.Model), "opus") {
			p.ModelMismatches = append(p.ModelMismatches, ModelMismatchProblem{
				SubagentID:    sub.SessionID,
				SubagentModel: sub.Model,
				ParentModel:   s.Model,
			})
		}
	}

	return p
}

