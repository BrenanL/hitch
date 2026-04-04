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
Logs every request/response to SQLite with token counts, model, cost, and latency.
Detects context stripping (microcompact) and tool result truncation bugs.`,
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
		newProxyInstallCmd(),
	)
	return cmd
}

// --- start ---

func newProxyStartCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start the logging proxy server",
		Long: `Start the transparent HTTP proxy that logs all Claude Code API calls to SQLite.
Runs in the foreground — use systemd for background operation (see: ht proxy install).`,
		RunE: runProxyStart,
	}
	cmd.Flags().Int("port", 9800, "Port to listen on")
	cmd.Flags().Bool("foreground", true, "Run in foreground (default, for systemd compatibility)")
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

	// Check if process is alive
	proc, err := os.FindProcess(info.PID)
	if err != nil || proc.Signal(syscall.Signal(0)) != nil {
		fmt.Printf("Proxy: not running (stale PID %d)\n", info.PID)
		return nil
	}

	// Call health endpoint
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
	return cmd
}

func runProxyTail(cmd *cobra.Command, args []string) error {
	n, _ := cmd.Flags().GetInt("number")

	db, _, err := openDB()
	if err != nil {
		return err
	}
	defer db.Close()

	requests, err := db.QueryRecentRequests(n)
	if err != nil {
		return err
	}

	if len(requests) == 0 {
		fmt.Println("No requests logged yet.")
		return nil
	}

	fmt.Printf("%-20s %-22s %7s %7s %7s %9s %6s %s\n",
		"TIMESTAMP", "MODEL", "IN", "OUT", "CACHE", "COST", "MS", "STOP")

	// Print oldest first
	for i := len(requests) - 1; i >= 0; i-- {
		r := requests[i]
		ts := r.Timestamp
		if len(ts) > 19 {
			ts = ts[:19]
		}
		model := r.Model
		if len(model) > 22 {
			model = model[:22]
		}

		flags := ""
		if r.MicrocompactCount > 0 {
			flags += fmt.Sprintf(" mc:%d", r.MicrocompactCount)
		}
		if r.TruncatedResults > 0 {
			flags += fmt.Sprintf(" tr:%d", r.TruncatedResults)
		}
		if r.Error != "" {
			flags += " ERR"
		}

		fmt.Printf("%-20s %-22s %7d %7d %7d $%8.4f %6d %s%s\n",
			ts, model, r.InputTokens, r.OutputTokens, r.CacheReadTokens,
			r.CostUSD, r.LatencyMS, r.StopReason, flags)
	}
	return nil
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

	fmt.Printf("Requests:        %d\n", stats.TotalRequests)
	fmt.Printf("Input tokens:    %d\n", stats.TotalInputTokens)
	fmt.Printf("Output tokens:   %d\n", stats.TotalOutputTokens)
	fmt.Printf("Cache read:      %d\n", stats.TotalCacheRead)
	fmt.Printf("Cache creation:  %d\n", stats.TotalCacheCreation)
	fmt.Printf("Cache hit rate:  %.1f%%\n", stats.CacheHitRate)
	fmt.Printf("Total cost:      $%.4f\n", stats.TotalCostUSD)
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
			fmt.Printf("  %-28s %6d %10d $%9.4f\n", m.Model, m.Requests, m.Tokens, m.CostUSD)
		}
	}

	return nil
}

// --- install ---

func newProxyInstallCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "install",
		Short: "Generate systemd service and print installation steps",
		Long: `Generates the systemd user service file for always-on proxy operation.
Does NOT modify global Claude settings — prints manual steps instead.`,
		RunE: runProxyInstall,
	}
}

func runProxyInstall(cmd *cobra.Command, args []string) error {
	home, _ := os.UserHomeDir()

	// Resolve binary path
	htBinary, err := filepath.Abs(os.Args[0])
	if err != nil {
		htBinary = filepath.Join(home, "dev", "hitch", "ht")
	}
	// If running via "go run", fall back to project binary path
	if strings.Contains(htBinary, "go-build") || strings.Contains(htBinary, "/tmp/") {
		htBinary = filepath.Join(home, "dev", "hitch", "ht")
	}

	// Write systemd unit file
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

	fmt.Println("=== Installation Steps ===")
	fmt.Println()

	cwd, _ := os.Getwd()
	fmt.Println("1. Build the binary (if not already built):")
	fmt.Printf("   cd %s && go build -o ht ./cmd/ht\n\n", cwd)

	fmt.Println("2. Enable and start the systemd service:")
	fmt.Println("   systemctl --user daemon-reload")
	fmt.Println("   systemctl --user enable hitch-proxy")
	fmt.Println("   systemctl --user start hitch-proxy")
	fmt.Println()

	fmt.Println("3. Add env vars to ~/.claude/settings.json:")
	fmt.Println(`   {`)
	fmt.Println(`     "env": {`)
	fmt.Println(`       "ANTHROPIC_BASE_URL": "http://localhost:9800",`)
	fmt.Println(`       "CLAUDE_CODE_PROXY_RESOLVES_HOSTS": "1"`)
	fmt.Println(`     }`)
	fmt.Println(`   }`)
	fmt.Println()

	fmt.Println("4. (Optional) Add SessionStart health check hook:")
	fmt.Println(`   In the "hooks" section of settings.json:`)
	fmt.Println(`   "SessionStart": [{`)
	fmt.Println(`     "matcher": "",`)
	fmt.Println(`     "hooks": [{`)
	fmt.Println(`       "type": "command",`)
	fmt.Printf(`       "command": "curl -sf http://localhost:9800/health >/dev/null 2>&1 || echo '{\"warning\": \"Hitch proxy not running\"}'"`)
	fmt.Println()
	fmt.Println(`     }]`)
	fmt.Println(`   }]`)
	fmt.Println()

	fmt.Println("5. Verify:")
	fmt.Println("   ht proxy status")
	fmt.Println("   curl http://localhost:9800/health")
	fmt.Println()

	fmt.Println("6. Test with Claude Code:")
	fmt.Println("   claude -p 'say hello'")
	fmt.Println("   ht proxy tail -n 5")
	fmt.Println()

	fmt.Println("To bypass proxy temporarily (one session):")
	fmt.Println("   ANTHROPIC_BASE_URL=https://api.anthropic.com claude -p '...'")
	fmt.Println()

	fmt.Println("To check logs:")
	fmt.Println("   journalctl --user -u hitch-proxy -f")

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
		// Backward compat: try plain PID number
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
