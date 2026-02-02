package cli

import (
	"fmt"

	"github.com/BrenanL/hitch/internal/state"
	"github.com/spf13/cobra"
)

func newStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show hitch status overview",
		RunE:  runStatus,
	}
}

func runStatus(cmd *cobra.Command, args []string) error {
	db, paths, err := openDB()
	if err != nil {
		return err
	}
	defer db.Close()

	// Channels
	channels, err := db.ChannelList()
	if err != nil {
		return err
	}

	fmt.Printf("Channels: %d configured\n", len(channels))
	for _, ch := range channels {
		fmt.Printf("  %s (%s)\n", ch.ID, ch.Adapter)
	}

	// Rules
	rules, err := db.RuleList()
	if err != nil {
		return err
	}
	enabledCount := 0
	for _, r := range rules {
		if r.Enabled {
			enabledCount++
		}
	}
	fmt.Printf("\nRules: %d total, %d enabled\n", len(rules), enabledCount)
	for _, r := range rules {
		status := "+"
		if !r.Enabled {
			status = "-"
		}
		fmt.Printf("  [%s] %s: %s\n", status, r.ID, r.DSL)
	}

	// Mute
	muted, err := db.IsMuted()
	if err == nil {
		if muted {
			until, _ := db.MuteGet()
			fmt.Printf("\nNotifications: MUTED until %s\n", until)
		} else {
			fmt.Println("\nNotifications: active")
		}
	}

	// Recent events
	events, err := db.EventQuery(state.EventFilter{Limit: 5})
	if err == nil && len(events) > 0 {
		fmt.Println("\nRecent events:")
		for _, e := range events {
			fmt.Printf("  %s %s %s %s\n", e.Timestamp[:19], e.HookEvent, e.RuleID, e.ActionTaken)
		}
	}

	fmt.Printf("\nDatabase: %s\n", paths.GlobalDB)
	return nil
}
