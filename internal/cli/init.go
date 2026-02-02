package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/BrenanL/hitch/internal/generator"
	"github.com/BrenanL/hitch/internal/state"
	"github.com/spf13/cobra"
)

func newInitCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize hitch in the current project or globally",
		RunE:  runInit,
	}
	cmd.Flags().Bool("global", false, "Initialize global hitch configuration")
	return cmd
}

func runInit(cmd *cobra.Command, args []string) error {
	global, _ := cmd.Flags().GetBool("global")

	paths, err := resolvePaths()
	if err != nil {
		return err
	}

	// Create directories
	if global {
		if err := os.MkdirAll(paths.GlobalDir, 0o755); err != nil {
			return fmt.Errorf("creating %s: %w", paths.GlobalDir, err)
		}
		fmt.Printf("Created %s\n", paths.GlobalDir)
	} else {
		if err := os.MkdirAll(paths.ProjectDir, 0o755); err != nil {
			return fmt.Errorf("creating %s: %w", paths.ProjectDir, err)
		}
		fmt.Printf("Created %s\n", paths.ProjectDir)
	}

	// Initialize database
	db, err := state.Open(paths.GlobalDB)
	if err != nil {
		return fmt.Errorf("initializing database: %w", err)
	}
	defer db.Close()
	fmt.Printf("Database at %s\n", paths.GlobalDB)

	// Find ht binary
	htBinary, err := os.Executable()
	if err != nil {
		htBinary = "ht"
	}

	// Install system hooks
	settingsPath := paths.GlobalSettings
	manifestPath := paths.GlobalManifest
	scope := "global"
	if !global {
		settingsPath = paths.ProjectSettings
		manifestPath = paths.ProjectManifest
		scope = "project:" + filepath.Dir(paths.ProjectDir)
	}

	entries := generator.SystemHooks(htBinary)

	// Read existing settings
	settings, err := generator.ReadSettings(settingsPath)
	if err != nil {
		return fmt.Errorf("reading settings: %w", err)
	}

	// Read manifest
	manifest, err := generator.ReadManifest(manifestPath)
	if err != nil {
		return fmt.Errorf("reading manifest: %w", err)
	}

	// Merge system hooks
	generator.MergeHooks(settings, manifest, entries)
	generator.UpdateManifest(manifest, entries, scope, settingsPath)

	// Write back
	if err := generator.WriteSettings(settingsPath, settings); err != nil {
		return fmt.Errorf("writing settings: %w", err)
	}
	if err := generator.WriteManifest(manifestPath, manifest); err != nil {
		return fmt.Errorf("writing manifest: %w", err)
	}

	fmt.Printf("System hooks installed in %s\n", settingsPath)
	fmt.Println("Hitch initialized successfully.")
	return nil
}

func ensureSettingsDir(settingsPath string) error {
	dir := filepath.Dir(settingsPath)
	return os.MkdirAll(dir, 0o755)
}

// writeJSON is a helper to write pretty-printed JSON.
func writeJSON(path string, v any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
