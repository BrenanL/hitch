package cli

import (
	"fmt"

	"github.com/BrenanL/hitch/internal/dsl"
	"github.com/BrenanL/hitch/internal/packages"
	"github.com/BrenanL/hitch/internal/state"
	"github.com/spf13/cobra"
)

func newPackageCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "package",
		Short: "Manage hook packages",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	cmd.AddCommand(
		newPackageListCmd(),
		newPackageEnableCmd(),
		newPackageDisableCmd(),
		newPackageShowCmd(),
	)
	return cmd
}

func newPackageListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List available hook packages",
		RunE: func(cmd *cobra.Command, args []string) error {
			pkgs := packages.List()
			if len(pkgs) == 0 {
				fmt.Println("No packages available.")
				return nil
			}
			for _, pkg := range pkgs {
				fmt.Printf("%-12s %s (%d rules)\n", pkg.Name, pkg.Description, len(pkg.Rules))
			}
			return nil
		},
	}
}

func newPackageEnableCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "enable <name>",
		Short: "Enable a hook package",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			pkg := packages.Get(args[0])
			if pkg == nil {
				return fmt.Errorf("package %q not found", args[0])
			}

			db, paths, err := openDB()
			if err != nil {
				return err
			}
			defer db.Close()

			for _, dslStr := range pkg.Rules {
				// Validate
				if _, err := dsl.ParseRule(dslStr); err != nil {
					fmt.Printf("Warning: skipping invalid rule: %v\n", err)
					continue
				}

				id := "pkg-" + pkg.Name + "-" + ruleID(dslStr)
				rule := state.Rule{
					ID:      id,
					DSL:     dslStr,
					Scope:   "global",
					Enabled: true,
				}
				if err := db.RuleAdd(rule); err != nil {
					// Rule may already exist
					continue
				}
			}

			fmt.Printf("Package %q enabled (%d rules).\n", pkg.Name, len(pkg.Rules))
			return runSyncInternal(db, paths, false)
		},
	}
}

func newPackageDisableCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "disable <name>",
		Short: "Disable a hook package",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			pkg := packages.Get(args[0])
			if pkg == nil {
				return fmt.Errorf("package %q not found", args[0])
			}

			db, paths, err := openDB()
			if err != nil {
				return err
			}
			defer db.Close()

			prefix := "pkg-" + pkg.Name + "-"
			rules, err := db.RuleList()
			if err != nil {
				return err
			}

			removed := 0
			for _, r := range rules {
				if len(r.ID) >= len(prefix) && r.ID[:len(prefix)] == prefix {
					if err := db.RuleRemove(r.ID); err == nil {
						removed++
					}
				}
			}

			fmt.Printf("Package %q disabled (%d rules removed).\n", pkg.Name, removed)
			return runSyncInternal(db, paths, false)
		},
	}
}

func newPackageShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show <name>",
		Short: "Show package details and rules",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			pkg := packages.Get(args[0])
			if pkg == nil {
				return fmt.Errorf("package %q not found", args[0])
			}

			fmt.Printf("Package: %s\n", pkg.Name)
			fmt.Printf("Description: %s\n", pkg.Description)
			fmt.Printf("Rules:\n")
			for _, r := range pkg.Rules {
				fmt.Printf("  %s\n", r)
			}
			return nil
		},
	}
}
