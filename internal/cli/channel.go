package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/BrenanL/hitch/internal/adapters"
	"github.com/BrenanL/hitch/internal/state"
	"github.com/spf13/cobra"
)

func newChannelCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "channel",
		Short: "Manage notification channels",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	cmd.AddCommand(
		newChannelAddCmd(),
		newChannelListCmd(),
		newChannelTestCmd(),
		newChannelRemoveCmd(),
	)
	return cmd
}

func newChannelAddCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "add <adapter> [config...]",
		Short: "Add a notification channel",
		Long: `Add a notification channel.

Examples:
  ht channel add ntfy my-alerts
  ht channel add discord https://discord.com/api/webhooks/123/abc
  ht channel add slack https://hooks.slack.com/services/T.../B.../xxx
  ht channel add desktop`,
		Args: cobra.MinimumNArgs(1),
		RunE: runChannelAdd,
	}
}

func runChannelAdd(cmd *cobra.Command, args []string) error {
	adapterName := args[0]

	// Build config from remaining args
	config := make(map[string]string)
	switch adapterName {
	case "ntfy":
		if len(args) < 2 {
			return fmt.Errorf("ntfy requires a topic name: ht channel add ntfy <topic>")
		}
		config["topic"] = args[1]
		if len(args) > 2 {
			config["server"] = args[2]
		}
	case "discord":
		if len(args) < 2 {
			return fmt.Errorf("discord requires a webhook URL: ht channel add discord <url>")
		}
		config["webhook_url"] = args[1]
	case "slack":
		if len(args) < 2 {
			return fmt.Errorf("slack requires a webhook URL: ht channel add slack <url>")
		}
		config["webhook_url"] = args[1]
	case "desktop":
		// No config needed
	default:
		return fmt.Errorf("unknown adapter %q (available: %s)", adapterName, strings.Join(adapters.AvailableAdapters(), ", "))
	}

	// Validate by creating adapter
	if _, err := adapters.NewAdapter(adapterName, config); err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}

	db, _, err := openDB()
	if err != nil {
		return err
	}
	defer db.Close()

	// Use adapter name as channel ID (or first config value for uniqueness)
	channelID := adapterName
	if len(args) > 1 {
		// For ntfy, use topic as name
		if adapterName == "ntfy" {
			channelID = args[1]
		}
	}

	configJSON, _ := json.Marshal(config)
	ch := state.Channel{
		ID:        channelID,
		Adapter:   adapterName,
		Name:      channelID,
		Config:    string(configJSON),
		Enabled:   true,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	}

	if err := db.ChannelAdd(ch); err != nil {
		return fmt.Errorf("adding channel: %w", err)
	}

	fmt.Printf("Channel %q (%s) added.\n", channelID, adapterName)
	return nil
}

func newChannelListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List configured channels",
		RunE:  runChannelList,
	}
}

func runChannelList(cmd *cobra.Command, args []string) error {
	db, _, err := openDB()
	if err != nil {
		return err
	}
	defer db.Close()

	channels, err := db.ChannelList()
	if err != nil {
		return err
	}

	if len(channels) == 0 {
		fmt.Println("No channels configured. Add one with: ht channel add <adapter> [config...]")
		return nil
	}

	fmt.Printf("%-12s %-10s %-8s %s\n", "NAME", "ADAPTER", "ENABLED", "LAST USED")
	for _, ch := range channels {
		enabled := "yes"
		if !ch.Enabled {
			enabled = "no"
		}
		lastUsed := "-"
		if ch.LastUsedAt != "" {
			lastUsed = ch.LastUsedAt[:19]
		}
		fmt.Printf("%-12s %-10s %-8s %s\n", ch.ID, ch.Adapter, enabled, lastUsed)
	}
	return nil
}

func newChannelTestCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "test [name]",
		Short: "Send a test notification",
		RunE:  runChannelTest,
	}
}

func runChannelTest(cmd *cobra.Command, args []string) error {
	db, _, err := openDB()
	if err != nil {
		return err
	}
	defer db.Close()

	var channels []state.Channel
	if len(args) > 0 {
		ch, err := db.ChannelGet(args[0])
		if err != nil {
			return err
		}
		if ch == nil {
			return fmt.Errorf("channel %q not found", args[0])
		}
		channels = []state.Channel{*ch}
	} else {
		channels, err = db.ChannelList()
		if err != nil {
			return err
		}
	}

	for _, ch := range channels {
		var config map[string]string
		json.Unmarshal([]byte(ch.Config), &config)
		if config == nil {
			config = make(map[string]string)
		}

		adapter, err := adapters.NewAdapter(ch.Adapter, config)
		if err != nil {
			fmt.Printf("%-12s FAILED (config error: %v)\n", ch.ID, err)
			continue
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		result := adapter.Test(ctx)
		cancel()

		if result.Success {
			fmt.Printf("%-12s OK\n", ch.ID)
			db.ChannelUpdateLastUsed(ch.ID)
		} else {
			fmt.Printf("%-12s FAILED (%v)\n", ch.ID, result.Error)
		}
	}
	return nil
}

func newChannelRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "remove <name>",
		Short: "Remove a channel",
		Args:  cobra.ExactArgs(1),
		RunE:  runChannelRemove,
	}
}

func runChannelRemove(cmd *cobra.Command, args []string) error {
	db, _, err := openDB()
	if err != nil {
		return err
	}
	defer db.Close()

	if err := db.ChannelRemove(args[0]); err != nil {
		return err
	}
	fmt.Printf("Channel %q removed.\n", args[0])
	return nil
}
