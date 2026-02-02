package cli

import (
	"fmt"
	"time"

	"github.com/BrenanL/hitch/internal/state"
	"github.com/spf13/cobra"
)

func newLogCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "log",
		Short: "View hook event log",
		RunE:  runLog,
	}
	cmd.Flags().String("session", "", "Filter by session ID")
	cmd.Flags().String("event", "", "Filter by event type")
	cmd.Flags().String("since", "", "Filter by time (e.g., 1h, 30m)")
	cmd.Flags().IntP("limit", "n", 50, "Maximum number of events to show")
	return cmd
}

func runLog(cmd *cobra.Command, args []string) error {
	db, _, err := openDB()
	if err != nil {
		return err
	}
	defer db.Close()

	session, _ := cmd.Flags().GetString("session")
	event, _ := cmd.Flags().GetString("event")
	since, _ := cmd.Flags().GetString("since")
	limit, _ := cmd.Flags().GetInt("limit")

	filter := state.EventFilter{
		SessionID: session,
		HookEvent: event,
		Limit:     limit,
	}

	if since != "" {
		dur, err := parseSinceDuration(since)
		if err != nil {
			return fmt.Errorf("invalid --since value: %w", err)
		}
		filter.Since = time.Now().Add(-dur).UTC().Format(time.RFC3339)
	}

	events, err := db.EventQuery(filter)
	if err != nil {
		return err
	}

	if len(events) == 0 {
		fmt.Println("No events found.")
		return nil
	}

	fmt.Printf("%-20s %-16s %-8s %-10s %s\n", "TIMESTAMP", "EVENT", "RULE", "MS", "ACTION")
	for _, e := range events {
		ts := e.Timestamp
		if len(ts) > 19 {
			ts = ts[:19]
		}
		fmt.Printf("%-20s %-16s %-8s %-10d %s\n", ts, e.HookEvent, e.RuleID, e.DurationMs, e.ActionTaken)
	}
	return nil
}

func parseSinceDuration(s string) (time.Duration, error) {
	if len(s) < 2 {
		return 0, fmt.Errorf("invalid duration %q", s)
	}

	unit := s[len(s)-1]
	numStr := s[:len(s)-1]

	var dur time.Duration
	var n int
	_, err := fmt.Sscanf(numStr, "%d", &n)
	if err != nil {
		return 0, fmt.Errorf("invalid number in %q", s)
	}

	switch unit {
	case 's':
		dur = time.Duration(n) * time.Second
	case 'm':
		dur = time.Duration(n) * time.Minute
	case 'h':
		dur = time.Duration(n) * time.Hour
	case 'd':
		dur = time.Duration(n) * 24 * time.Hour
	default:
		return 0, fmt.Errorf("unknown unit %q in %q (use s, m, h, or d)", string(unit), s)
	}

	return dur, nil
}
