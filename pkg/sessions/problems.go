package sessions

import (
	"strings"
	"time"
)

// DetectProblems runs all heuristics against a fully-parsed session and
// returns any problems found.
func DetectProblems(s *ParsedSession, cfg ProblemConfig) Problems {
	var p Problems

	p.RepeatedReads = detectRepeatedReads(s, cfg)
	p.CompactionLoops = detectCompactionLoops(s, cfg)
	p.ModelMismatches = detectModelMismatches(s)
	p.ExcessiveSubagents = detectExcessiveSubagents(s, cfg)
	p.ContextFillNoProgress = detectContextFillNoProgress(s, cfg)

	return p
}

// detectRepeatedReads checks the main session and each subagent for files
// read >= cfg.RepeatedReadThreshold times using ToolName == "Read" tool calls.
func detectRepeatedReads(s *ParsedSession, cfg ProblemConfig) []RepeatedReadProblem {
	var problems []RepeatedReadProblem

	// Main session
	counts := make(map[string]int)
	for _, tc := range s.ToolCalls {
		if tc.ToolName == "Read" && tc.FilePath != "" {
			counts[tc.FilePath]++
		}
	}
	for path, count := range counts {
		if count >= cfg.RepeatedReadThreshold {
			problems = append(problems, RepeatedReadProblem{
				FilePath:  path,
				ReadCount: count,
			})
		}
	}

	// Each subagent independently
	for _, sa := range s.Subagents {
		saCounts := make(map[string]int)
		for _, tc := range sa.ToolCalls {
			if tc.ToolName == "Read" && tc.FilePath != "" {
				saCounts[tc.FilePath]++
			}
		}
		for path, count := range saCounts {
			if count >= cfg.RepeatedReadThreshold {
				problems = append(problems, RepeatedReadProblem{
					FilePath:   path,
					ReadCount:  count,
					SubagentID: sa.SessionID,
				})
			}
		}
	}

	return problems
}

// detectCompactionLoops checks the main session for compaction events followed
// by re-reads (ToolName == "Read") of files that were read before the compaction,
// within cfg.CompactionLoopWindowMin minutes.
func detectCompactionLoops(s *ParsedSession, cfg ProblemConfig) []CompactionLoopProblem {
	var problems []CompactionLoopProblem

	window := time.Duration(cfg.CompactionLoopWindowMin) * time.Minute

	for _, compaction := range s.Compactions {
		t := compaction.Timestamp

		// Build set of files read before this compaction
		readBefore := make(map[string]bool)
		for _, tc := range s.ToolCalls {
			if tc.ToolName == "Read" && tc.FilePath != "" && tc.Timestamp.Before(t) {
				readBefore[tc.FilePath] = true
			}
		}

		// Find files re-read strictly after t and within the window
		seen := make(map[string]bool)
		var rereadFiles []string
		for _, tc := range s.ToolCalls {
			if tc.ToolName != "Read" || tc.FilePath == "" {
				continue
			}
			if !tc.Timestamp.After(t) {
				continue
			}
			if tc.Timestamp.After(t.Add(window)) {
				continue
			}
			if readBefore[tc.FilePath] && !seen[tc.FilePath] {
				seen[tc.FilePath] = true
				rereadFiles = append(rereadFiles, tc.FilePath)
			}
		}

		if len(rereadFiles) > 0 {
			problems = append(problems, CompactionLoopProblem{
				CompactionAt:  t,
				RereadFiles:   rereadFiles,
				WindowMinutes: cfg.CompactionLoopWindowMin,
			})
		}
	}

	return problems
}

// detectModelMismatches raises a problem when a subagent uses opus and the
// parent session also uses opus.
func detectModelMismatches(s *ParsedSession) []ModelMismatchProblem {
	var problems []ModelMismatchProblem

	if !strings.Contains(strings.ToLower(s.Model), "opus") {
		return problems
	}

	for _, sa := range s.Subagents {
		if strings.Contains(strings.ToLower(sa.Model), "opus") {
			problems = append(problems, ModelMismatchProblem{
				SubagentID:    sa.SessionID,
				SubagentModel: sa.Model,
				ParentModel:   s.Model,
			})
		}
	}

	return problems
}

// detectExcessiveSubagents raises a problem when the session spawned too many
// subagents.
func detectExcessiveSubagents(s *ParsedSession, cfg ProblemConfig) *ExcessiveSubagentsProblem {
	if len(s.Subagents) >= cfg.ExcessiveSubagentCount {
		return &ExcessiveSubagentsProblem{
			SubagentCount: len(s.Subagents),
			Threshold:     cfg.ExcessiveSubagentCount,
		}
	}
	return nil
}

// detectContextFillNoProgress raises a problem when a session or subagent has
// output / (input + output) < cfg.ContextFillOutputRatio and input > 50_000.
func detectContextFillNoProgress(s *ParsedSession, cfg ProblemConfig) []ContextFillProblem {
	var problems []ContextFillProblem

	if p := contextFillCheck("", s.TokenUsage, cfg); p != nil {
		problems = append(problems, *p)
	}

	for _, sa := range s.Subagents {
		if p := contextFillCheck(sa.SessionID, sa.TokenUsage, cfg); p != nil {
			problems = append(problems, *p)
		}
	}

	return problems
}

func contextFillCheck(subagentID string, usage TokenUsage, cfg ProblemConfig) *ContextFillProblem {
	if usage.InputTokens <= 50_000 {
		return nil
	}
	total := usage.InputTokens + usage.OutputTokens
	if total == 0 {
		return nil
	}
	ratio := float64(usage.OutputTokens) / float64(total)
	if ratio < cfg.ContextFillOutputRatio {
		return &ContextFillProblem{
			SubagentID:   subagentID,
			InputTokens:  usage.InputTokens,
			OutputTokens: usage.OutputTokens,
			OutputRatio:  ratio,
		}
	}
	return nil
}
