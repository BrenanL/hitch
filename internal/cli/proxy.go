package cli

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/BrenanL/hitch/internal/proxy"
	"github.com/BrenanL/hitch/internal/state"
	"github.com/spf13/cobra"
)

type pidInfo struct {
	PID  int `json:"pid"`
	Port int `json:"port"`
}

func newProxyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "proxy",
		Short: "API logging proxy for Claude Code",
		Long: `Transparent HTTP proxy that sits between Claude Code and the Anthropic API.
Logs every request/response to SQLite and disk with full headers, bodies, token counts,
model, cost, and latency. Detects context stripping and tool result truncation bugs.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	cmd.AddCommand(
		newProxyStartCmd(),
		newProxyStopCmd(),
		newProxyStatusCmd(),
		newProxyTailCmd(),
		newProxyStatsCmd(),
		newProxyInspectCmd(),
		newProxySessionsCmd(),
		newProxySessionCmd(),
		newProxyAnalyzeCmd(),
		newProxyInstallCmd(),
		newProxyUpdatePricingCmd(),
	)
	return cmd
}

// --- start ---

func newProxyStartCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start the logging proxy server",
		RunE:  runProxyStart,
	}
	cmd.Flags().Int("port", 9800, "Port to listen on")
	cmd.Flags().Bool("foreground", true, "Run in foreground (for systemd)")
	return cmd
}

func runProxyStart(cmd *cobra.Command, args []string) error {
	port, _ := cmd.Flags().GetInt("port")

	db, _, err := openDB()
	if err != nil {
		return err
	}
	defer db.Close()

	srv := proxy.NewServer(port, db)
	return srv.Start()
}

// --- stop ---

func newProxyStopCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stop",
		Short: "Stop the running proxy server",
		RunE:  runProxyStop,
	}
}

func runProxyStop(cmd *cobra.Command, args []string) error {
	info, err := readPIDFile()
	if err != nil {
		return fmt.Errorf("proxy not running: %w", err)
	}

	proc, err := os.FindProcess(info.PID)
	if err != nil {
		return fmt.Errorf("process %d not found: %w", info.PID, err)
	}
	if err := proc.Signal(syscall.SIGTERM); err != nil {
		return fmt.Errorf("sending SIGTERM to %d: %w", info.PID, err)
	}

	fmt.Printf("Sent SIGTERM to proxy (PID %d, port %d)\n", info.PID, info.Port)
	return nil
}

// --- status ---

func newProxyStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show proxy status",
		RunE:  runProxyStatus,
	}
}

func runProxyStatus(cmd *cobra.Command, args []string) error {
	info, err := readPIDFile()
	if err != nil {
		fmt.Println("Proxy: not running")
		return nil
	}

	proc, err := os.FindProcess(info.PID)
	if err != nil || proc.Signal(syscall.Signal(0)) != nil {
		fmt.Printf("Proxy: not running (stale PID %d)\n", info.PID)
		return nil
	}

	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(fmt.Sprintf("http://localhost:%d/health", info.Port))
	if err != nil {
		fmt.Printf("Proxy: running (PID %d) but health check failed: %v\n", info.PID, err)
		return nil
	}
	defer resp.Body.Close()

	var health struct {
		UptimeSeconds  int64 `json:"uptime_seconds"`
		RequestsLogged int64 `json:"requests_logged"`
		Port           int   `json:"port"`
	}
	json.NewDecoder(resp.Body).Decode(&health)

	uptime := time.Duration(health.UptimeSeconds) * time.Second
	fmt.Printf("Proxy: running (PID %d)\n", info.PID)
	fmt.Printf("  Port:     %d\n", health.Port)
	fmt.Printf("  Uptime:   %s\n", formatDuration(uptime))
	fmt.Printf("  Requests: %d\n", health.RequestsLogged)
	return nil
}

// --- tail ---

func newProxyTailCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tail",
		Short: "Show recent API requests",
		RunE:  runProxyTail,
	}
	cmd.Flags().IntP("number", "n", 20, "Number of requests to show")
	cmd.Flags().BoolP("verbose", "v", false, "Show full detail per request")
	cmd.Flags().String("session", "", "Filter by session ID (prefix match)")
	return cmd
}

func runProxyTail(cmd *cobra.Command, args []string) error {
	n, _ := cmd.Flags().GetInt("number")
	verbose, _ := cmd.Flags().GetBool("verbose")
	sessionFilter, _ := cmd.Flags().GetString("session")

	db, _, err := openDB()
	if err != nil {
		return err
	}
	defer db.Close()

	pricing := proxy.LoadPricing()

	requests, err := db.QueryRecentRequests(n, sessionFilter)
	if err != nil {
		return err
	}

	if len(requests) == 0 {
		fmt.Println("No requests logged yet.")
		return nil
	}

	if verbose {
		for i := len(requests) - 1; i >= 0; i-- {
			r := requests[i]
			cost := pricing.EstimateCost(r.Model, r.InputTokens, r.OutputTokens,
				r.CacheReadTokens, r.CacheCreationTokens)
			fmt.Printf("[%d] %s  %s %s %d  model=%s  in=%d out=%d cr=%d cw=%d  $%.4f  %dms  %s",
				r.ID, r.Timestamp, r.HTTPMethod, r.Endpoint, r.HTTPStatus,
				r.Model, r.InputTokens, r.OutputTokens,
				r.CacheReadTokens, r.CacheCreationTokens,
				cost, r.LatencyMS, r.StopReason)
			if r.SessionID != "" {
				fmt.Printf("  sess=%s", r.SessionID[:min(12, len(r.SessionID))])
			}
			if r.MicrocompactCount > 0 {
				fmt.Printf("  mc:%d", r.MicrocompactCount)
			}
			if r.TruncatedResults > 0 {
				fmt.Printf("  tr:%d", r.TruncatedResults)
			}
			if r.Error != "" {
				fmt.Printf("  err=%s", r.Error)
			}
			fmt.Println()
		}
		return nil
	}

	// Compact table view
	fmt.Printf("%-4s %-19s %-8s %-8s %-16s %5s %6s %7s %8s %9s %6s %-9s %s\n",
		"ID", "TIMESTAMP", "EP", "SESSION", "MODEL", "IN", "OUT", "C_READ", "C_CREATE", "COST", "MS", "STOP", "FLAGS")

	for i := len(requests) - 1; i >= 0; i-- {
		r := requests[i]
		ts := r.Timestamp
		if len(ts) > 19 {
			ts = ts[:19] // YYYY-MM-DDThh:mm:ss
		}

		ep := r.Endpoint
		if ep == "/v1/messages" {
			ep = "/v1/msg"
		}
		if len(ep) > 8 {
			ep = ep[:8]
		}

		sess := ""
		if r.SessionID != "" {
			sess = r.SessionID
			if len(sess) > 8 {
				sess = sess[:8]
			}
		}

		model := r.Model
		if strings.HasPrefix(model, "claude-") {
			model = strings.TrimPrefix(model, "claude-")
		}
		if len(model) > 16 {
			model = model[:16]
		}

		cost := pricing.EstimateCost(r.Model, r.InputTokens, r.OutputTokens,
			r.CacheReadTokens, r.CacheCreationTokens)

		flags := ""
		if r.MicrocompactCount > 0 {
			flags += fmt.Sprintf("mc:%d ", r.MicrocompactCount)
		}
		if r.TruncatedResults > 0 {
			flags += fmt.Sprintf("tr:%d ", r.TruncatedResults)
		}
		if r.Error != "" && r.StopReason == "" {
			errShort := r.Error
			if len(errShort) > 10 {
				errShort = errShort[:10]
			}
			flags += errShort
		}

		fmt.Printf("%-4d %-19s %-8s %-8s %-16s %5d %6d %7d %8d $%8.4f %6d %-9s %s\n",
			r.ID, ts, ep, sess, model, r.InputTokens, r.OutputTokens,
			r.CacheReadTokens, r.CacheCreationTokens, cost, r.LatencyMS,
			r.StopReason, flags)
	}
	return nil
}

// --- inspect ---

func newProxyInspectCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "inspect <id>",
		Short: "Show full detail for a single request",
		Args:  cobra.ExactArgs(1),
		RunE:  runProxyInspect,
	}
}

func runProxyInspect(cmd *cobra.Command, args []string) error {
	id, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil {
		return fmt.Errorf("invalid request ID: %w", err)
	}

	db, _, err := openDB()
	if err != nil {
		return err
	}
	defer db.Close()

	pricing := proxy.LoadPricing()

	r, err := db.GetRequest(id)
	if err != nil {
		return err
	}

	cost := pricing.EstimateCost(r.Model, r.InputTokens, r.OutputTokens,
		r.CacheReadTokens, r.CacheCreationTokens)

	fmt.Printf("ID:            %d\n", r.ID)
	fmt.Printf("Timestamp:     %s\n", r.Timestamp)
	fmt.Printf("Session:       %s\n", r.SessionID)
	fmt.Printf("Request ID:    %s\n", r.RequestID)
	fmt.Printf("Model:         %s\n", r.Model)
	fmt.Printf("Endpoint:      %s\n", r.Endpoint)
	fmt.Printf("HTTP:          %s %d\n", r.HTTPMethod, r.HTTPStatus)
	fmt.Printf("Streaming:     %v\n", r.Streaming)
	fmt.Printf("Latency:       %dms\n", r.LatencyMS)
	fmt.Printf("Messages:      %d\n", r.MessageCount)

	fmt.Printf("\nTokens:\n")
	fmt.Printf("  Input:         %d\n", r.InputTokens)
	fmt.Printf("  Output:        %d\n", r.OutputTokens)
	fmt.Printf("  Cache Read:    %d\n", r.CacheReadTokens)
	fmt.Printf("  Cache Create:  %d\n", r.CacheCreationTokens)
	fmt.Printf("  Cost:          $%.4f\n", cost)

	if r.MicrocompactCount > 0 || r.TruncatedResults > 0 || r.TotalToolResultSize > 0 {
		fmt.Printf("\nBug Detection:\n")
		fmt.Printf("  Microcompact:    %d\n", r.MicrocompactCount)
		fmt.Printf("  Truncated:       %d\n", r.TruncatedResults)
		fmt.Printf("  Tool Result Size: %d bytes\n", r.TotalToolResultSize)
	}

	if r.Error != "" {
		fmt.Printf("\nError: %s\n", r.Error)
	}

	if r.RequestHeaders != "" {
		fmt.Printf("\nRequest Headers:\n")
		var headers map[string][]string
		if json.Unmarshal([]byte(r.RequestHeaders), &headers) == nil {
			for k, v := range headers {
				fmt.Printf("  %s: %s\n", k, strings.Join(v, ", "))
			}
		}
	}

	if r.ResponseHeaders != "" {
		fmt.Printf("\nResponse Headers:\n")
		var headers map[string][]string
		if json.Unmarshal([]byte(r.ResponseHeaders), &headers) == nil {
			for k, v := range headers {
				fmt.Printf("  %s: %s\n", k, strings.Join(v, ", "))
			}
		}
	}

	fmt.Printf("\nBody Size:     %d bytes req / %d bytes resp\n", r.RequestBodySize, r.ResponseBodySize)
	if r.RequestLogPath != "" {
		fmt.Printf("Request Log:   %s\n", r.RequestLogPath)
	}
	if r.ResponseLogPath != "" {
		fmt.Printf("Response Log:  %s\n", r.ResponseLogPath)
	}
	return nil
}

// --- sessions ---

func newProxySessionsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sessions",
		Short: "List sessions with aggregate stats",
		RunE:  runProxySessions,
	}
	cmd.Flags().IntP("number", "n", 20, "Number of sessions to show")
	return cmd
}

func runProxySessions(cmd *cobra.Command, args []string) error {
	n, _ := cmd.Flags().GetInt("number")

	db, _, err := openDB()
	if err != nil {
		return err
	}
	defer db.Close()

	pricing := proxy.LoadPricing()

	sessions, err := db.ListSessions(n)
	if err != nil {
		return err
	}

	if len(sessions) == 0 {
		fmt.Println("No sessions found. (Session tracking requires X-Claude-Code-Session-Id header)")
		return nil
	}

	fmt.Printf("%-36s %5s %-15s %-15s %10s %10s\n",
		"SESSION", "REQS", "FIRST", "LAST", "TOKENS", "COST")

	for _, s := range sessions {
		total := s.TotalInput + s.TotalOutput + s.TotalCacheRead + s.TotalCacheCreation
		cost := pricing.EstimateCost("claude-opus-4-6",
			int(s.TotalInput), int(s.TotalOutput),
			int(s.TotalCacheRead), int(s.TotalCacheCreation))

		first := s.FirstTimestamp
		if len(first) > 15 {
			first = first[11:] // time only
			if len(first) > 8 {
				first = first[:8]
			}
		}
		last := s.LastTimestamp
		if len(last) > 15 {
			last = last[11:]
			if len(last) > 8 {
				last = last[:8]
			}
		}

		sessID := s.SessionID
		if len(sessID) > 36 {
			sessID = sessID[:36]
		}

		fmt.Printf("%-36s %5d %-15s %-15s %10d $%9.4f\n",
			sessID, s.RequestCount, first, last, total, cost)
	}
	return nil
}

// --- session (single session drill-down) ---

func newProxySessionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "session <session-id>",
		Short: "Show full transaction list for a session",
		Long:  "Displays a session summary and complete chronological transaction list. Accepts session ID prefix (first 4+ characters).",
		Args:  cobra.ExactArgs(1),
		RunE:  runProxySession,
	}
	cmd.Flags().BoolP("verbose", "v", false, "Show full detail per request")
	return cmd
}

func runProxySession(cmd *cobra.Command, args []string) error {
	verbose, _ := cmd.Flags().GetBool("verbose")
	prefix := args[0]

	db, _, err := openDB()
	if err != nil {
		return err
	}
	defer db.Close()

	pricing := proxy.LoadPricing()

	// Resolve session ID prefix
	sessions, err := db.ListSessions(500)
	if err != nil {
		return err
	}
	session, err := matchSession(sessions, prefix)
	if err != nil {
		return err
	}

	// Fetch all transactions
	requests, err := db.QuerySessionRequests(session.SessionID)
	if err != nil {
		return err
	}
	if len(requests) == 0 {
		fmt.Println("No transactions found for this session.")
		return nil
	}

	// Compute summary
	var totalInput, totalOutput, totalCacheRead, totalCacheCreate int64
	var totalCost float64
	var totalMC, totalTR int
	models := map[string]bool{}
	for _, r := range requests {
		totalInput += int64(r.InputTokens)
		totalOutput += int64(r.OutputTokens)
		totalCacheRead += int64(r.CacheReadTokens)
		totalCacheCreate += int64(r.CacheCreationTokens)
		totalCost += pricing.EstimateCost(r.Model, r.InputTokens, r.OutputTokens,
			r.CacheReadTokens, r.CacheCreationTokens)
		totalMC += r.MicrocompactCount
		totalTR += r.TruncatedResults
		if r.Model != "" {
			models[r.Model] = true
		}
	}

	cacheHit := 0.0
	totalInputAll := totalInput + totalCacheRead
	if totalInputAll > 0 {
		cacheHit = float64(totalCacheRead) / float64(totalInputAll) * 100
	}

	// Parse timestamps for duration
	first := requests[0].Timestamp
	last := requests[len(requests)-1].Timestamp
	firstTime := first
	lastTime := last
	if len(firstTime) > 8 && len(lastTime) > 8 {
		if idx := strings.Index(firstTime, "T"); idx > 0 {
			firstTime = firstTime[idx+1:]
		}
		if idx := strings.Index(lastTime, "T"); idx > 0 {
			lastTime = lastTime[idx+1:]
		}
		if len(firstTime) > 8 {
			firstTime = firstTime[:8]
		}
		if len(lastTime) > 8 {
			lastTime = lastTime[:8]
		}
	}

	var modelList []string
	for m := range models {
		modelList = append(modelList, m)
	}

	// Print session summary
	fmt.Printf("Session:  %s\n", session.SessionID)
	fmt.Printf("Duration: %s -- %s\n", firstTime, lastTime)
	fmt.Printf("Requests: %d     Cost: $%.4f     Cache Hit: %.1f%%\n\n",
		len(requests), totalCost, cacheHit)

	fmt.Printf("Tokens:\n")
	fmt.Printf("  Input:         %d\n", totalInput)
	fmt.Printf("  Output:        %d\n", totalOutput)
	fmt.Printf("  Cache Read:    %d\n", totalCacheRead)
	fmt.Printf("  Cache Create:  %d\n", totalCacheCreate)

	if len(modelList) > 0 {
		fmt.Printf("\nModels:  %s\n", strings.Join(modelList, ", "))
	}

	if totalMC > 0 || totalTR > 0 {
		fmt.Printf("\nBugs:\n")
		if totalMC > 0 {
			fmt.Printf("  Microcompact:  %d\n", totalMC)
		}
		if totalTR > 0 {
			fmt.Printf("  Truncated:     %d\n", totalTR)
		}
	}

	fmt.Println()

	// Print transaction table
	if verbose {
		for i, r := range requests {
			cost := pricing.EstimateCost(r.Model, r.InputTokens, r.OutputTokens,
				r.CacheReadTokens, r.CacheCreationTokens)
			fmt.Printf("[%d] %s  %s %s %d  model=%s  in=%d out=%d cr=%d cw=%d  $%.4f  %dms  %s",
				i+1, r.Timestamp, r.HTTPMethod, r.Endpoint, r.HTTPStatus,
				r.Model, r.InputTokens, r.OutputTokens,
				r.CacheReadTokens, r.CacheCreationTokens,
				cost, r.LatencyMS, r.StopReason)
			if r.MicrocompactCount > 0 {
				fmt.Printf("  mc:%d", r.MicrocompactCount)
			}
			if r.TruncatedResults > 0 {
				fmt.Printf("  tr:%d", r.TruncatedResults)
			}
			if r.Error != "" {
				fmt.Printf("  err=%s", r.Error)
			}
			fmt.Println()
		}
		return nil
	}

	// Compact table
	fmt.Printf("%4s  %-8s  %-16s %5s %6s %7s %8s %9s %6s %-9s %s\n",
		"#", "TIME", "MODEL", "IN", "OUT", "C_READ", "C_CREATE", "COST", "MS", "STOP", "FLAGS")

	for i, r := range requests {
		ts := r.Timestamp
		if idx := strings.Index(ts, "T"); idx > 0 {
			ts = ts[idx+1:]
		}
		if len(ts) > 8 {
			ts = ts[:8]
		}

		model := r.Model
		if strings.HasPrefix(model, "claude-") {
			model = strings.TrimPrefix(model, "claude-")
		}
		if len(model) > 16 {
			model = model[:16]
		}

		cost := pricing.EstimateCost(r.Model, r.InputTokens, r.OutputTokens,
			r.CacheReadTokens, r.CacheCreationTokens)

		flags := ""
		if r.MicrocompactCount > 0 {
			flags += fmt.Sprintf("mc:%d ", r.MicrocompactCount)
		}
		if r.TruncatedResults > 0 {
			flags += fmt.Sprintf("tr:%d ", r.TruncatedResults)
		}
		if r.Error != "" && r.StopReason == "" {
			errShort := r.Error
			if len(errShort) > 12 {
				errShort = errShort[:12]
			}
			flags += errShort
		}

		fmt.Printf("%4d  %-8s  %-16s %5d %6d %7d %8d $%8.4f %6d %-9s %s\n",
			i+1, ts, model, r.InputTokens, r.OutputTokens,
			r.CacheReadTokens, r.CacheCreationTokens, cost, r.LatencyMS,
			r.StopReason, flags)
	}
	return nil
}

func matchSession(sessions []state.SessionInfo, prefix string) (state.SessionInfo, error) {
	var matches []state.SessionInfo
	for _, s := range sessions {
		if strings.HasPrefix(s.SessionID, prefix) {
			matches = append(matches, s)
		}
	}
	switch len(matches) {
	case 0:
		return state.SessionInfo{}, fmt.Errorf("no session found matching %q", prefix)
	case 1:
		return matches[0], nil
	default:
		var ids []string
		for _, m := range matches {
			id := m.SessionID
			if len(id) > 20 {
				id = id[:20] + "..."
			}
			ids = append(ids, id)
		}
		return state.SessionInfo{}, fmt.Errorf("ambiguous prefix %q matches %d sessions:\n  %s\nProvide more characters to disambiguate",
			prefix, len(matches), strings.Join(ids, "\n  "))
	}
}

// --- analyze ---

func newProxyAnalyzeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "analyze <id>",
		Short: "Analyze request body content composition",
		Long: `Reads the transaction log file for a request and produces a detailed breakdown
