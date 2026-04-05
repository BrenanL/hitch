package cli

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/BrenanL/hitch/internal/daemon"
	"github.com/spf13/cobra"
)

func newDashboardCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "dashboard",
		Short: "Full-screen monitoring dashboard",
		RunE:  runDashboard,
	}
}

func runDashboard(cmd *cobra.Command, args []string) error {
	p := tea.NewProgram(newDashboardModel(), tea.WithAltScreen())
	_, err := p.Run()
	return err
}

// --- Bubbletea model ---

type dashboardModel struct {
	sessions     []daemon.SessionSummary
	selected     int
	detail       *daemon.SessionDetail
	alerts       []daemon.Alert
	width        int
	height       int
	err          error
	loading      bool
	lastRefresh  time.Time
}

type tickMsg time.Time
type sessionsMsg []daemon.SessionSummary
type detailMsg *daemon.SessionDetail
type alertsMsg []daemon.Alert
type errMsg error

func newDashboardModel() dashboardModel {
	return dashboardModel{loading: true}
}

func (m dashboardModel) Init() tea.Cmd {
	return tea.Batch(
		fetchSessionsCmd(),
		fetchAlertsCmd(),
		tickCmd(),
	)
}

func (m dashboardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "j", "down":
			if m.selected < len(m.sessions)-1 {
				m.selected++
				return m, fetchDetailCmd(m.sessions[m.selected].SessionID)
			}
		case "k", "up":
			if m.selected > 0 {
				m.selected--
				return m, fetchDetailCmd(m.sessions[m.selected].SessionID)
			}
		case "enter":
			if len(m.sessions) > 0 {
				return m, fetchDetailCmd(m.sessions[m.selected].SessionID)
			}
		case "r":
			m.loading = true
			return m, tea.Batch(fetchSessionsCmd(), fetchAlertsCmd())
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case sessionsMsg:
		m.sessions = msg
		m.loading = false
		m.lastRefresh = time.Now()
		if len(m.sessions) > 0 && m.detail == nil {
			return m, fetchDetailCmd(m.sessions[m.selected].SessionID)
		}

	case detailMsg:
		m.detail = msg

	case alertsMsg:
		m.alerts = msg

	case errMsg:
		m.err = msg
		m.loading = false

	case tickMsg:
		return m, tea.Batch(fetchSessionsCmd(), fetchAlertsCmd(), tickCmd())
	}

	return m, nil
}

func (m dashboardModel) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	// Styles
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))
	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("14"))
	activeStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	alertStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("11"))

	var b strings.Builder

	// Title bar
	title := titleStyle.Render("  Hitch Dashboard")
	if m.loading {
		title += dimStyle.Render("  (refreshing...)")
	}
	b.WriteString(title + "\n\n")

	if m.err != nil {
		b.WriteString(fmt.Sprintf("  Error: %v\n", m.err))
		b.WriteString("  Is the daemon running? Try: ht daemon start\n")
		return b.String()
	}

	if len(m.sessions) == 0 {
		b.WriteString("  No sessions found\n")
		b.WriteString(dimStyle.Render("  Waiting for Claude Code sessions...\n"))
		return b.String()
	}

	// Left panel: session list
	leftWidth := 50
	if m.width > 120 {
		leftWidth = 60
	}
	rightWidth := m.width - leftWidth - 4

	// Session list
	b.WriteString(headerStyle.Render("  Sessions") + "\n")
	maxSessions := m.height - 10
	if maxSessions > len(m.sessions) {
		maxSessions = len(m.sessions)
	}
	if maxSessions < 1 {
		maxSessions = 1
	}

	for i := 0; i < maxSessions; i++ {
		s := m.sessions[i]
		cursor := "  "
		if i == m.selected {
			cursor = "▸ "
		}

		indicator := activeStyle.Render("●")
		if !s.IsActive {
			indicator = dimStyle.Render("○")
		}

		id := s.SessionID
		if len(id) > 8 {
			id = id[:8]
		}

		model := shortModelName(s.Model)
		burn := "—"
		if s.BurnRateTPM > 0 {
			burn = fmt.Sprintf("%.0f/m", s.BurnRateTPM)
		}

		line := fmt.Sprintf("%s%s %-8s %-10s %8s $%.2f",
			cursor, indicator, id, model, burn, s.TotalCostUSD)

		if i == m.selected {
			line = lipgloss.NewStyle().Bold(true).Render(line)
		}
		b.WriteString(line + "\n")
	}

	b.WriteString("\n")

	// Right panel: detail
	if m.detail != nil && rightWidth > 20 {
		b.WriteString(headerStyle.Render("  Session Detail: "+m.detail.SessionID[:dashMin(12, len(m.detail.SessionID))]) + "\n")
		b.WriteString(fmt.Sprintf("  Model:        %s\n", shortModelName(m.detail.Model)))
		b.WriteString(fmt.Sprintf("  Input:        %s tokens\n", formatTokenCount(m.detail.TotalInputTokens)))
		b.WriteString(fmt.Sprintf("  Output:       %s tokens\n", formatTokenCount(m.detail.TotalOutputTokens)))
		b.WriteString(fmt.Sprintf("  Cache Read:   %s tokens\n", formatTokenCount(m.detail.TotalCacheRead)))
		b.WriteString(fmt.Sprintf("  Requests:     %d\n", m.detail.RequestCount))
		b.WriteString(fmt.Sprintf("  Compactions:  %d\n", m.detail.CompactionCount))
		b.WriteString(fmt.Sprintf("  Cost:         $%.2f\n", m.detail.TotalCostUSD))
		b.WriteString(fmt.Sprintf("  Burn Rate:    %.0f tok/min\n", m.detail.BurnRateTPM))

		// Sparkline
		if len(m.detail.BurnRateSamples) > 2 {
			b.WriteString("\n" + headerStyle.Render("  Burn Rate") + "\n")
			b.WriteString("  " + renderSparkline(m.detail.BurnRateSamples, dashMin(rightWidth-4, 40)) + "\n")
		}

		// Subagents
		if len(m.detail.ActiveSubagents) > 0 {
			b.WriteString("\n" + headerStyle.Render("  Subagents") + "\n")
			for _, sa := range m.detail.ActiveSubagents {
				status := activeStyle.Render("●")
				if sa.StoppedAt != nil {
					status = dimStyle.Render("○")
				}
				b.WriteString(fmt.Sprintf("  %s %s (%s)\n", status, sa.AgentID[:dashMin(12, len(sa.AgentID))], sa.AgentType))
			}
		}
	}

	// Alert bar
	if len(m.alerts) > 0 {
		b.WriteString("\n" + alertStyle.Render("  Alerts") + "\n")
		show := dashMin(5, len(m.alerts))
		for i := 0; i < show; i++ {
			a := m.alerts[i]
			ts := a.Timestamp
			if t, err := time.Parse(time.RFC3339, ts); err == nil {
				ts = t.Format("15:04:05")
			}
			b.WriteString(fmt.Sprintf("  %s [%s] %s\n", ts, a.Level, a.Title))
		}
	}

	// Footer
	b.WriteString("\n" + dimStyle.Render("  j/k: navigate  enter: select  r: refresh  q: quit"))

	return b.String()
}

