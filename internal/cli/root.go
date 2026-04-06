package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:     "ht",
	Short:   "Hitch — a hooks framework for AI coding agents",
	Long:    "Hitch lets you declare behaviors (notifications, safety guards, quality gates) in a DSL,\nand generates Claude Code hook configurations and scripts.",
	Version: Version,
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
}

func init() {
	rootCmd.SetVersionTemplate("ht version {{.Version}}\n")

	rootCmd.AddCommand(
		newInitCmd(),
		newChannelCmd(),
		newRuleCmd(),
		newHookCmd(),
		newSyncCmd(),
		newStatusCmd(),
		newLogCmd(),
		newMuteCmd(),
		newUnmuteCmd(),
		newPackageCmd(),
		newNotifyCmd(),
		newExportCmd(),
		newImportCmd(),
		newDenyListCmd(),
		newProxyCmd(),
		newSettingsCmd(),
		newSessionsCmd(),
		newAutopsyCmd(),
		newProfileCmd(),
		newLaunchCmd(),
		newDaemonCmd(),
		newAgentsCmd(),
		newWatchCmd(),
		newDashboardCmd(),
	)
}

// Execute runs the root command.
func Execute() error {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return err
	}
	return nil
}
