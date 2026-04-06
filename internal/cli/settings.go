package cli

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/BrenanL/hitch/internal/tui"
)

func newSettingsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "settings",
		Short: "Open the interactive settings TUI",
		RunE: func(cmd *cobra.Command, args []string) error {
			p := tea.NewProgram(tui.New(), tea.WithAltScreen())
			_, err := p.Run()
			return err
		},
	}
}
