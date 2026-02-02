package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/BrenanL/hitch/internal/adapters"
	"github.com/BrenanL/hitch/internal/engine"
	"github.com/BrenanL/hitch/internal/state"
	"github.com/BrenanL/hitch/pkg/hookio"
	"github.com/spf13/cobra"
)

func newHookCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "hook",
		Short: "Hook execution and management",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	cmd.AddCommand(
		newHookExecCmd(),
		newHookListCmd(),
	)
	return cmd
}

func newHookExecCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "exec <rule-id>",
		Short: "Execute a hook rule (called by Claude Code)",
		Args:  cobra.ExactArgs(1),
		RunE:  runHookExec,
	}
}

func runHookExec(cmd *cobra.Command, args []string) error {
	ruleRef := args[0]

	// Read input from stdin
	input, err := hookio.ReadStdin()
	if err != nil {
		// If stdin is empty or invalid, create minimal input
		input = &hookio.HookInput{HookEventName: "Unknown"}
	}

	db, _, err := openDB()
	if err != nil {
		// If DB fails, allow by default
		hookio.WriteStdout(hookio.Allow())
		return nil
	}
	defer db.Close()

	// Check mute
	muted, _ := db.IsMuted()

	// Build executor
	exec := &engine.Executor{
		DB: db,
		GetAdapter: func(name string) (adapters.Adapter, error) {
			if muted {
				return nil, fmt.Errorf("notifications muted")
			}
			return resolveAdapterFromDB(db, name)
		},
		DenyLists: engine.LoadDenyLists(),
	}

	// Handle system hooks
	if strings.HasPrefix(ruleRef, "system:") {
		name := strings.TrimPrefix(ruleRef, "system:")
		result := exec.ExecuteSystemHook(name, input)
		if result.Output != nil {
			hookio.WriteStdout(result.Output)
		}
		return nil
	}

	// Look up rule
	rule, err := db.RuleGet(ruleRef)
	if err != nil || rule == nil {
		hookio.WriteStdout(hookio.Allow())
		return nil
	}

	if !rule.Enabled {
		hookio.WriteStdout(hookio.Allow())
		return nil
	}

	// Execute
	result := exec.Execute(context.Background(), rule, input)
	if result.Output != nil {
		hookio.WriteStdout(result.Output)
	} else {
		hookio.WriteStdout(hookio.Allow())
	}

	// Exit 2 if blocked
	if result.Blocked {
		os.Exit(2)
	}

	return nil
}

func newHookListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List registered custom hooks",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("No custom hooks registered.")
			return nil
		},
	}
}

// resolveAdapterFromDB creates an adapter from a channel stored in the database.
func resolveAdapterFromDB(db *state.DB, name string) (adapters.Adapter, error) {
	ch, err := db.ChannelGet(name)
	if err != nil {
		return nil, err
	}
	if ch == nil {
		return nil, fmt.Errorf("channel %q not found", name)
	}

	var config map[string]string
	if err := json.Unmarshal([]byte(ch.Config), &config); err != nil {
		config = make(map[string]string)
	}

	return adapters.NewAdapter(ch.Adapter, config)
}

