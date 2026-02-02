package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/BrenanL/hitch/internal/adapters"
	"github.com/spf13/cobra"
)

func newNotifyCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "notify <channel> <message>",
		Short: "Send a notification directly",
		Args:  cobra.ExactArgs(2),
		RunE:  runNotify,
	}
}

func runNotify(cmd *cobra.Command, args []string) error {
	channelName := args[0]
	message := args[1]

	db, _, err := openDB()
	if err != nil {
		return err
	}
	defer db.Close()

	adapter, err := resolveAdapterFromDB(db, channelName)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result := adapter.Send(ctx, adapters.Message{
		Title: "Hitch",
		Body:  message,
		Level: adapters.Info,
	})

	if !result.Success {
		return fmt.Errorf("send failed: %v", result.Error)
	}

	db.ChannelUpdateLastUsed(channelName)
	fmt.Printf("Notification sent to %s.\n", channelName)
	return nil
}
