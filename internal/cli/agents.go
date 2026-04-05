package cli

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/BrenanL/hitch/internal/daemon"
	"github.com/spf13/cobra"
)

func newAgentsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "agents",
		Short: "List active and recent Claude Code sessions",
		RunE:  runAgents,
	}
	cmd.Flags().Bool("all", false, "Show all sessions from last 24 hours")
	cmd.Flags().Bool("json", false, "Output as JSON array")
	return cmd
}

func runAgents(cmd *cobra.Command, args []string) error {
	jsonFlag, _ := cmd.Flags().GetBool("json")
	allFlag, _ := cmd.Flags().GetBool("all")

	sessions, err := fetchSessions(allFlag)
	if err != nil {
		return err
	}

	if jsonFlag {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(sessions)
	}

	if len(sessions) == 0 {
		fmt.Println("No active sessions")
		return nil
	}

	// Print table header
	fmt.Printf(" %-7s %-12s %-25s %-14s %10s %12s %8s\n",
		"ACTIVE", "SESSION", "PROJECT", "MODEL", "BURN RATE", "TOKENS", "COST")

	for _, s := range sessions {
		active := "●"
		suffix := ""
		if !s.IsActive {
			active = "○"
			suffix = fmt.Sprintf("  (%s idle)", timeSinceISO(s.LastActivity))
		}

		burnRate := "—"
		if s.IsActive && s.BurnRateTPM > 0 {
			burnRate = fmt.Sprintf("%s/min", formatTokenCount(int(s.BurnRateTPM)))
		}

		model := shortModelName(s.Model)
		project := truncateAgentPath(s.ProjectDir, 25)
		sessionID := s.SessionID
		if len(sessionID) > 12 {
			sessionID = sessionID[:12]
		}

		fmt.Printf(" %-7s %-12s %-25s %-14s %10s %12s %8s%s\n",
			active, sessionID, project, model,
			burnRate, formatTokenCount(s.TotalTokens),
			fmt.Sprintf("$%.2f", s.TotalCostUSD), suffix)
	}

	return nil
}

func fetchSessions(all bool) ([]daemon.SessionSummary, error) {
	// Try daemon API first
	client := &http.Client{Timeout: 2 * time.Second}
	url := "http://127.0.0.1:9801/api/sessions"
	if !all {
		url += "?active=true"
	}

	resp, err := client.Get(url)
	if err != nil {
		// Fallback: read directly from SQLite
		return fetchSessionsFallback()
	}
	defer resp.Body.Close()

	var sessions []daemon.SessionSummary
	json.NewDecoder(resp.Body).Decode(&sessions)
	return sessions, nil
}

func fetchSessionsFallback() ([]daemon.SessionSummary, error) {
	db, _, err := openDB()
	if err != nil {
		return nil, err
	}
	defer db.Close()

	dbSessions, err := db.ListSessions(50)
	if err != nil {
		return nil, err
	}

	var result []daemon.SessionSummary
	for _, s := range dbSessions {
		result = append(result, daemon.SessionSummary{
			SessionID:    s.SessionID,
			IsActive:     false,
			TotalTokens:  int(s.TotalInput + s.TotalOutput),
			RequestCount: s.RequestCount,
			LastActivity: s.LastTimestamp,
		})
	}

	if len(result) == 0 {
		fmt.Fprintln(os.Stderr, "(daemon not running — showing cached sessions from SQLite)")
	}
	return result, nil
}

func truncateAgentPath(p string, maxLen int) string {
	if len(p) <= maxLen {
		return p
	}
	home, _ := os.UserHomeDir()
	if strings.HasPrefix(p, home) {
		p = "~" + p[len(home):]
	}
	if len(p) <= maxLen {
		return p
	}
	return "..." + p[len(p)-maxLen+3:]
}

func timeSinceISO(isoTime string) string {
	t, err := time.Parse(time.RFC3339, isoTime)
	if err != nil {
		return "?"
	}
	d := time.Since(t)
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	return fmt.Sprintf("%dh", int(d.Hours()))
}
