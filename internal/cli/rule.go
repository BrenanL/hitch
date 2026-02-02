package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"

	"github.com/BrenanL/hitch/internal/dsl"
	"github.com/BrenanL/hitch/internal/state"
	"github.com/spf13/cobra"
)

func newRuleCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rule",
		Short: "Manage DSL rules",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	cmd.AddCommand(
		newRuleAddCmd(),
		newRuleListCmd(),
		newRuleRemoveCmd(),
		newRuleEnableCmd(),
		newRuleDisableCmd(),
	)
	return cmd
}

func newRuleAddCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add '<dsl>'",
		Short: "Add a rule from a DSL string",
		Args:  cobra.MaximumNArgs(1),
		RunE:  runRuleAdd,
	}
	cmd.Flags().StringP("file", "f", "", "Add rules from a .hitch file")
	cmd.Flags().Bool("global", false, "Add as global rule")
	return cmd
}

func runRuleAdd(cmd *cobra.Command, args []string) error {
	filePath, _ := cmd.Flags().GetString("file")
	global, _ := cmd.Flags().GetBool("global")

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

	var dslStrings []string

	if filePath != "" {
		data, err := os.ReadFile(filePath)
		if err != nil {
			return fmt.Errorf("reading file: %w", err)
		}
		rules, err := dsl.Parse(string(data))
		if err != nil {
			return fmt.Errorf("parsing file: %w", err)
		}
		for _, r := range rules {
			dslStrings = append(dslStrings, r.Raw)
		}
		// If multiple rules from a file, add each one individually
		if len(rules) > 1 {
			for _, r := range rules {
				// Reconstruct individual rule DSL
				_ = r // We'll use the raw input approach below
			}
		}
		// Actually, just add each rule line from the parsed output
		dslStrings = nil
		for _, r := range rules {
			// Re-extract individual rule text
			dslStrings = append(dslStrings, reconstructDSL(&r))
		}
	} else if len(args) > 0 {
		dslStrings = []string{args[0]}
	} else {
		return fmt.Errorf("provide a DSL string or use --file")
	}

	for _, dslStr := range dslStrings {
		// Validate
		if _, err := dsl.ParseRule(dslStr); err != nil {
			return fmt.Errorf("invalid rule: %w", err)
		}

		// Generate ID
		id := ruleID(dslStr)

		rule := state.Rule{
			ID:      id,
			DSL:     dslStr,
			Scope:   scope,
			Enabled: true,
		}

		if err := db.RuleAdd(rule); err != nil {
			return fmt.Errorf("adding rule: %w", err)
		}

		fmt.Printf("Rule %s added: %s\n", id, dslStr)
	}

	// Auto-sync
	return runSyncInternal(db, paths, false)
}

func newRuleListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List rules",
		RunE:  runRuleList,
	}
	cmd.Flags().String("scope", "", "Filter by scope (global|project)")
	return cmd
}

func runRuleList(cmd *cobra.Command, args []string) error {
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
		fmt.Println("No rules configured. Add one with: ht rule add '<dsl>'")
		return nil
	}

	for _, r := range rules {
		enabled := "+"
		if !r.Enabled {
			enabled = "-"
		}
		fmt.Printf("[%s] %s %s  (%s)\n", enabled, r.ID, r.DSL, r.Scope)
	}
	return nil
}

func newRuleRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "remove <id>",
		Short: "Remove a rule",
		Args:  cobra.ExactArgs(1),
		RunE:  runRuleRemove,
	}
}

func runRuleRemove(cmd *cobra.Command, args []string) error {
	db, paths, err := openDB()
	if err != nil {
		return err
	}
	defer db.Close()

	if err := db.RuleRemove(args[0]); err != nil {
		return err
	}
	fmt.Printf("Rule %s removed.\n", args[0])

	return runSyncInternal(db, paths, false)
}

func newRuleEnableCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "enable <id>",
		Short: "Enable a disabled rule",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			db, paths, err := openDB()
			if err != nil {
				return err
			}
			defer db.Close()
			if err := db.RuleEnable(args[0]); err != nil {
				return err
			}
			fmt.Printf("Rule %s enabled.\n", args[0])
			return runSyncInternal(db, paths, false)
		},
	}
}

func newRuleDisableCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "disable <id>",
		Short: "Disable a rule without removing it",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			db, paths, err := openDB()
			if err != nil {
				return err
			}
			defer db.Close()
			if err := db.RuleDisable(args[0]); err != nil {
				return err
			}
			fmt.Printf("Rule %s disabled.\n", args[0])
			return runSyncInternal(db, paths, false)
		},
	}
}

// ruleID generates a short hash ID for a rule.
func ruleID(dslStr string) string {
	h := sha256.Sum256([]byte(dslStr))
	return hex.EncodeToString(h[:])[:6]
}

// reconstructDSL reconstructs a DSL string from a parsed rule.
// For file imports where the raw text might contain multiple rules.
func reconstructDSL(r *dsl.Rule) string {
	// Simple reconstruction — use the event name and re-serialize
	result := "on " + r.Event.Name
	if r.Event.Matcher != "" && r.Event.Matcher != r.Event.DefaultMatcher {
		result += ":" + r.Event.Matcher
	}
	result += " ->"
	for i, action := range r.Actions {
		if i > 0 {
			result += " ->"
		}
		switch a := action.(type) {
		case dsl.NotifyAction:
			result += " notify " + a.Channel
			if a.Message != "" {
				result += fmt.Sprintf(" %q", a.Message)
			}
		case dsl.RunAction:
			result += fmt.Sprintf(` run %q`, a.Command)
			if a.Async {
				result += " async"
			}
		case dsl.DenyAction:
			result += " deny"
			if a.Reason != "" {
				result += fmt.Sprintf(" %q", a.Reason)
			}
		case dsl.RequireAction:
			result += " require " + a.Check
		case dsl.SummarizeAction:
			result += " summarize"
		case dsl.LogAction:
			result += " log"
			if a.Target != "" {
				result += " " + a.Target
			}
		}
	}
	// Condition reconstruction would be complex; for now use original raw
	return result
}

// runSyncInternal performs sync without opening a new DB connection.
func runSyncInternal(db *state.DB, paths *Paths, dryRun bool) error {
	htBinary, err := os.Executable()
	if err != nil {
		htBinary = "ht"
	}

	// Get all enabled rules
	rules, err := db.RuleList()
	if err != nil {
		return fmt.Errorf("listing rules: %w", err)
	}

	// Only sync global scope if there are global-scoped rules or a global
	// manifest already exists (meaning the user previously ran init --global).
	// Never write to ~/.claude/settings.json as a side effect of project operations.
	hasGlobalRules := false
	for _, r := range rules {
		if r.Scope == "global" {
			hasGlobalRules = true
			break
		}
	}
	if hasGlobalRules {
		if _, err := os.Stat(paths.GlobalManifest); err == nil {
			if err := syncScope(db, rules, "global", paths.GlobalSettings, paths.GlobalManifest, htBinary, dryRun); err != nil {
				return fmt.Errorf("syncing global: %w", err)
			}
		}
	}

	// Sync project scope if in a project
	cwd, _ := os.Getwd()
	projectScope := "project:" + cwd
	projectDir := filepath.Join(cwd, ".hitch")
	if _, err := os.Stat(projectDir); err == nil {
		if err := syncScope(db, rules, projectScope, paths.ProjectSettings, paths.ProjectManifest, htBinary, dryRun); err != nil {
			return fmt.Errorf("syncing project: %w", err)
		}
	}

	return nil
}