of the content: system prompt, messages, tool definitions, tool uses/results,
file reads, and a composition percentage. Use --json for structured output.`,
		Args: cobra.ExactArgs(1),
		RunE: runProxyAnalyze,
	}
	cmd.Flags().Bool("json", false, "Output as structured JSON (for agent consumption)")
	return cmd
}

func runProxyAnalyze(cmd *cobra.Command, args []string) error {
	id, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil {
		return fmt.Errorf("invalid request ID: %w", err)
	}
	jsonOutput, _ := cmd.Flags().GetBool("json")

	db, _, err := openDB()
	if err != nil {
		return err
	}
	defer db.Close()

	req, err := db.GetRequest(id)
	if err != nil {
		return fmt.Errorf("request %d not found: %w", id, err)
	}

	if req.RequestLogPath == "" {
		return fmt.Errorf("no request log file for request %d", id)
	}

	analysis, err := proxy.AnalyzeRequestBody(req.RequestLogPath)
	if err != nil {
		return fmt.Errorf("analyzing request: %w", err)
	}

	if jsonOutput {
		data, _ := json.MarshalIndent(analysis, "", "  ")
		fmt.Println(string(data))
		return nil
	}

	// Human-readable output
	fmt.Printf("Request #%d Analysis\n", id)
	fmt.Printf("Model: %s\n\n", analysis.Model)

	fmt.Printf("System Prompt:\n")
	fmt.Printf("  Blocks:     %d\n", analysis.System.BlockCount)
	fmt.Printf("  Size:       %s\n", formatBytes(analysis.System.TotalSizeBytes))
	if len(analysis.System.Types) > 0 {
		fmt.Printf("  Types:      ")
		i := 0
		for typ, count := range analysis.System.Types {
			if i > 0 {
				fmt.Printf(", ")
			}
			fmt.Printf("%s(%d)", typ, count)
			i++
		}
		fmt.Println()
	}

	fmt.Printf("\nMessages:\n")
	fmt.Printf("  Total:      %d\n", analysis.Messages.Total)
	fmt.Printf("  Turns:      %d\n", analysis.Messages.ConversationTurns)
	fmt.Printf("  Size:       %s\n", formatBytes(analysis.Messages.TotalSizeBytes))
	if len(analysis.Messages.ByRole) > 0 {
		fmt.Printf("  By role:    ")
		i := 0
		for role, count := range analysis.Messages.ByRole {
			if i > 0 {
				fmt.Printf(", ")
			}
			fmt.Printf("%s(%d)", role, count)
			i++
		}
		fmt.Println()
	}

	if analysis.Tools.Count > 0 {
		fmt.Printf("\nTool Definitions: %d\n", analysis.Tools.Count)
		fmt.Printf("  Size:       %s\n", formatBytes(analysis.Tools.TotalSizeBytes))
		if len(analysis.Tools.Names) > 0 {
			max := 15
			names := analysis.Tools.Names
			if len(names) > max {
				names = append(names[:max], fmt.Sprintf("... +%d more", len(analysis.Tools.Names)-max))
			}
			fmt.Printf("  Names:      %s\n", strings.Join(names, ", "))
		}
	}

	if analysis.ToolUses.Count > 0 {
		fmt.Printf("\nTool Uses: %d\n", analysis.ToolUses.Count)
		for tool, count := range analysis.ToolUses.ByTool {
			fmt.Printf("  %-20s %d\n", tool, count)
		}
	}

	if analysis.ToolResults.Count > 0 {
		fmt.Printf("\nTool Results: %d\n", analysis.ToolResults.Count)
		fmt.Printf("  Total size: %s\n", formatBytes(analysis.ToolResults.TotalSizeBytes))
		fmt.Printf("  Avg size:   %s\n", formatBytes(analysis.ToolResults.AvgSizeBytes))
	}

	if len(analysis.FileReads) > 0 {
		fmt.Printf("\nFile Reads: %d\n", len(analysis.FileReads))
		max := 20
		for i, f := range analysis.FileReads {
			if i >= max {
				fmt.Printf("  ... +%d more\n", len(analysis.FileReads)-max)
				break
			}
			fmt.Printf("  %s\n", f)
		}
	}

	fmt.Printf("\nComposition:\n")
	fmt.Printf("  System:       %5.1f%%\n", analysis.Composition.SystemPercent)
	fmt.Printf("  Conversation: %5.1f%%\n", analysis.Composition.ConversationPercent)
	fmt.Printf("  Tool Results: %5.1f%%\n", analysis.Composition.ToolResultPercent)
	fmt.Printf("  Tool Defs:    %5.1f%%\n", analysis.Composition.ToolDefPercent)

	return nil
}

func formatBytes(b int) string {
	switch {
	case b >= 1_000_000:
		return fmt.Sprintf("%.1f MB", float64(b)/1_000_000)
	case b >= 1_000:
		return fmt.Sprintf("%.1f KB", float64(b)/1_000)
	default:
		return fmt.Sprintf("%d B", b)
	}
}

// --- stats ---

func newProxyStatsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stats",
		Short: "Show aggregate API usage statistics",
		RunE:  runProxyStats,
	}
	cmd.Flags().String("since", "24h", "Time window (e.g. 1h, 24h, 7d)")
	cmd.Flags().Bool("today", false, "Show today's stats")
	cmd.Flags().String("session", "", "Filter by session ID")
	return cmd
}

func runProxyStats(cmd *cobra.Command, args []string) error {
	db, _, err := openDB()
	if err != nil {
		return err
	}
	defer db.Close()

	pricing := proxy.LoadPricing()

	var since, sessionID string
	if s, _ := cmd.Flags().GetString("session"); s != "" {
		sessionID = s
		since = "1970-01-01"
	} else if today, _ := cmd.Flags().GetBool("today"); today {
		since = time.Now().UTC().Format("2006-01-02")
	} else {
		dur, _ := cmd.Flags().GetString("since")
		d, err := parseDuration(dur)
		if err != nil {
			return fmt.Errorf("invalid duration %q: %w", dur, err)
		}
		since = time.Now().UTC().Add(-d).Format("2006-01-02T15:04:05")
	}

	stats, err := db.GetProxyStats(since, sessionID)
	if err != nil {
		return err
	}

	if stats.TotalRequests == 0 {
		fmt.Println("No requests in this time window.")
		return nil
	}

	totalCost := pricing.EstimateCost("claude-opus-4-6",
		int(stats.TotalInputTokens), int(stats.TotalOutputTokens),
		int(stats.TotalCacheRead), int(stats.TotalCacheCreation))

	fmt.Printf("Requests:        %d\n", stats.TotalRequests)
	fmt.Printf("Input tokens:    %d\n", stats.TotalInputTokens)
	fmt.Printf("Output tokens:   %d\n", stats.TotalOutputTokens)
	fmt.Printf("Cache read:      %d\n", stats.TotalCacheRead)
	fmt.Printf("Cache creation:  %d\n", stats.TotalCacheCreation)
	fmt.Printf("Cache hit rate:  %.1f%%\n", stats.CacheHitRate)
	fmt.Printf("Est. cost:       $%.4f\n", totalCost)
	fmt.Printf("Avg latency:     %.0fms\n", stats.AvgLatencyMS)
	if stats.TotalMicrocompacts > 0 {
		fmt.Printf("Microcompacts:   %d (context stripping detected)\n", stats.TotalMicrocompacts)
	}
	if stats.TotalTruncated > 0 {
		fmt.Printf("Truncated:       %d (budget cap active)\n", stats.TotalTruncated)
	}

	models, err := db.GetProxyStatsByModel(since, sessionID)
	if err != nil {
		return err
	}

	if len(models) > 0 {
		fmt.Println()
		fmt.Printf("  %-28s %6s %10s %10s\n", "MODEL", "REQS", "TOKENS", "COST")
		for _, m := range models {
			totalTokens := m.InputTokens + m.OutputTokens + m.CacheReadTokens + m.CacheCreationTokens
			cost := pricing.EstimateCost(m.Model,
				int(m.InputTokens), int(m.OutputTokens),
				int(m.CacheReadTokens), int(m.CacheCreationTokens))
			fmt.Printf("  %-28s %6d %10d $%9.4f\n", m.Model, m.Requests, totalTokens, cost)
		}
	}
	return nil
}

// --- install ---

func newProxyInstallCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "install",
		Short: "Generate systemd service and print installation steps",
		RunE:  runProxyInstall,
	}
}

func runProxyInstall(cmd *cobra.Command, args []string) error {
	home, _ := os.UserHomeDir()

	htBinary, err := filepath.Abs(os.Args[0])
	if err != nil {
		htBinary = filepath.Join(home, "dev", "hitch", "ht")
	}
	if strings.Contains(htBinary, "go-build") || strings.Contains(htBinary, "/tmp/") {
		htBinary = filepath.Join(home, "dev", "hitch", "ht")
	}

	unitDir := filepath.Join(home, ".config", "systemd", "user")
	unitPath := filepath.Join(unitDir, "hitch-proxy.service")

	unit := fmt.Sprintf(`[Unit]