// --- Commands ---

func tickCmd() tea.Cmd {
	return tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func fetchSessionsCmd() tea.Cmd {
	return func() tea.Msg {
		client := &http.Client{Timeout: 2 * time.Second}
		resp, err := client.Get("http://127.0.0.1:9801/api/sessions")
		if err != nil {
			return errMsg(err)
		}
		defer resp.Body.Close()
		var sessions []daemon.SessionSummary
		json.NewDecoder(resp.Body).Decode(&sessions)
		return sessionsMsg(sessions)
	}
}

func fetchDetailCmd(sessionID string) tea.Cmd {
	return func() tea.Msg {
		client := &http.Client{Timeout: 2 * time.Second}
		resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:9801/api/sessions/%s", sessionID))
		if err != nil {
			return nil
		}
		defer resp.Body.Close()
		var detail daemon.SessionDetail
		json.NewDecoder(resp.Body).Decode(&detail)
		return detailMsg(&detail)
	}
}

func fetchAlertsCmd() tea.Cmd {
	return func() tea.Msg {
		client := &http.Client{Timeout: 2 * time.Second}
		resp, err := client.Get("http://127.0.0.1:9801/api/alerts")
		if err != nil {
			return nil
		}
		defer resp.Body.Close()
		var alerts []daemon.Alert
		json.NewDecoder(resp.Body).Decode(&alerts)
		return alertsMsg(alerts)
	}
}

// --- Sparkline ---

func renderSparkline(samples []daemon.BurnPointJSON, width int) string {
	if len(samples) < 2 || width < 2 {
		return ""
	}

	// Bucket samples into width columns
	values := make([]float64, width)
	step := float64(len(samples)) / float64(width)
	for i := 0; i < width; i++ {
		idx := int(float64(i) * step)
		if idx >= len(samples) {
			idx = len(samples) - 1
		}
		values[i] = samples[idx].TokensTPM
	}

	// Find max
	maxVal := 0.0
	for _, v := range values {
		if v > maxVal {
			maxVal = v
		}
	}

	if maxVal == 0 {
		return strings.Repeat("▁", width)
	}

	blocks := []rune{'▁', '▂', '▃', '▄', '▅', '▆', '▇', '█'}
	var sb strings.Builder
	for _, v := range values {
		idx := int(v / maxVal * 7)
		if idx > 7 {
			idx = 7
		}
		sb.WriteRune(blocks[idx])
	}
	return sb.String()
}

// minInt avoids collisions with dashMin() declared elsewhere in the package.
func dashMin(a, b int) int {
	if a < b {
		return a
	}
	return b
}
