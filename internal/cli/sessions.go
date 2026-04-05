package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/BrenanL/hitch/internal/pricing"
	"github.com/BrenanL/hitch/pkg/sessions"
	"github.com/spf13/cobra"
)

func newSessionsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sessions",
		Short: "Browse and analyze Claude Code session transcripts",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	cmd.AddCommand(
		newSessionsListCmd(),
		newSessionsShowCmd(),
		newSessionsAgentsCmd(),
		newSessionsProblemsCmd(),
	)
	return cmd
}

// claudeDir returns the path to ~/.claude, respecting the HOME env var.
func claudeDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".claude")
}

// formatTokens formats a token count with SI suffix.
func formatTokens(n int) string {
	if n >= 1_000_000 {
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	}
	if n >= 1_000 {
		return fmt.Sprintf("%.0fK", float64(n)/1_000)
	}
	return fmt.Sprintf("%d", n)
}

// formatSessionDuration formats a duration as "Xm" or "Xh Ym" (no seconds).
func formatSessionDuration(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	if h > 0 {
		return fmt.Sprintf("%dh %dm", h, m)
	}
	return fmt.Sprintf("%dm", m)
}

// shortenHome replaces the user home directory prefix with "~/".
func shortenHome(path string) string {
	home, _ := os.UserHomeDir()
	if strings.HasPrefix(path, home) {
		return "~" + path[len(home):]
	}
	return path
}

// truncate truncates s to at most n runes.
func truncate(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n])
}

// findSessionByPrefix searches all projects for a session matching the given
// ID prefix. Returns the session summary and the project directory it came from.
// Returns an error if zero or multiple matches are found.
func findSessionByPrefix(prefix string, projectFilter string) (*sessions.SessionSummary, error) {
	prefix = strings.ToLower(prefix)

	projs, err := sessions.DiscoverProjects(claudeDir())
	if err != nil {
		return nil, fmt.Errorf("discovering projects: %w", err)
	}

	var matches []sessions.SessionSummary
	for _, proj := range projs {
		if projectFilter != "" && !strings.Contains(proj.OriginalPath, projectFilter) {
			continue
		}
		sums, err := sessions.DiscoverSessions(proj.DirPath)
		if err != nil {
			continue
		}
		for _, s := range sums {
			if strings.HasPrefix(strings.ToLower(s.ID), prefix) {
				matches = append(matches, s)
			}
		}
	}

	if len(matches) == 0 {
		return nil, fmt.Errorf("no session found with prefix %q", prefix)
	}
	if len(matches) > 1 {
		var ids []string
		for _, m := range matches {
			ids = append(ids, m.ID[:8])
		}
		return nil, fmt.Errorf("ambiguous prefix %q matches %d sessions: %s", prefix, len(matches), strings.Join(ids, ", "))
	}
	return &matches[0], nil
}

// --- list ---

func newSessionsListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List recent sessions",
		RunE:  runSessionsList,
	}
	cmd.Flags().Int("limit", 20, "Maximum sessions to show")
	cmd.Flags().String("project", "", "Filter by project path substring")
	cmd.Flags().Bool("today", false, "Only sessions started today (local time)")
	cmd.Flags().Bool("active", false, "Only sessions with mtime within 5 minutes")
	return cmd
}

