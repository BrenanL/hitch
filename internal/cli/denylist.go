package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BrenanL/hitch/internal/engine"
	"github.com/spf13/cobra"
)

func newDenyListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "deny-list",
		Short: "Manage deny lists",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	cmd.AddCommand(
		newDenyListListCmd(),
		newDenyListShowCmd(),
		newDenyListAddCmd(),
	)
	return cmd
}

func newDenyListListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List available deny lists",
		RunE: func(cmd *cobra.Command, args []string) error {
			lists := engine.LoadDenyLists()
			if len(lists) == 0 {
				fmt.Println("No deny lists available.")
				return nil
			}
			for name, patterns := range lists {
				fmt.Printf("%-20s %d patterns\n", name, len(patterns))
			}
			return nil
		},
	}
}

func newDenyListShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show <name>",
		Short: "Show deny list contents",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			patterns := engine.GetDenyList(args[0])
			if len(patterns) == 0 {
				return fmt.Errorf("deny list %q not found or empty", args[0])
			}
			fmt.Printf("Deny list: %s (%d patterns)\n", args[0], len(patterns))
			for _, p := range patterns {
				fmt.Printf("  %s\n", p)
			}
			return nil
		},
	}
}

func newDenyListAddCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "add <name> <pattern>",
		Short: "Add a pattern to a custom deny list",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			pattern := args[1]

			home, err := os.UserHomeDir()
			if err != nil {
				return err
			}

			denyDir := filepath.Join(home, ".hitch", "deny-lists")
			if err := os.MkdirAll(denyDir, 0o755); err != nil {
				return err
			}

			filePath := filepath.Join(denyDir, name+".txt")

			// Append pattern
			f, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
			if err != nil {
				return err
			}
			defer f.Close()

			if !strings.HasSuffix(pattern, "\n") {
				pattern += "\n"
			}
			if _, err := f.WriteString(pattern); err != nil {
				return err
			}

			fmt.Printf("Added pattern %q to deny list %q\n", strings.TrimSpace(pattern), name)
			return nil
		},
	}
}
