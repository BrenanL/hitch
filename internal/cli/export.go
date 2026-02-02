package cli

import (
	"fmt"

	"github.com/BrenanL/hitch/internal/state"
	"github.com/spf13/cobra"
)

func newExportCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export rules as .hitch DSL",
		RunE:  runExport,
	}
	cmd.Flags().String("scope", "", "Scope to export (global|project)")
	return cmd
}

func runExport(cmd *cobra.Command, args []string) error {
	db, _, err := openDB()
	if err != nil {
		return err
	}
	defer db.Close()

	scopeFilter, _ := cmd.Flags().GetString("scope")

	var rules []state.Rule
	if scopeFilter != "" {
		rules, err = db.RuleListByScope(scopeFilter)
	} else {
		rules, err = db.RuleList()
	}
	if err != nil {
		return err
	}

	if len(rules) == 0 {
		return nil
	}

	fmt.Println("# Hitch rules")
	for _, r := range rules {
		fmt.Printf("# scope: %s, id: %s, enabled: %v\n", r.Scope, r.ID, r.Enabled)
		fmt.Println(r.DSL)
		fmt.Println()
	}
	return nil
}
