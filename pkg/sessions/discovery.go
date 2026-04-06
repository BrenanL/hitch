package sessions

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// DiscoverProjects scans claudeDir/projects/ and returns one ProjectInfo per
// subdirectory, sorted by last activity descending.
// claudeDir is typically ~/.claude.
func DiscoverProjects(claudeDir string) ([]ProjectInfo, error) {
	base := filepath.Join(claudeDir, "projects")

	entries, err := os.ReadDir(base)
	if err != nil {
		return nil, err
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

		// Try sessions-index.json first (authoritative source).
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
			pi.OriginalPath = decodeProjectDirName(entry.Name())
		}

		// Count JSONL files and find the latest mtime.
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
// found directly in projectDir (not recursive), sorted by last-modified descending.
// Files whose names are not valid UUIDs (e.g. legacy agent-*.jsonl) are skipped.
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

// ParseSessionSummary reads only the first 20 lines of a JSONL transcript to
// populate a SessionSummary without full parsing. Fast enough for listing
// hundreds of sessions.
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

// ParseProjectDir extracts the decoded project directory path from a transcript
// file path. It reads sessions-index.json in the parent directory if present
// (authoritative source), then falls back to heuristic decoding of the encoded
// directory name.
func ParseProjectDir(transcriptPath string) string {
	projectDir := filepath.Dir(transcriptPath)
	encodedName := filepath.Base(projectDir)

	// Try sessions-index.json first.
	indexPath := filepath.Join(projectDir, "sessions-index.json")
	if data, err := os.ReadFile(indexPath); err == nil {
		var idx struct {
			OriginalPath string `json:"originalPath"`
		}
		if json.Unmarshal(data, &idx) == nil && idx.OriginalPath != "" {
			return idx.OriginalPath
		}
	}

	return decodeProjectDirName(encodedName)
}

// decodeProjectDirName converts an encoded project directory name
// (e.g. "-home-user-dev-hitch") back to an absolute path ("/home/user/dev/hitch").
func decodeProjectDirName(encodedName string) string {
	decoded := strings.ReplaceAll(encodedName, "-", "/")
	if !strings.HasPrefix(decoded, "/") {
		decoded = "/" + decoded
	}
	for strings.Contains(decoded, "//") {
		decoded = strings.ReplaceAll(decoded, "//", "/")
	}
	return decoded
}

// isValidUUID checks if s matches the UUID format (lowercase hex with dashes).
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