func runSessionsList(cmd *cobra.Command, args []string) error {
	limit, _ := cmd.Flags().GetInt("limit")
	projectFilter, _ := cmd.Flags().GetString("project")
	todayOnly, _ := cmd.Flags().GetBool("today")
	activeOnly, _ := cmd.Flags().GetBool("active")

	projs, err := sessions.DiscoverProjects(claudeDir())
	if err != nil {
		return fmt.Errorf("discovering projects: %w", err)
	}

	type row struct {
		s       sessions.SessionSummary
		project string
	}
	var rows []row

	for _, proj := range projs {
		if projectFilter != "" && !strings.Contains(proj.OriginalPath, projectFilter) {
			continue
		}
		sums, err := sessions.DiscoverSessions(proj.DirPath)
		if err != nil {
			continue
		}
		for _, s := range sums {
			rows = append(rows, row{s: s, project: proj.OriginalPath})
		}
	}

	// Sort by LastModified descending.
	sort.Slice(rows, func(i, j int) bool {
		return rows[i].s.LastModified.After(rows[j].s.LastModified)
	})

	// Apply filters.
	today := time.Now()
	var filtered []row
	for _, r := range rows {
		if todayOnly {
			st := r.s.StartedAt
			if st.IsZero() {
				st = r.s.LastModified
			}
			if st.Year() != today.Year() || st.YearDay() != today.YearDay() {
				continue
			}
		}
		if activeOnly && !r.s.IsActive {
			continue
		}
		filtered = append(filtered, r)
		if len(filtered) >= limit {
			break
		}
	}

	if len(filtered) == 0 {
		fmt.Println("No sessions found.")
		return nil
	}

	fmt.Printf("%-10s %-32s %-17s %-10s %-8s %s\n",
		"ID", "Project", "Started", "Duration", "Tokens", "Cost")
	for _, r := range filtered {
		s := r.s
		idStr := s.ID
		if len(idStr) > 8 {
			idStr = idStr[:8]
		}
		if s.IsActive {
			idStr += "*"
		}

		proj := shortenHome(r.project)
		proj = truncate(proj, 30)

		started := ""
		startTime := s.StartedAt
		if startTime.IsZero() {
			startTime = s.LastModified
		}
		if !startTime.IsZero() {
			started = startTime.Local().Format("2006-01-02 15:04")
		}

		duration := ""
		if s.IsActive {
			duration = "(active)"
		} else if !s.StartedAt.IsZero() && !s.LastModified.IsZero() {
			d := s.LastModified.Sub(s.StartedAt)
			duration = formatSessionDuration(d)
		}

		// Sessions list uses summary only — token/cost data not available without full parse.
		tokens := "-"
		cost := "-"

		fmt.Printf("%-10s %-32s %-17s %-10s %-8s %s\n",
			idStr, proj, started, duration, tokens, cost)
	}
	return nil
}

// --- show ---

func newSessionsShowCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show <id>",
		Short: "Detailed view of a session",
		Args:  cobra.ExactArgs(1),
		RunE:  runSessionsShow,
	}
	cmd.Flags().String("project", "", "Narrow search to one project")
	cmd.Flags().Bool("no-files", false, "Omit the top file reads section")
	cmd.Flags().Bool("no-subagents", false, "Omit the subagents section")
	return cmd
}

