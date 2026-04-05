package cli

import (
	"fmt"
	"time"

	"github.com/BrenanL/hitch/internal/daemon"
	"github.com/spf13/cobra"
)

func newDaemonCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "daemon",
		Short: "Background monitoring daemon",
		Long: `Persistent background daemon that aggregates session data from SQLite,
proxy logs, and JSONL transcripts. Exposes an HTTP API on port 9801.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	cmd.AddCommand(
		newDaemonStartCmd(),
		newDaemonStopCmd(),
		newDaemonStatusCmd(),
	)
	return cmd
}

func newDaemonStartCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start the monitoring daemon",
		RunE:  runDaemonStart,
	}
	cmd.Flags().Int("port", 9801, "Port to listen on")
	cmd.Flags().Bool("foreground", false, "Run in foreground (for systemd)")
	return cmd
}

func runDaemonStart(cmd *cobra.Command, args []string) error {
	port, _ := cmd.Flags().GetInt("port")
	foreground, _ := cmd.Flags().GetBool("foreground")

	db, _, err := openDB()
	if err != nil {
		return err
	}
	// Only close DB if not running in foreground (foreground blocks until shutdown)
	if !foreground {
		defer db.Close()
	}

	d := daemon.New(port, db)
	return d.Start(foreground)
}

func newDaemonStopCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stop",
		Short: "Stop the running daemon",
		RunE:  runDaemonStop,
	}
}

func runDaemonStop(cmd *cobra.Command, args []string) error {
	pidPath := daemon.DefaultPIDPath()
	if err := daemon.Stop(pidPath); err != nil {
		return err
	}
	fmt.Println("Hitch daemon stopped")
	return nil
}

func newDaemonStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show daemon status",
		RunE:  runDaemonStatus,
	}
}

func runDaemonStatus(cmd *cobra.Command, args []string) error {
	pidPath := daemon.DefaultPIDPath()
	status, err := daemon.Status(pidPath, 9801)
	if err != nil {
		return err
	}

	if !status.Running {
		if status.StalePID > 0 {
			fmt.Printf("Hitch daemon: not running (stale PID %d)\n", status.StalePID)
		} else {
			fmt.Println("Hitch daemon: not running")
		}
		return nil
	}

	if status.Health == nil {
		fmt.Printf("Hitch daemon: running (PID %d) but health check failed", status.PID)
		if status.HealthError != "" {
			fmt.Printf(": %s", status.HealthError)
		}
		fmt.Println()
		return nil
	}

	h := status.Health
	uptime := time.Duration(h.UptimeSeconds) * time.Second
	fmt.Printf("Hitch daemon: running (PID %d, uptime %s)\n", status.PID, formatDaemonDuration(uptime))
	fmt.Printf("  API port:          %d\n", h.Port)
	fmt.Printf("  Tracked sessions:  %d active, %d total\n", h.ActiveSessions, h.TrackedSessions)
	fmt.Printf("  Poll count:        %d\n", h.PollCount)
	return nil
}

func formatDaemonDuration(d time.Duration) string {
	d = d.Round(time.Second)
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60
	if h > 0 {
		return fmt.Sprintf("%dh%02dm%02ds", h, m, s)
	}
	if m > 0 {
		return fmt.Sprintf("%dm%02ds", m, s)
	}
	return fmt.Sprintf("%ds", s)
}
