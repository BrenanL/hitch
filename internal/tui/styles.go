package tui

import "github.com/charmbracelet/lipgloss"

var (
	styleTabBar = lipgloss.NewStyle().
			Background(lipgloss.Color("236")).
			Padding(0, 1)

	styleActiveTab = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("15")).
			Background(lipgloss.Color("62")).
			Padding(0, 1)

	styleInactiveTab = lipgloss.NewStyle().
				Foreground(lipgloss.Color("245")).
				Background(lipgloss.Color("236")).
				Padding(0, 1)

	styleStatusBar = lipgloss.NewStyle().
			Foreground(lipgloss.Color("245")).
			Background(lipgloss.Color("236")).
			Padding(0, 1)

	styleHelpOverlay = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("62")).
				Padding(1, 2).
				Background(lipgloss.Color("236")).
				Foreground(lipgloss.Color("15"))

	styleContent = lipgloss.NewStyle().
			Padding(1, 2)
)