func runSessionsShow(cmd *cobra.Command, args []string) error {
	projectFilter, _ := cmd.Flags().GetString("project")
	noFiles, _ := cmd.Flags().GetBool("no-files")
	noSubagents, _ := cmd.Flags().GetBool("no-subagents")

	sum, err := findSessionByPrefix(args[0], projectFilter)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	pricingData := pricing.LoadPricing()
	s, err := sessions.ParseSession(sum.TranscriptPath, nil, pricingData.EstimateCost)
	if err != nil {
		return fmt.Errorf("parsing session: %w", err)
	}

	shortID := s.ID
	if len(shortID) > 8 {
		shortID = shortID[:8]
	}

	proj := shortenHome(s.ProjectDir)
	fmt.Printf("Session %s — %s\n", shortID, proj)

	started := ""
	if !s.StartedAt.IsZero() {
		started = s.StartedAt.Local().Format("2006-01-02 15:04")
	}
	dur := ""
	if !s.StartedAt.IsZero() && !s.EndedAt.IsZero() {
		dur = formatSessionDuration(s.EndedAt.Sub(s.StartedAt))
	}
	fmt.Printf("Started: %s | Duration: %s | Model: %s\n\n", started, dur, s.Model)

	u := s.TokenUsage
	fmt.Printf("Tokens: %s input / %s output / %s cache-read / %s cache-write\n",
		formatTokens(u.InputTokens),
		formatTokens(u.OutputTokens),
		formatTokens(u.CacheReadTokens),
		formatTokens(u.CacheCreationTokens),
	)
	fmt.Printf("Cost:   $%.2f estimated\n", u.EstimatedCost)
	fmt.Printf("Rate-limit tokens: %s  (excludes cache-read)\n\n", formatTokens(u.RateLimit()))

	fmt.Printf("Compactions: %d\n", len(s.Compactions))
	for _, c := range s.Compactions {
		var pct float64
		if c.TokensBefore > 0 {
			pct = float64(c.TokensBefore-c.TokensAfter) / float64(c.TokensBefore) * 100
		}
		fmt.Printf("  %s  %s tok -> %s tok  (%.1f%% reduction)\n",
			c.Timestamp.Local().Format("15:04"),
			formatTokens(c.TokensBefore),
			formatTokens(c.TokensAfter),
			pct,
		)
	}
	fmt.Println()

	// Tool calls summary
	toolCounts := make(map[string]int)
	for _, tc := range s.ToolCalls {
		toolCounts[tc.ToolName]++
	}
	fmt.Printf("Tool calls: %d total\n", len(s.ToolCalls))
	if len(toolCounts) > 0 {
		type kv struct {
			k string
			v int
		}
		var sorted []kv
		for k, v := range toolCounts {
			sorted = append(sorted, kv{k, v})
		}
		sort.Slice(sorted, func(i, j int) bool {
			if sorted[i].v != sorted[j].v {
				return sorted[i].v > sorted[j].v
			}
			return sorted[i].k < sorted[j].k
		})
		var parts []string
		for _, kv := range sorted {
			parts = append(parts, fmt.Sprintf("  %s x %d", kv.k, kv.v))
		}
		fmt.Println(strings.Join(parts, "    "))
	}
	fmt.Println()

	// Unique files
	uniqueFiles := len(s.FileReadCounts)
	fmt.Printf("Files touched: %d unique files\n\n", uniqueFiles)

	// Subagents
	if !noSubagents && len(s.Subagents) > 0 {
		fmt.Printf("Subagents: %d\n", len(s.Subagents))
		fmt.Printf("  %-10s %-24s %-10s %-8s %-7s %s\n",
			"ID", "Name", "Model", "Tokens", "Cost", "Compactions")
		for _, sa := range s.Subagents {
			saShort := sa.SessionID
			if len(saShort) > 8 {
				saShort = saShort[:8]
			}
			saModel := shortModelName(sa.Model)
			saName := sa.AgentName
			if saName == "" {
				saName = "-"
			}
			fmt.Printf("  %-10s %-24s %-10s %-8s $%-6.2f %d\n",
				saShort,
				truncate(saName, 22),
				saModel,
				formatTokens(sa.TokenUsage.Total()),
				sa.TokenUsage.EstimatedCost,
				len(sa.Compactions),
			)
		}
		fmt.Println()
	}

	// Top file reads
	if !noFiles && len(s.FileReadCounts) > 0 {
		type fc struct {
			path  string
			count int
		}
		var fcs []fc
		for p, c := range s.FileReadCounts {
			fcs = append(fcs, fc{p, c})
		}
		sort.Slice(fcs, func(i, j int) bool {
			if fcs[i].count != fcs[j].count {
				return fcs[i].count > fcs[j].count
			}
			return fcs[i].path < fcs[j].path
		})
		maxShow := 10
		if len(fcs) < maxShow {
			maxShow = len(fcs)
		}
		fmt.Println("Top file reads (by read count):")
		for _, f := range fcs[:maxShow] {
			// Rough token estimate: bytes / 3.5
			var tokEst string
			if fi, err := os.Stat(f.path); err == nil {
				est := int(float64(fi.Size()) / 3.5)
				tokEst = fmt.Sprintf("(~%s tok est.)", formatTokens(est))
			}
			fmt.Printf("  %-60s  %dx  %s\n", f.path, f.count, tokEst)
		}
		fmt.Println()
	}

	// Problems
	probs := sessions.DetectProblems(s, sessions.DefaultProblemConfig())
	hasProblems := len(probs.RepeatedReads) > 0 ||
		len(probs.CompactionLoops) > 0 ||
		len(probs.ModelMismatches) > 0 ||
		probs.ExcessiveSubagents != nil ||
		len(probs.ContextFillNoProgress) > 0

	if !hasProblems {
		fmt.Println("Problems detected: none")
	} else {
		fmt.Println("Problems detected:")
		for _, rr := range probs.RepeatedReads {
			where := ""
			if rr.SubagentID != "" {
				where = fmt.Sprintf(" (%s)", rr.SubagentID[:min(8, len(rr.SubagentID))])
			}
			fmt.Printf("  ! Repeated reads: %s read %d times%s\n",
				filepath.Base(rr.FilePath), rr.ReadCount, where)
		}
		for _, cl := range probs.CompactionLoops {
			var fnames []string
			for _, f := range cl.RereadFiles {
				fnames = append(fnames, filepath.Base(f))
			}
			fmt.Printf("  ! Compaction loop detected at %s:\n", cl.CompactionAt.Local().Format("15:04"))
			fmt.Printf("    Re-read after compaction: %s\n", strings.Join(fnames, ", "))
		}
		for _, mm := range probs.ModelMismatches {
			fmt.Printf("  ! Model mismatch: subagent %s uses %s (parent also uses %s).\n",
				mm.SubagentID[:min(8, len(mm.SubagentID))], mm.SubagentModel, mm.ParentModel)
		}
		if probs.ExcessiveSubagents != nil {
			fmt.Printf("  ! Excessive subagents: %d subagents spawned (threshold: %d).\n",
				probs.ExcessiveSubagents.SubagentCount, probs.ExcessiveSubagents.Threshold)
		}
		for _, cf := range probs.ContextFillNoProgress {
			where := "main session"
			if cf.SubagentID != "" {
				where = "subagent " + cf.SubagentID[:min(8, len(cf.SubagentID))]
			}
			fmt.Printf("  ! Context fill without progress (%s): %s input / %s output (%.1f%% output ratio).\n",
				where,
				formatTokens(cf.InputTokens),
				formatTokens(cf.OutputTokens),
				cf.OutputRatio*100,
			)
		}
	}

	return nil
}