Description=Hitch Proxy for Claude Code API Logging

[Service]
ExecStart=%s proxy start --foreground
Restart=always
RestartSec=2
Environment=HOME=%s

[Install]
WantedBy=default.target
`, htBinary, home)

	if err := os.MkdirAll(unitDir, 0o755); err != nil {
		return fmt.Errorf("creating systemd directory: %w", err)
	}
	if err := os.WriteFile(unitPath, []byte(unit), 0o644); err != nil {
		return fmt.Errorf("writing systemd unit: %w", err)
	}

	fmt.Printf("Wrote systemd unit: %s\n", unitPath)
	fmt.Printf("Binary path:        %s\n\n", htBinary)

	cwd, _ := os.Getwd()
	fmt.Println("=== Installation Steps ===")
	fmt.Println()
	fmt.Println("1. Build the binary:")
	fmt.Printf("   cd %s && go build -o ht ./cmd/ht\n\n", cwd)
	fmt.Println("2. Enable and start the systemd service:")
	fmt.Println("   systemctl --user daemon-reload")
	fmt.Println("   systemctl --user enable hitch-proxy")
	fmt.Println("   systemctl --user start hitch-proxy")
	fmt.Println()
	fmt.Println("3. Add env vars to ~/.claude/settings.json:")
	fmt.Println(`   "env": {`)
	fmt.Println(`     "ANTHROPIC_BASE_URL": "http://localhost:9800",`)
	fmt.Println(`     "CLAUDE_CODE_PROXY_RESOLVES_HOSTS": "1"`)
	fmt.Println(`   }`)
	fmt.Println()
	fmt.Println("4. Verify:")
	fmt.Println("   ht proxy status")
	fmt.Println("   curl http://localhost:9800/health")
	fmt.Println()
	fmt.Println("5. Test:")
	fmt.Println("   claude -p 'say hello'")
	fmt.Println("   ht proxy tail -n 5")
	return nil
}

// --- update-pricing ---

func newProxyUpdatePricingCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "update-pricing",
		Short: "Fetch latest pricing from LiteLLM and update ~/.hitch/pricing.json",
		RunE:  runProxyUpdatePricing,
	}
}

func runProxyUpdatePricing(cmd *cobra.Command, args []string) error {
	fmt.Println("Fetching pricing from LiteLLM...")

	pricing, err := proxy.FetchLiteLLMPricing()
	if err != nil {
		fmt.Printf("Fetch failed: %v\n", err)
		fmt.Println("Seeding from built-in defaults instead.")
		pricing = proxy.DefaultPricing()
	} else {
		fmt.Printf("Found %d Claude models.\n", len(pricing))
	}

	if err := proxy.WritePricingFile(pricing); err != nil {
		return fmt.Errorf("writing pricing file: %w", err)
	}

	path := proxy.PricingFilePath()
	fmt.Printf("Wrote %s\n", path)

	for model, p := range pricing {
		if strings.Contains(model, "opus") || strings.Contains(model, "sonnet-4-6") {
			fmt.Printf("  %-30s in=$%.2f out=$%.2f cw=$%.2f cr=$%.2f\n",
				model, p.Input, p.Output, p.CacheWrite, p.CacheRead)
		}
	}
	return nil
}

// --- helpers ---

func readPIDFile() (*pidInfo, error) {
	home, _ := os.UserHomeDir()
	data, err := os.ReadFile(filepath.Join(home, ".hitch", "proxy.pid"))
	if err != nil {
		return nil, err
	}
	var info pidInfo
	if err := json.Unmarshal(data, &info); err != nil {
		pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
		if err != nil {
			return nil, fmt.Errorf("invalid PID file")
		}
		return &pidInfo{PID: pid, Port: 9800}, nil
	}
	return &info, nil
}

func parseDuration(s string) (time.Duration, error) {
	if strings.HasSuffix(s, "d") {
		days, err := strconv.Atoi(strings.TrimSuffix(s, "d"))
		if err != nil {
			return 0, err
		}
		return time.Duration(days) * 24 * time.Hour, nil
	}
	return time.ParseDuration(s)
}

func formatDuration(d time.Duration) string {
	d = d.Truncate(time.Second)
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60
	if h > 0 {
		return fmt.Sprintf("%dh%dm%ds", h, m, s)
	}
	if m > 0 {
		return fmt.Sprintf("%dm%ds", m, s)
	}
	return fmt.Sprintf("%ds", s)
}
