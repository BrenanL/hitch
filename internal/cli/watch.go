package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/BrenanL/hitch/internal/daemon"
	"github.com/spf13/cobra"
)

func newWatchCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "watch <session-id>",
		Short: "Live event stream for a session",
		Args:  cobra.ExactArgs(1),
		RunE:  runWatch,
	}
	cmd.Flags().Int("last", 20, "Show last N events before live stream")
	cmd.Flags().Bool("json", false, "Output raw DaemonEvent JSON")
	return cmd
}

func runWatch(cmd *cobra.Command, args []string) error {
	sessionID := args[0]
	last, _ := cmd.Flags().GetInt("last")
	jsonFlag, _ := cmd.Flags().GetBool("json")

	// Show recent events first
	if last > 0 {
		if err := showRecentEvents(sessionID, last, jsonFlag); err != nil {
			fmt.Printf("(warning: could not fetch recent events: %v)\n", err)
		}
	}

	fmt.Printf("--- streaming events for %s (Ctrl+C to stop) ---\n", sessionID)

	// Connect to SSE stream
	url := fmt.Sprintf("http://127.0.0.1:9801/api/sessions/%s/stream", sessionID)
	client := &http.Client{Timeout: 0} // no timeout for streaming

	maxRetries := 10
	retryDelay := 500 * time.Millisecond

	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			time.Sleep(retryDelay)
			if retryDelay < 5*time.Second {
				retryDelay *= 2
			}
		}

		resp, err := client.Get(url)
		if err != nil {
			if attempt < maxRetries-1 {
				fmt.Printf("(reconnecting in %v...)\n", retryDelay)
				continue
			}
			return fmt.Errorf("cannot connect to daemon: %w", err)
		}

		// Reset retry state on successful connection
		retryDelay = 500 * time.Millisecond

		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, "data: ") {
				data := line[6:]
				if jsonFlag {
					fmt.Println(data)
				} else {
					var evt daemon.DaemonEvent
					if json.Unmarshal([]byte(data), &evt) == nil {
						printEvent(evt)
					}
				}
			}
		}
		resp.Body.Close()

		if scanner.Err() != nil {
			fmt.Printf("(stream disconnected: %v, reconnecting...)\n", scanner.Err())
		} else {
			fmt.Println("(stream ended, reconnecting...)")
		}
	}

	return fmt.Errorf("max retries exceeded")
}

func showRecentEvents(sessionID string, limit int, jsonFlag bool) error {
	client := &http.Client{Timeout: 2 * time.Second}
	url := fmt.Sprintf("http://127.0.0.1:9801/api/sessions/%s/events?limit=%d", sessionID, limit)
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var events []daemon.DaemonEvent
	json.NewDecoder(resp.Body).Decode(&events)

	// Print in chronological order (API returns newest first)
	for i := len(events) - 1; i >= 0; i-- {
		if jsonFlag {
			data, _ := json.Marshal(events[i])
			fmt.Println(string(data))
		} else {
			printEvent(events[i])
		}
	}
	return nil
}

func printEvent(evt daemon.DaemonEvent) {
	ts := evt.Timestamp
	if t, err := time.Parse(time.RFC3339, ts); err == nil {
		ts = t.Format("15:04:05")
	}

	sourceTag := ""
	switch evt.Source {
	case "proxy":
		sourceTag = "[proxy]"
	case "hooks":
		sourceTag = "[hook]"
	case "jsonl":
		sourceTag = "[jsonl]"
	}

	fmt.Printf("%s %s %s\n", ts, sourceTag, evt.Description)
}
