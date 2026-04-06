package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/BrenanL/hitch/internal/pricing"
	"github.com/BrenanL/hitch/internal/proxy"
	"github.com/BrenanL/hitch/pkg/sessions"
	"github.com/spf13/cobra"
)

func newAutopsyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "autopsy <session-id>",
		Short: "Deep inspection of a session combining transcript and proxy data",
		Long: `Combines session transcript analysis with proxy request data to produce
a chronological breakdown: expensive turns, file reads driving context growth,
compaction events, subagent costs, and total session economics.`,
		Args: cobra.ExactArgs(1),
		RunE: runAutopsy,
	}
	cmd.Flags().String("project", "", "Narrow search to one project")
	return cmd
}

func runAutopsy(cmd *cobra.Command, args []string) error {
	projectFilter, _ := cmd.Flags().GetString("project")

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

	fmt.Printf("Autopsy: session %s — %s\n", shortID, shortenHome(s.ProjectDir))
	fmt.Println(strings.Repeat("=", 60))

	// --- Header / economics ---
	started := ""
	dur := ""
	if !s.StartedAt.IsZero() {
		started = s.StartedAt.Local().Format("2006-01-02 15:04")
	}
	if !s.StartedAt.IsZero() && !s.EndedAt.IsZero() {
		dur = formatSessionDuration(s.EndedAt.Sub(s.StartedAt))
	}
	fmt.Printf("Started: %s | Duration: %s | Model: %s\n", started, dur, s.Model)
	fmt.Println()

	u := s.TokenUsage
	fmt.Println("Session Economics")
	fmt.Println("-----------------")
	fmt.Printf("  Input:        %s tokens\n", formatTokens(u.InputTokens))
	fmt.Printf("  Output:       %s tokens\n", formatTokens(u.OutputTokens))
	fmt.Printf("  Cache read:   %s tokens\n", formatTokens(u.CacheReadTokens))
	fmt.Printf("  Cache write:  %s tokens\n", formatTokens(u.CacheCreationTokens))
	fmt.Printf("  Total cost:   $%.4f estimated\n", u.EstimatedCost)
	fmt.Println()

	// --- Proxy data (optional) ---
	db, _, dbErr := openDB()
	var proxyRequests []apiRequestSummary
	if dbErr == nil {
		defer db.Close()
		reqs, qErr := db.QuerySessionRequests(s.ID)
		if qErr == nil && len(reqs) > 0 {
			for i, r := range reqs {
				ts := r.Timestamp
				if len(ts) > 16 {
					ts = ts[:16]
				}
				cost := pricingData.EstimateCost(r.Model, r.InputTokens, r.OutputTokens, r.CacheReadTokens, r.CacheCreationTokens)
				proxyRequests = append(proxyRequests, apiRequestSummary{
					index:               i + 1,
					timestamp:           ts,
					requestID:           r.RequestID,
					model:               r.Model,
					inputTokens:         r.InputTokens,
					outputTokens:        r.OutputTokens,
					cacheReadTokens:     r.CacheReadTokens,
					cacheCreationTokens: r.CacheCreationTokens,
					latencyMS:           r.LatencyMS,
					messageCount:        r.MessageCount,
					microcompactCount:   r.MicrocompactCount,
					truncatedResults:    r.TruncatedResults,
					requestLogPath:      r.RequestLogPath,
					cost:                cost,
				})
			}
		}
	}

	// --- Compactions ---
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

	// --- Chronological proxy request breakdown ---
	if len(proxyRequests) > 0 {
		fmt.Printf("API Requests (%d total via proxy)\n", len(proxyRequests))
		fmt.Println(strings.Repeat("-", 60))
		fmt.Printf("  %-4s %-16s %-8s %-8s %-8s %-8s %s\n",
			"#", "Time", "Msgs", "Input", "Output", "Cost", "Flags")
		for _, r := range proxyRequests {
			flags := ""
			if r.microcompactCount > 0 {
				flags += fmt.Sprintf("[compact x%d]", r.microcompactCount)
			}
			if r.truncatedResults > 0 {
				flags += fmt.Sprintf("[trunc x%d]", r.truncatedResults)
			}
			fmt.Printf("  %-4d %-16s %-8d %-8s %-8s $%-7.4f %s\n",
				r.index,
				r.timestamp,
				r.messageCount,
				formatTokens(r.inputTokens),
				formatTokens(r.outputTokens),
				r.cost,
				flags,
			)
		}
		fmt.Println()

		// Top 5 most expensive requests
		sorted := make([]apiRequestSummary, len(proxyRequests))
		copy(sorted, proxyRequests)
		sort.Slice(sorted, func(i, j int) bool {
			return sorted[i].cost > sorted[j].cost
		})
		top := sorted
		if len(top) > 5 {
			top = top[:5]
		}
		fmt.Println("Top expensive requests:")
		for _, r := range top {
			fmt.Printf("  #%-3d  $%.4f  %s input / %s output  (msg #%d)\n",
				r.index, r.cost,
				formatTokens(r.inputTokens),
				formatTokens(r.outputTokens),
				r.messageCount,
			)
		}
		fmt.Println()

		// File reads driving context growth: aggregate from proxy request breakdowns
		type fileEntry struct {
			path  string
			reads int
			chars int
		}
		fileMap := map[string]*fileEntry{}
		for _, r := range proxyRequests {
			if r.requestLogPath == "" {
				continue
			}
			bd, err := proxy.ParseRequestFile(r.requestLogPath, r.inputTokens, r.cacheReadTokens, r.cacheCreationTokens)
			if err != nil {
				continue
			}
			for _, fr := range bd.FileReads {
				fe := fileMap[fr.FilePath]
				if fe == nil {
					fe = &fileEntry{path: fr.FilePath}
					fileMap[fr.FilePath] = fe
				}
				fe.reads++
				fe.chars += fr.CharCount
			}
		}

		if len(fileMap) > 0 {
			var fes []fileEntry
			for _, fe := range fileMap {
				fes = append(fes, *fe)
			}
			sort.Slice(fes, func(i, j int) bool {
				if fes[i].chars != fes[j].chars {
					return fes[i].chars > fes[j].chars
				}
				return fes[i].path < fes[j].path
			})
			maxShow := 10
			if len(fes) < maxShow {
				maxShow = len(fes)
			}
			fmt.Println("File reads driving context growth (by bytes):")
			for _, fe := range fes[:maxShow] {
				name := filepath.Base(fe.path)
				fmt.Printf("  %-40s  %dx  ~%s chars\n",
					name, fe.reads, formatTokens(fe.chars))
			}
			fmt.Println()
		}
	} else {
		fmt.Println("Proxy data: not available for this session")
		fmt.Println("  (run 'ht proxy start' to capture future sessions)")
		fmt.Println()

		// Fall back to transcript file reads
		if len(s.FileReadCounts) > 0 {
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
			fmt.Println("Top file reads (from transcript):")
			for _, f := range fcs[:maxShow] {
				fmt.Printf("  %-60s  %dx\n", f.path, f.count)
			}
			fmt.Println()
		}
	}

	// --- Subagent costs ---
	if len(s.Subagents) > 0 {
		fmt.Printf("Subagents: %d\n", len(s.Subagents))
		fmt.Printf("  %-10s %-20s %-8s %-8s %-8s %s\n",
			"ID", "Name", "Model", "Tokens", "Cost", "Compacts")
		var subTotal float64
		for _, sa := range s.Subagents {
			saShort := sa.SessionID
			if len(saShort) > 8 {
				saShort = saShort[:8]
			}
			name := sa.AgentName
			if name == "" {
				name = "-"
			}
			subTotal += sa.TokenUsage.EstimatedCost
			fmt.Printf("  %-10s %-20s %-8s %-8s $%-7.4f %d\n",
				saShort,
				truncate(name, 18),
				shortModelName(sa.Model),
				formatTokens(sa.TokenUsage.Total()),
				sa.TokenUsage.EstimatedCost,
				len(sa.Compactions),
			)
		}
		fmt.Printf("  Subagent total: $%.4f\n", subTotal)
		fmt.Printf("  Parent cost:    $%.4f\n", u.EstimatedCost)
		fmt.Printf("  Grand total:    $%.4f\n", u.EstimatedCost+subTotal)
		fmt.Println()
	}

	// --- Problems ---
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
			fmt.Printf("  ! Context fill without progress (%s): %s input / %s output (%.1f%% output ratio)\n",
				where,
				formatTokens(cf.InputTokens),
				formatTokens(cf.OutputTokens),
				cf.OutputRatio*100,
			)
		}
	}

	return nil
}

// apiRequestSummary is a lightweight struct for displaying proxy requests.
type apiRequestSummary struct {
	index               int
	timestamp           string
	requestID           string
	model               string
	inputTokens         int
	outputTokens        int
	cacheReadTokens     int
	cacheCreationTokens int
	latencyMS           int64
	messageCount        int
	microcompactCount   int
	truncatedResults    int
	requestLogPath      string
	cost                float64
}