// --- agents ---

func newSessionsAgentsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "agents <id>",
		Short: "Show subagent tree for a session",
		Args:  cobra.ExactArgs(1),
		RunE:  runSessionsAgents,
	}
	cmd.Flags().Bool("no-tools", false, "Omit tool call lines")
	cmd.Flags().Bool("no-warnings", false, "Suppress problem markers")
	return cmd
}

func runSessionsAgents(cmd *cobra.Command, args []string) error {
	noTools, _ := cmd.Flags().GetBool("no-tools")
	noWarnings, _ := cmd.Flags().GetBool("no-warnings")

	sum, err := findSessionByPrefix(args[0], "")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	pricingData := pricing.LoadPricing()
	s, err := sessions.ParseSession(sum.TranscriptPath, nil, pricingData.EstimateCost)
	if err != nil {
		return fmt.Errorf("parsing session: %w", err)
	}

	shortID := s.ID
	if len(shortID) > 8 {
		shortID = shortID[:8]
	}

	fmt.Printf("Session %s (%s)  %s tok  $%.2f\n",
		shortID,
		shortModelName(s.Model),
		formatTokens(s.TokenUsage.Total()),
		s.TokenUsage.EstimatedCost,
	)

	cfg := sessions.DefaultProblemConfig()
	n := len(s.Subagents)
	for i, sa := range s.Subagents {
		isLast := i == n-1
		connector := "├──"
		linePrefix := "│   "
		if isLast {
			connector = "└──"
			linePrefix = "    "
		}

		saShort := sa.SessionID
		if len(saShort) > 8 {
			saShort = saShort[:8]
		}
		name := sa.AgentName
		if name != "" {
			name = fmt.Sprintf(" %q", name)
		}

		fmt.Printf("%s %s%s (%s)\n", connector, saShort, name, shortModelName(sa.Model))

		if !noTools {
			toolCounts := make(map[string]int)
			for _, tc := range sa.ToolCalls {
				toolCounts[tc.ToolName]++
			}
			if len(toolCounts) > 0 {
				type kv struct {
					k string
					v int
				}
				var sorted []kv
				for k, v := range toolCounts {
					sorted = append(sorted, kv{k, v})
				}
				sort.Slice(sorted, func(i, j int) bool {
					return sorted[i].v > sorted[j].v
				})
				var parts []string
				for _, kv := range sorted {
					parts = append(parts, fmt.Sprintf("%s x %d", kv.k, kv.v))
				}
				fmt.Printf("%sTool calls: %s\n", linePrefix, strings.Join(parts, ", "))
			}
		}

		fmt.Printf("%sFiles: %d unique\n", linePrefix, len(sa.FileReads))
		fmt.Printf("%sCompactions: %d\n", linePrefix, len(sa.Compactions))
		fmt.Printf("%sTokens: %s in / %s out  $%.2f\n",
			linePrefix,
			formatTokens(sa.TokenUsage.InputTokens),
			formatTokens(sa.TokenUsage.OutputTokens),
			sa.TokenUsage.EstimatedCost,
		)

		if !noWarnings {
			// Run problem detection for this subagent's data.
			fakeSession := &sessions.ParsedSession{
				Model:     sa.Model,
				ToolCalls: sa.ToolCalls,
			}
			saProbs := sessions.DetectProblems(fakeSession, cfg)
			for _, rr := range saProbs.RepeatedReads {
				fmt.Printf("%s! Repeated reads: %s read %d times\n",
					linePrefix, filepath.Base(rr.FilePath), rr.ReadCount)
			}
		}
	}

	return nil
}

// --- problems ---

func newSessionsProblemsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "problems [id]",
		Short: "Show detected problems for a session or recent sessions",
		Args:  cobra.MaximumNArgs(1),
		RunE:  runSessionsProblems,
	}
	return cmd
}

