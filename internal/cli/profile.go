package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BrenanL/hitch/pkg/profiles"
	"github.com/spf13/cobra"
)

func newProfileCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "profile",
		Short: "Manage named settings profiles",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	cmd.AddCommand(
		newProfileListCmd(),
		newProfileShowCmd(),
		newProfileSwitchCmd(),
		newProfileCurrentCmd(),
		newProfileCreateCmd(),
		newProfileDeleteCmd(),
	)
	return cmd
}

// profileProjectDir returns the project directory for profile operations.
// When useProject is true, returns the current working directory.
// When false, returns the user home directory (for global profile).
func profileProjectDir(useProject bool) (string, error) {
	if useProject {
		return os.Getwd()
	}
	return os.UserHomeDir()
}

// isBuiltinProfile returns true if the profile has a "builtin" tag.
func isBuiltinProfile(p profiles.Profile) bool {
	for _, tag := range p.Tags {
		if tag == "builtin" {
			return true
		}
	}
	return false
}

// profileSource returns "built-in" or "user" string for a profile.
func profileSource(p profiles.Profile) string {
	if isBuiltinProfile(p) {
		return "built-in"
	}
	return "user"
}

// --- list ---

func newProfileListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all available profiles",
		RunE:  runProfileList,
	}
	cmd.Flags().Bool("project", false, "Show project-active profile instead of global")
	cmd.Flags().Bool("json", false, "Output as JSON array")
	return cmd
}

func runProfileList(cmd *cobra.Command, args []string) error {
	useProject, _ := cmd.Flags().GetBool("project")
	outputJSON, _ := cmd.Flags().GetBool("json")

	all, err := profiles.LoadAll()
	if err != nil {
		return fmt.Errorf("loading profiles: %w", err)
	}

	projectDir, err := profileProjectDir(useProject)
	if err != nil {
		return fmt.Errorf("resolving project dir: %w", err)
	}

	active, err := profiles.CurrentProfile(projectDir)
	if err != nil {
		// Non-fatal: no active profile is fine
		active = ""
	}

	if outputJSON {
		type jsonProfile struct {
			profiles.Profile
			Active bool   `json:"active"`
			Source string `json:"source"`
		}
		out := make([]jsonProfile, 0, len(all))
		for _, p := range all {
			out = append(out, jsonProfile{
				Profile: p,
				Active:  p.Name == active,
				Source:  profileSource(p),
			})
		}
		data, err := json.MarshalIndent(out, "", "  ")
		if err != nil {
			return fmt.Errorf("marshaling JSON: %w", err)
		}
		fmt.Println(string(data))
		return nil
	}

	for _, p := range all {
		marker := " "
		if p.Name == active {
			marker = "*"
		}
		source := ""
		if !isBuiltinProfile(p) {
			source = " [user-defined]"
		}
		fmt.Printf("%s %-14s %s%s\n", marker, p.Name, p.Description, source)
	}
	return nil
}

// --- show ---

func newProfileShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show <name>",
		Short: "Show profile details",
		Args:  cobra.ExactArgs(1),
		RunE:  runProfileShow,
	}
}

func runProfileShow(cmd *cobra.Command, args []string) error {
	name := args[0]

	p, err := profiles.Load(name)
	if err != nil {
		return fmt.Errorf("profile %q not found: %w", name, err)
	}

	// Determine source annotation.
	source := "builtin (embedded)"
	if !isBuiltinProfile(*p) {
		home, _ := os.UserHomeDir()
		userPath := filepath.Join(home, ".hitch", "profiles", name+".json")
		source = userPath
	}

	fmt.Printf("# Source: %s\n", source)

	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling profile: %w", err)
	}
	fmt.Println(string(data))
	return nil
}

// --- switch ---

func newProfileSwitchCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "switch <name>",
		Short: "Apply a profile to settings.local.json",
		Args:  cobra.MaximumNArgs(1),
		RunE:  runProfileSwitch,
	}
	cmd.Flags().Bool("project", false, "Write to project-scoped settings.local.json")
	cmd.Flags().Bool("reset", false, "Remove all profile-applied keys and clear active profile")
	return cmd
}

func runProfileSwitch(cmd *cobra.Command, args []string) error {
	useProject, _ := cmd.Flags().GetBool("project")
	reset, _ := cmd.Flags().GetBool("reset")

	projectDir, err := profileProjectDir(useProject)
	if err != nil {
		return fmt.Errorf("resolving project dir: %w", err)
	}

	if reset {
		// Read active profile record to get tracked keys.
		rec, err := readActiveProfileRecord(projectDir)
		if err != nil {
			return fmt.Errorf("reading active profile: %w", err)
		}
		if rec == nil {
			fmt.Println("No active profile to reset.")
			return nil
		}

		if err := profiles.ResetProfile(rec.TrackedKeys, projectDir); err != nil {
			return fmt.Errorf("resetting profile: %w", err)
		}

		scope := "~/.claude/settings.local.json"
		if useProject {
			scope = ".claude/settings.local.json"
		}
		fmt.Printf("Reset profile %q from %s\n", rec.Name, scope)
		return nil
	}

	if len(args) == 0 {
		return fmt.Errorf("profile name required (or use --reset)")
	}
	name := args[0]

	p, err := profiles.Load(name)
	if err != nil {
		return fmt.Errorf("profile %q not found: %w", name, err)
	}

	// Reset previous profile if any.
	rec, err := readActiveProfileRecord(projectDir)
	if err == nil && rec != nil {
		if resetErr := profiles.ResetProfile(rec.TrackedKeys, projectDir); resetErr != nil {
			return fmt.Errorf("resetting previous profile: %w", resetErr)
		}
	}

	written, err := profiles.ApplyProfile(p, projectDir)
	if err != nil {
		return fmt.Errorf("applying profile: %w", err)
	}

	scope := "~/.claude/settings.local.json"
	if useProject {
		cwd, _ := os.Getwd()
		scope = filepath.Join(cwd, ".claude", "settings.local.json")
	}
	fmt.Printf("Applied profile %q to %s\n", name, scope)
	fmt.Printf("Keys written: %d\n", len(written))
	return nil
}

