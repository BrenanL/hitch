package engine

import (
	"testing"
)

func TestLoadDenyLists(t *testing.T) {
	lists := LoadDenyLists()

	destructive, ok := lists["destructive"]
	if !ok {
		t.Fatal("destructive deny list not found")
	}
	if len(destructive) == 0 {
		t.Fatal("destructive deny list is empty")
	}

	// Check some expected patterns
	found := false
	for _, p := range destructive {
		if p == "rm -rf /" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected 'rm -rf /' in destructive list")
	}
}

func TestMatchesDenyList(t *testing.T) {
	lists := map[string][]string{
		"destructive": {"rm -rf /", "DROP DATABASE", "mkfs"},
	}

	tests := []struct {
		command string
		want    bool
	}{
		{"rm -rf /", true},
		{"rm -rf /home/user", true}, // contains "rm -rf /"
		{"npm test", false},
		{"echo hello", false},
		{"DROP DATABASE mydb", true},
		{"mkfs.ext4 /dev/sda1", true},
	}

	for _, tt := range tests {
		got := MatchesDenyList(tt.command, lists, "destructive")
		if got != tt.want {
			t.Errorf("MatchesDenyList(%q) = %v, want %v", tt.command, got, tt.want)
		}
	}
}

func TestMatchesDenyListUnknown(t *testing.T) {
	lists := map[string][]string{}
	if MatchesDenyList("rm -rf /", lists, "nonexistent") {
		t.Error("expected false for unknown deny list")
	}
}