func runSessionsProblems(cmd *cobra.Command, args []string) error {
	pricingData := pricing.LoadPricing()

	if len(args) == 1 {
		// Single session
		sum, err := findSessionByPrefix(args[0], "")
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		s, err := sessions.ParseSession(sum.TranscriptPath, nil, pricingData.EstimateCost)
		if err != nil {
			return fmt.Errorf("parsing session: %w", err)
		}
		printSessionProblems(s)
		return nil
	}

	// Scan recent sessions across all projects.
	projs, err := sessions.DiscoverProjects(claudeDir())
	if err != nil {
		return fmt.Errorf("discovering projects: %w", err)
	}

	found := false
	for _, proj := range projs {
		sums, err := sessions.DiscoverSessions(proj.DirPath)
		if err != nil {
			continue
		}
		limit := 5
		for i, sum := range sums {
			if i >= limit {
				break
			}
			s, err := sessions.ParseSession(sum.TranscriptPath, nil, pricingData.EstimateCost)
			if err != nil {
				continue
			}
			probs := sessions.DetectProblems(s, sessions.DefaultProblemConfig())
			hasProblems := len(probs.RepeatedReads) > 0 ||
				len(probs.CompactionLoops) > 0 ||
				len(probs.ModelMismatches) > 0 ||
				probs.ExcessiveSubagents != nil ||
				len(probs.ContextFillNoProgress) > 0
			if hasProblems {
				shortID := s.ID
				if len(shortID) > 8 {
					shortID = shortID[:8]
				}
				fmt.Printf("Session %s (%s):\n", shortID, shortenHome(s.ProjectDir))
				printProblemsDetail(probs)
				found = true
			}
		}
	}

	if !found {
		fmt.Println("No problems detected in recent sessions.")
	}
	return nil
}

func printSessionProblems(s *sessions.ParsedSession) {
	shortID := s.ID
	if len(shortID) > 8 {
		shortID = shortID[:8]
	}
	fmt.Printf("Session %s (%s):\n", shortID, shortenHome(s.ProjectDir))
	probs := sessions.DetectProblems(s, sessions.DefaultProblemConfig())
	printProblemsDetail(probs)
}

func printProblemsDetail(probs sessions.Problems) {
	hasProblems := len(probs.RepeatedReads) > 0 ||
		len(probs.CompactionLoops) > 0 ||
		len(probs.ModelMismatches) > 0 ||
		probs.ExcessiveSubagents != nil ||
		len(probs.ContextFillNoProgress) > 0

	if !hasProblems {
		fmt.Println("  No problems detected.")
		return
	}

	for _, rr := range probs.RepeatedReads {
		where := ""
		if rr.SubagentID != "" {
			where = fmt.Sprintf(" (subagent %s)", rr.SubagentID[:min(8, len(rr.SubagentID))])
		}
		fmt.Printf("  ! Repeated reads: %s read %d times%s\n",
			filepath.Base(rr.FilePath), rr.ReadCount, where)
	}
	for _, cl := range probs.CompactionLoops {
		var fnames []string
		for _, f := range cl.RereadFiles {
			fnames = append(fnames, filepath.Base(f))
		}
		fmt.Printf("  ! Compaction loop at %s: re-read %s\n",
			cl.CompactionAt.Local().Format("15:04"),
			strings.Join(fnames, ", "),
		)
	}
	for _, mm := range probs.ModelMismatches {
		fmt.Printf("  ! Model mismatch: subagent %s uses %s (parent: %s)\n",
			mm.SubagentID[:min(8, len(mm.SubagentID))], mm.SubagentModel, mm.ParentModel)
	}
	if probs.ExcessiveSubagents != nil {
		fmt.Printf("  ! Excessive subagents: %d spawned (threshold: %d)\n",
			probs.ExcessiveSubagents.SubagentCount, probs.ExcessiveSubagents.Threshold)
	}
	for _, cf := range probs.ContextFillNoProgress {
		where := "main session"
		if cf.SubagentID != "" {
			where = "subagent " + cf.SubagentID[:min(8, len(cf.SubagentID))]
		}
		fmt.Printf("  ! Context fill without progress (%s): %.1f%% output ratio\n",
			where, cf.OutputRatio*100)
	}
}

// shortModelName converts a full model string to a short display name.
func shortModelName(model string) string {
	lower := strings.ToLower(model)
	switch {
	case strings.Contains(lower, "opus"):
		return "Opus"
	case strings.Contains(lower, "sonnet"):
		return "Sonnet"
	case strings.Contains(lower, "haiku"):
		return "Haiku"
	default:
		if model == "" {
			return "unknown"
		}
		return model
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
