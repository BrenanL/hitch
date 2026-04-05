package sessions

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// LoadSubagents finds and parses subagent JSONL files located in
// <sessionDir>/subagents/. Subagents are discovered by globbing agent-*.jsonl
// in that directory. Recursive subagents (subagents that spawned their own
// subagents) are loaded up to a depth limit of 5.
//
// sessionDir is the path to the session directory derived from the parent
// session transcript path by stripping the ".jsonl" suffix.
// For example, if the parent transcript is:
//
//	~/.claude/projects/-home-user-dev-hitch/<uuid>.jsonl
//
// then sessionDir is:
//
//	~/.claude/projects/-home-user-dev-hitch/<uuid>
func LoadSubagents(sessionDir string) ([]SubagentInfo, error) {
	return loadSubagentsFromDir(sessionDir, nil, nil, make(map[string]bool), 0)
}

// loadSubagentsFromDir is the internal recursive implementation used by both
// LoadSubagents (public API) and parseSessionInternal (parser integration).
// depth tracks recursion depth; at depth >= 5 loading stops.
func loadSubagentsFromDir(sessionDir string, log Logger, cost CostEstimator, seenIDs map[string]bool, depth int) ([]SubagentInfo, error) {
	if depth >= 5 {
		return nil, nil
	}

	subagentDir := filepath.Join(sessionDir, "subagents")
	entries, err := os.ReadDir(subagentDir)
	if err != nil {
		// No subagents directory is normal; not an error.
		return nil, nil
	}

	var infos []SubagentInfo
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasPrefix(name, "agent-") || !strings.HasSuffix(name, ".jsonl") {
			continue
		}
		agentPath := filepath.Join(subagentDir, name)
		ps, err := parseSessionInternal(agentPath, log, cost, seenIDs, depth+1)
		if err != nil {
			if log != nil {
				log.Warnf("failed to parse subagent %s: %v", agentPath, err)
			}
			continue
		}

		info := SubagentInfo{
			SessionID:      ps.ID,
			Model:          ps.Model,
			StartedAt:      ps.StartedAt,
			EndedAt:        ps.EndedAt,
			TokenUsage:     ps.TokenUsage,
			ToolCalls:      ps.ToolCalls,
			Compactions:    ps.Compactions,
			FileReadCounts: ps.FileReadCounts,
		}

		// Build sorted FileReads list from the map.
		for path := range ps.FileReadCounts {
			info.FileReads = append(info.FileReads, path)
		}
		sort.Strings(info.FileReads)

		// Load optional metadata from agent-<id>.meta.json.
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

	return infos, nil
}
