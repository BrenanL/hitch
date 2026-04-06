package tui

import (
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	styleExplorerHeader = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("214"))

	styleExplorerDir = lipgloss.NewStyle().
				Foreground(lipgloss.Color("39")) // blue

	styleExplorerFile = lipgloss.NewStyle().
				Foreground(lipgloss.Color("250"))

	styleExplorerSize = lipgloss.NewStyle().
				Foreground(lipgloss.Color("245"))

	styleExplorerMissing = lipgloss.NewStyle().
				Foreground(lipgloss.Color("245"))
)

// explorerEntry is a displayable row in the explorer tab.
type explorerEntry struct {
	isHeader   bool
	label      string // header text
	indent     string
	name       string
	isDir      bool
	fileCount  int    // for directories
	sizeBytes  int64  // for files
}

// ExplorerTab implements tabModel for Tab 5 (Explorer).
type ExplorerTab struct {
	lines  []string
	scroll int
	cwd    string
}

// NewExplorerTab creates an ExplorerTab for the given cwd.
func NewExplorerTab(cwd string) *ExplorerTab {
	t := &ExplorerTab{cwd: cwd}
	t.load()
	return t
}

func (t *ExplorerTab) load() {
	cwd := t.cwd
	if cwd == "" {
		var err error
		cwd, err = os.Getwd()
		if err != nil {
			cwd = "."
		}
	}

	home, _ := os.UserHomeDir()
	globalClaudeDir := filepath.Join(home, ".claude")
	projectClaudeDir := filepath.Join(cwd, ".claude")

	// Key project directories to survey.
	keyDirs := []string{"internal", "pkg", "cmd", "docs"}

	var lines []string

	// Section 1: Project structure summary
	lines = append(lines, styleExplorerHeader.Render("PROJECT STRUCTURE"))
	lines = append(lines, "  "+styleExplorerMissing.Render("cwd: "+shortenPath(cwd, home)))
	lines = append(lines, "")

	for _, dir := range keyDirs {
		dirPath := filepath.Join(cwd, dir)
		info, err := os.Stat(dirPath)
		if err != nil || !info.IsDir() {
			continue
		}
		count := countFiles(dirPath)
		lines = append(lines,
			"  "+styleExplorerDir.Render(dir+"/")+
				"  "+styleExplorerSize.Render("("+itoa(count)+" files)"),
		)
	}
	lines = append(lines, "")

	// Section 2: ~/.claude/ directory
	lines = append(lines, styleExplorerHeader.Render("~/.claude/"))
	globalLines := buildDirTree(globalClaudeDir, home, 1, 2)
	if len(globalLines) == 0 {
		lines = append(lines, styleExplorerMissing.Render("  (not found)"))
	} else {
		lines = append(lines, globalLines...)
	}
	lines = append(lines, "")

	// Section 3: .claude/ (project-scoped)
	lines = append(lines, styleExplorerHeader.Render(".claude/  (project)"))
	projectLines := buildDirTree(projectClaudeDir, home, 1, 2)
	if len(projectLines) == 0 {
		lines = append(lines, styleExplorerMissing.Render("  (no .claude/ directory in project)"))
	} else {
		lines = append(lines, projectLines...)
	}

	t.lines = lines
}

// buildDirTree renders a directory tree up to maxDepth as a list of styled strings.
func buildDirTree(dir, home string, depth, maxDepth int) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	indent := strings.Repeat("  ", depth)
	var lines []string

	for _, entry := range entries {
		if depth == maxDepth {
			// At max depth, just show count.
			if entry.IsDir() {
				subPath := filepath.Join(dir, entry.Name())
				count := countFiles(subPath)
				lines = append(lines,
					indent+styleExplorerDir.Render(entry.Name()+"/")+
						"  "+styleExplorerSize.Render("("+itoa(count)+" files)"),
				)
			} else {
				info, err := entry.Info()
				size := int64(0)
				if err == nil {
					size = info.Size()
				}
				lines = append(lines,
					indent+styleExplorerFile.Render(entry.Name())+
						"  "+styleExplorerSize.Render(formatSize(size)),
				)
			}
			continue
		}

		if entry.IsDir() {
			lines = append(lines, indent+styleExplorerDir.Render(entry.Name()+"/"))
			subLines := buildDirTree(filepath.Join(dir, entry.Name()), home, depth+1, maxDepth)
			lines = append(lines, subLines...)
		} else {
			info, err := entry.Info()
			size := int64(0)
			if err == nil {
				size = info.Size()
			}
			lines = append(lines,
				indent+styleExplorerFile.Render(entry.Name())+
					"  "+styleExplorerSize.Render(formatSize(size)),
			)
		}
	}
	return lines
}

// countFiles counts files (not directories) recursively.
func countFiles(dir string) int {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}
	count := 0
	for _, e := range entries {
		if e.IsDir() {
			count += countFiles(filepath.Join(dir, e.Name()))
		} else {
			count++
		}
	}
	return count
}

// formatSize formats bytes as a human-friendly string.
func formatSize(b int64) string {
	if b < 1024 {
		return itoa(int(b)) + " B"
	}
	kb := b / 1024
	if kb < 1024 {
		return itoa(int(kb)) + " KB"
	}
	mb := kb / 1024
	return itoa(int(mb)) + " MB"
}

// shortenPath replaces home directory prefix with ~.
func shortenPath(path, home string) string {
	if home != "" && strings.HasPrefix(path, home) {
		return "~" + path[len(home):]
	}
	return path
}

// Init implements tabModel.
func (t *ExplorerTab) Init() tea.Cmd {
	return nil
}

// Update implements tabModel.
func (t *ExplorerTab) Update(msg tea.Msg) (tabModel, tea.Cmd) {
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
func (t *ExplorerTab) View() string {
	var sb strings.Builder

	if len(t.lines) == 0 {
		sb.WriteString(styleExplorerMissing.Render("  (no content)"))
		return styleContent.Render(sb.String())
	}

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
		sb.WriteString(styleExplorerMissing.Render("  (j/k to scroll, " + itoa(len(t.lines)) + " lines total)"))
	}

	return styleContent.Render(sb.String())
}

// Title implements tabModel.
func (t *ExplorerTab) Title() string {
	return "Explorer"
}
