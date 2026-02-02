package cli

import (
	"fmt"
	"os"

	"github.com/BrenanL/hitch/internal/dsl"
	"github.com/BrenanL/hitch/internal/state"
	"github.com/spf13/cobra"
)

func newImportCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "import <file>",
		Short: "Import rules from a .hitch file",
		Args:  cobra.ExactArgs(1),
		RunE:  runImport,
	}
	cmd.Flags().Bool("global", false, "Import as global rules")
	return cmd
}

func runImport(cmd *cobra.Command, args []string) error {
	filePath := args[0]
	global, _ := cmd.Flags().GetBool("global")

	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("reading file: %w", err)
	}

	rules, err := dsl.Parse(string(data))
	if err != nil {
		return fmt.Errorf("parsing file: %w", err)
	}

	if len(rules) == 0 {
		fmt.Println("No rules found in file.")
		return nil
	}

	db, paths, err := openDB()
	if err != nil {
		return err
	}
	defer db.Close()

	scope := "global"
	if !global {
		cwd, _ := os.Getwd()
		scope = "project:" + cwd
	}

	for _, r := range rules {
		dslStr := reconstructDSL(&r)
		id := ruleID(dslStr)

		rule := state.Rule{
			ID:      id,
			DSL:     dslStr,
			Scope:   scope,
			Enabled: true,
		}

		if err := db.RuleAdd(rule); err != nil {
			fmt.Printf("Warning: skipping rule: %v\n", err)
			continue
		}
		fmt.Printf("Imported rule %s: %s\n", id, dslStr)
	}

	return runSyncInternal(db, paths, false)
}
