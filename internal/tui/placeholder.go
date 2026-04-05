package tui

import tea "github.com/charmbracelet/bubbletea"

type placeholderTab struct {
	title string
}

func (p placeholderTab) Init() tea.Cmd {
	return nil
}

func (p placeholderTab) Update(msg tea.Msg) (tabModel, tea.Cmd) {
	return p, nil
}

func (p placeholderTab) View() string {
	return styleContent.Render("Coming soon: " + p.title)
}

func (p placeholderTab) Title() string {
	return p.title
}
