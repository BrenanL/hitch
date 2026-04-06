package tui

import (
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	styleMemoryHeader = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("214"))

	styleMemoryLine = lipgloss.NewStyle().
			Foreground(lipgloss.Color("250"))

	styleMemoryCursor = lipgloss.NewStyle().
				Foreground(lipgloss.Color("15")).
				Background(lipgloss.Color("62"))

	styleMemoryMissing = lipgloss.NewStyle().
				Foreground(lipgloss.Color("245"))
)

// memoryEntry is a single displayable item in the memory tab.
type memoryEntry struct {
	isHeader bool
	label    string // header label
	path     string // file path (if not header)
	exists   bool
	lines    []string // file content lines (loaded on demand)
	loaded   bool
}

// MemoryTab implements tabModel for Tab 4 (Memory).
type MemoryTab struct {
	entries  []memoryEntry
	cursor   int
	scroll   int
	lines    []string // flat rendered lines for scrolling
	cwd      string
}

// NewMemoryTab creates a MemoryTab looking for MEMORY.md in the project.
// cwd is the project directory.
func NewMemoryTab(cwd string) *MemoryTab {
	t := &MemoryTab{cwd: cwd}
	t.load()
	return t
}

func (t *MemoryTab) load() {
	cwd := t.cwd
	if cwd == "" {
		var err error
		cwd, err = os.Getwd()
		if err != nil {
			cwd = "."
		}
	}

	// Compute the auto-memory path: ~/.claude/projects/<url-encoded-cwd>/memory/MEMORY.md
	home, _ := os.UserHomeDir()
	encodedCWD := urlEncodePath(cwd)
	memoryPath := filepath.Join(home, ".claude", "projects", encodedCWD, "memory", "MEMORY.md")

	var allLines []string

	// Header
	allLines = append(allLines, styleMemoryHeader.Render("MEMORY.md"))
	allLines = append(allLines, styleMemoryMissing.Render("  Path: "+memoryPath))

	content, err := os.ReadFile(memoryPath)
	if err != nil {
		if os.IsNotExist(err) {
			allLines = append(allLines, styleMemoryMissing.Render("  (file not found)"))
		} else {
			allLines = append(allLines, styleMemoryMissing.Render("  (error reading file: "+err.Error()+")"))
		}
	} else {
		allLines = append(allLines, "")
		for _, line := range strings.Split(string(content), "\n") {
			allLines = append(allLines, styleMemoryLine.Render("  "+line))
		}
	}

	t.lines = allLines
}

// Init implements tabModel.
func (t *MemoryTab) Init() tea.Cmd {
	return nil
}

// Update implements tabModel.
func (t *MemoryTab) Update(msg tea.Msg) (tabModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "j", "down":
			if t.scroll < len(t.lines)-1 {
				t.scroll++
			}
		case "k", "up":
			if t.scroll > 0 {
				t.scroll--
			}
		case "g":
			t.scroll = 0
		case "G":
			t.scroll = len(t.lines) - 1
			if t.scroll < 0 {
				t.scroll = 0
			}
		}
	}
	return t, nil
}

// View implements tabModel.
func (t *MemoryTab) View() string {
	var sb strings.Builder

	if len(t.lines) == 0 {
		sb.WriteString(styleMemoryMissing.Render("  (no content)"))
		return styleContent.Render(sb.String())
	}

	// Show lines starting from scroll offset, up to a reasonable limit.
	const maxLines = 40
	start := t.scroll
	if start > len(t.lines)-1 {
		start = len(t.lines) - 1
	}
	end := start + maxLines
	if end > len(t.lines) {
		end = len(t.lines)
	}

	for _, line := range t.lines[start:end] {
		sb.WriteString(line)
		sb.WriteString("\n")
	}

	if len(t.lines) > maxLines {
		sb.WriteString(styleMemoryMissing.Render("  (j/k to scroll, " + itoa(len(t.lines)) + " lines total)"))
	}

	return styleContent.Render(sb.String())
}

// Title implements tabModel.
func (t *MemoryTab) Title() string {
	return "Memory"
}

// urlEncodePath encodes a filesystem path for use as a CC project key.
// CC uses URL encoding: slashes become %2F, etc.
func urlEncodePath(path string) string {
	var b strings.Builder
	for _, c := range path {
		switch {
		case c >= 'A' && c <= 'Z':
			b.WriteRune(c)
		case c >= 'a' && c <= 'z':
			b.WriteRune(c)
		case c >= '0' && c <= '9':
			b.WriteRune(c)
		case c == '-' || c == '_' || c == '.' || c == '~':
			b.WriteRune(c)
		default:
			b.WriteString("%")
			b.WriteByte(hexChar(byte(c) >> 4))
			b.WriteByte(hexChar(byte(c) & 0xf))
		}
	}
	return b.String()
}

func hexChar(n byte) byte {
	if n < 10 {
		return '0' + n
	}
	return 'A' + n - 10
}
