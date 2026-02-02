package cli

import (
	"fmt"

	"github.com/BrenanL/hitch/internal/generator"
	"github.com/BrenanL/hitch/internal/state"
	"github.com/spf13/cobra"
)

func newSyncCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Regenerate settings.json from rules",
		RunE:  runSync,
	}
	cmd.Flags().Bool("dry-run", false, "Show what would change without writing")
	cmd.Flags().String("scope", "all", "Scope to sync (global|project|all)")
	return cmd
}

func runSync(cmd *cobra.Command, args []string) error {
	dryRun, _ := cmd.Flags().GetBool("dry-run")

	db, paths, err := openDB()
	if err != nil {
		return err
	}
	defer db.Close()

	if dryRun {
		fmt.Println("Dry run — no changes will be written.")
	}

	return runSyncInternal(db, paths, dryRun)
}

// syncScope syncs rules for a specific scope to its settings.json.
func syncScope(db *state.DB, allRules []state.Rule, scope, settingsPath, manifestPath, htBinary string, dryRun bool) error {
	// Filter rules for this scope
	var scopeRules []state.Rule
	for _, r := range allRules {
		if r.Scope == scope && r.Enabled {
			scopeRules = append(scopeRules, r)
		}
	}

	// Generate entries
	var entries []*generator.HookEntryInfo

	// System hooks
	entries = append(entries, generator.SystemHooks(htBinary)...)

	// Rule entries
	for _, r := range scopeRules {
		entry, err := generator.RuleToHookEntry(r, htBinary)
		if err != nil {
			fmt.Printf("Warning: skipping rule %s: %v\n", r.ID, err)
			continue
		}
		entries = append(entries, entry)
	}

	// Read existing
	settings, err := generator.ReadSettings(settingsPath)
	if err != nil {
		return err
	}
	manifest, err := generator.ReadManifest(manifestPath)
	if err != nil {
		return err
	}

	// Merge
	generator.MergeHooks(settings, manifest, entries)
	generator.UpdateManifest(manifest, entries, scope, settingsPath)

	if dryRun {
		fmt.Printf("Would write %d hook entries to %s\n", len(entries), settingsPath)
		for _, e := range entries {
			fmt.Printf("  [%s] %s → %s\n", e.Event, e.Matcher, e.Entry.Command[:min(60, len(e.Entry.Command))])
		}
		return nil
	}

	// Write
	if err := generator.WriteSettings(settingsPath, settings); err != nil {
		return err
	}
	if err := generator.WriteManifest(manifestPath, manifest); err != nil {
		return err
	}

	fmt.Printf("Synced %d entries to %s\n", len(entries), settingsPath)
	return nil
}
