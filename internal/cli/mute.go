package cli

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

func newMuteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "mute [duration]",
		Short: "Mute all notifications",
		Long:  "Mute all notifications. Optionally specify a duration (e.g., 30m, 2h). Without duration, mutes for 24h.",
		RunE:  runMute,
	}
}

func runMute(cmd *cobra.Command, args []string) error {
	db, _, err := openDB()
	if err != nil {
		return err
	}
	defer db.Close()

	duration := 24 * time.Hour
	if len(args) > 0 {
		d, err := parseSinceDuration(args[0])
		if err != nil {
			return fmt.Errorf("invalid duration: %w", err)
		}
		duration = d
	}

	until := time.Now().UTC().Add(duration).Format(time.RFC3339)
	if err := db.MuteSet(until); err != nil {
		return err
	}

	fmt.Printf("Notifications muted until %s\n", until)
	return nil
}

func newUnmuteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "unmute",
		Short: "Unmute notifications",
		RunE:  runUnmute,
	}
}

func runUnmute(cmd *cobra.Command, args []string) error {
	db, _, err := openDB()
	if err != nil {
		return err
	}
	defer db.Close()

	if err := db.MuteClear(); err != nil {
		return err
	}

	fmt.Println("Notifications unmuted.")
	return nil
}
