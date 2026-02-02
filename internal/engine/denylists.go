package engine

import (
	_ "embed"
	"os"
	"path/filepath"
	"strings"
)

//go:embed embedded/destructive.txt
var embeddedDestructive string

// LoadDenyLists loads all deny lists: built-in embedded + custom from disk.
func LoadDenyLists() map[string][]string {
	lists := make(map[string][]string)

	// Embedded lists
	lists["destructive"] = parsePatterns(embeddedDestructive)

	// Custom lists from ~/.hitch/deny-lists/
	home, err := os.UserHomeDir()
	if err != nil {
		return lists
	}

	denyDir := filepath.Join(home, ".hitch", "deny-lists")
	entries, err := os.ReadDir(denyDir)
	if err != nil {
		return lists
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))
		data, err := os.ReadFile(filepath.Join(denyDir, entry.Name()))
		if err != nil {
			continue
		}
		lists[name] = parsePatterns(string(data))
	}

	return lists
}

// MatchesDenyList checks if the command matches any pattern in the named deny list.
func MatchesDenyList(command string, lists map[string][]string, listName string) bool {
	patterns, ok := lists[listName]
	if !ok {
		return false
	}

	for _, pattern := range patterns {
		if strings.Contains(command, pattern) {
			return true
		}
	}
	return false
}

// ListDenyLists returns the names of all available deny lists.
func ListDenyLists() []string {
	lists := LoadDenyLists()
	names := make([]string, 0, len(lists))
	for name := range lists {
		names = append(names, name)
	}
	return names
}

// GetDenyList returns the patterns for a named deny list.
func GetDenyList(name string) []string {
	lists := LoadDenyLists()
	return lists[name]
}

func parsePatterns(data string) []string {
	var patterns []string
	for _, line := range strings.Split(data, "\n") {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "#") {
			patterns = append(patterns, line)
		}
	}
	return patterns
}