// readActiveProfileRecord reads the active-profile.json for the given project dir.
// Returns nil, nil if no active profile is recorded.
func readActiveProfileRecord(projectDir string) (*activeProfileRecord, error) {
	path := filepath.Join(projectDir, ".hitch", "active-profile.json")
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading active-profile.json: %w", err)
	}
	var rec activeProfileRecord
	if err := json.Unmarshal(data, &rec); err != nil {
		return nil, fmt.Errorf("parsing active-profile.json: %w", err)
	}
	return &rec, nil
}

// activeProfileRecord mirrors the unexported type in pkg/profiles/apply.go.
type activeProfileRecord struct {
	Name        string   `json:"name"`
	TrackedKeys []string `json:"tracked_keys"`
}

// --- current ---

func newProfileCurrentCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "current",
		Short: "Show the currently active profile",
		RunE:  runProfileCurrent,
	}
	cmd.Flags().Bool("project", false, "Show project-active profile")
	return cmd
}

func runProfileCurrent(cmd *cobra.Command, args []string) error {
	useProject, _ := cmd.Flags().GetBool("project")

	projectDir, err := profileProjectDir(useProject)
	if err != nil {
		return fmt.Errorf("resolving project dir: %w", err)
	}

	name, err := profiles.CurrentProfile(projectDir)
	if err != nil {
		return fmt.Errorf("checking active profile: %w", err)
	}

	if name == "" {
		if useProject {
			// Also report global.
			home, _ := os.UserHomeDir()
			globalName, _ := profiles.CurrentProfile(home)
			if globalName != "" {
				fmt.Printf("No project profile active. Global: %s\n", globalName)
			} else {
				fmt.Println("none")
			}
		} else {
			fmt.Println("none")
		}
		return nil
	}

	fmt.Println(name)
	return nil
}

// --- create ---

func newProfileCreateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create <name>",
		Short: "Create a new user profile",
		Args:  cobra.ExactArgs(1),
		RunE:  runProfileCreate,
	}
	cmd.Flags().String("description", "", "Profile description")
	cmd.Flags().StringArray("env", nil, "Env var to set (KEY=VALUE)")
	return cmd
}

func runProfileCreate(cmd *cobra.Command, args []string) error {
	name := args[0]
	description, _ := cmd.Flags().GetString("description")
	envPairs, _ := cmd.Flags().GetStringArray("env")

	if description == "" {
		description = fmt.Sprintf("User profile: %s", name)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("resolving home dir: %w", err)
	}

	profilesDir := filepath.Join(home, ".hitch", "profiles")
	if err := os.MkdirAll(profilesDir, 0o755); err != nil {
		return fmt.Errorf("creating profiles dir: %w", err)
	}

	destPath := filepath.Join(profilesDir, name+".json")
	if _, err := os.Stat(destPath); err == nil {
		return fmt.Errorf("profile %q already exists at %s", name, destPath)
	}

	envMap := make(map[string]string)
	for _, pair := range envPairs {
		k, v, ok := strings.Cut(pair, "=")
		if !ok {
			return fmt.Errorf("invalid --env value %q: expected KEY=VALUE", pair)
		}
		envMap[k] = v
	}

	p := profiles.Profile{
		Name:        name,
		Description: description,
	}
	if len(envMap) > 0 {
		p.Env = envMap
	}

	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling profile: %w", err)
	}

	if err := os.WriteFile(destPath, data, 0o644); err != nil {
		return fmt.Errorf("writing profile: %w", err)
	}

	fmt.Printf("Created profile %q at %s\n", name, destPath)
	return nil
}

// --- delete ---

func newProfileDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <name>",
		Short: "Delete a user profile",
		Args:  cobra.ExactArgs(1),
		RunE:  runProfileDelete,
	}
}

func runProfileDelete(cmd *cobra.Command, args []string) error {
	name := args[0]

	// Load the profile to check if it's a built-in.
	p, err := profiles.Load(name)
	if err != nil {
		return fmt.Errorf("profile %q not found: %w", name, err)
	}

	if isBuiltinProfile(*p) {
		return fmt.Errorf("cannot delete built-in profile %q; create a user override to shadow it instead", name)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("resolving home dir: %w", err)
	}

	destPath := filepath.Join(home, ".hitch", "profiles", name+".json")
	if err := os.Remove(destPath); err != nil {
		return fmt.Errorf("deleting profile: %w", err)
	}

	fmt.Printf("Deleted profile %q (%s)\n", name, destPath)
	return nil
}
