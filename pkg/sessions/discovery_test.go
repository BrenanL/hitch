package sessions

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestDiscoverProjects verifies that DiscoverProjects returns one ProjectInfo per
// subdirectory in claudeDir/projects/ sorted by last activity.
func TestDiscoverProjects(t *testing.T) {
	claudeDir := t.TempDir()
	projectsDir := filepath.Join(claudeDir, "projects")

	// Create two project directories.
	proj1 := filepath.Join(projectsDir, "-home-user-dev-hitch")
	proj2 := filepath.Join(projectsDir, "-home-user-exp-other")
	if err := os.MkdirAll(proj1, 0755); err != nil {
		t.Fatalf("MkdirAll proj1: %v", err)
	}
	if err := os.MkdirAll(proj2, 0755); err != nil {
		t.Fatalf("MkdirAll proj2: %v", err)
	}

	// Write a JSONL file into proj1 to give it a recent mtime.
	uuid1 := "a1b2c3d4-e5f6-7890-abcd-ef1234567890"
	f1 := filepath.Join(proj1, uuid1+".jsonl")
	if err := os.WriteFile(f1, []byte(`{"type":"user"}`+"\n"), 0600); err != nil {
		t.Fatalf("WriteFile f1: %v", err)
	}

	// proj2 gets an older mtime by adjusting it.
	if err := os.Chtimes(proj2, time.Time{}, time.Now().Add(-2*time.Hour)); err != nil {
		t.Fatalf("Chtimes proj2: %v", err)
	}

	projects, err := DiscoverProjects(claudeDir)
	if err != nil {
		t.Fatalf("DiscoverProjects: %v", err)
	}

	if len(projects) != 2 {
		t.Fatalf("len(projects) = %d, want 2", len(projects))
	}

	// proj1 should be first (more recent activity).
	if projects[0].EncodedName != "-home-user-dev-hitch" {
		t.Errorf("projects[0].EncodedName = %q, want -home-user-dev-hitch", projects[0].EncodedName)
	}
	if projects[0].SessionCount != 1 {
		t.Errorf("projects[0].SessionCount = %d, want 1", projects[0].SessionCount)
	}
	if projects[0].OriginalPath == "" {
		t.Error("projects[0].OriginalPath is empty")
	}
	if projects[0].DirPath != proj1 {
		t.Errorf("projects[0].DirPath = %q, want %q", projects[0].DirPath, proj1)
	}
}

// TestDiscoverProjectsSessionsIndexJSON verifies that sessions-index.json is used
// for the OriginalPath when present.
func TestDiscoverProjectsSessionsIndexJSON(t *testing.T) {
	claudeDir := t.TempDir()
	projectsDir := filepath.Join(claudeDir, "projects")
	proj := filepath.Join(projectsDir, "-home-user-dev-myproject")
	if err := os.MkdirAll(proj, 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	// Write sessions-index.json.
	indexContent := `{"originalPath":"/home/user/dev/myproject"}`
	if err := os.WriteFile(filepath.Join(proj, "sessions-index.json"), []byte(indexContent), 0600); err != nil {
		t.Fatalf("WriteFile sessions-index.json: %v", err)
	}

	projects, err := DiscoverProjects(claudeDir)
	if err != nil {
		t.Fatalf("DiscoverProjects: %v", err)
	}

	if len(projects) != 1 {
		t.Fatalf("len(projects) = %d, want 1", len(projects))
	}
	if projects[0].OriginalPath != "/home/user/dev/myproject" {
		t.Errorf("OriginalPath = %q, want /home/user/dev/myproject", projects[0].OriginalPath)
	}
}

// TestDiscoverProjectsEmptyDir verifies that a missing projects/ directory returns
// an error (not a panic).
func TestDiscoverProjectsEmptyDir(t *testing.T) {
	claudeDir := t.TempDir()
	// Don't create projects/ subdirectory.
	_, err := DiscoverProjects(claudeDir)
	if err == nil {
		t.Error("expected error for missing projects directory, got nil")
	}
}

// TestDiscoverSessions verifies that DiscoverSessions returns only UUID-named
// JSONL files sorted by last-modified.
func TestDiscoverSessions(t *testing.T) {
	projectDir := t.TempDir()

	uuid1 := "a1b2c3d4-e5f6-7890-abcd-ef1234567890"
	uuid2 := "b2c3d4e5-f6a7-8901-bcde-f01234567891"

	// Write two session files with distinct content.
	content := `{"type":"user","uuid":"u1","sessionId":"` + uuid1 + `","timestamp":"2026-04-04T10:00:00.000Z","isSidechain":false,"message":{"role":"user","content":"hi"}}` + "\n"
	if err := os.WriteFile(filepath.Join(projectDir, uuid1+".jsonl"), []byte(content), 0600); err != nil {
		t.Fatalf("WriteFile uuid1: %v", err)
	}
	content2 := `{"type":"user","uuid":"u2","sessionId":"` + uuid2 + `","timestamp":"2026-04-04T11:00:00.000Z","isSidechain":false,"message":{"role":"user","content":"hello"}}` + "\n"
	if err := os.WriteFile(filepath.Join(projectDir, uuid2+".jsonl"), []byte(content2), 0600); err != nil {
		t.Fatalf("WriteFile uuid2: %v", err)
	}

	// Write a non-UUID file that should be filtered out.
	if err := os.WriteFile(filepath.Join(projectDir, "agent-abc.jsonl"), []byte("{}"), 0600); err != nil {
		t.Fatalf("WriteFile agent: %v", err)
	}

	// Give uuid2 a more recent mtime so it sorts first.
	future := time.Now().Add(time.Minute)
	if err := os.Chtimes(filepath.Join(projectDir, uuid2+".jsonl"), future, future); err != nil {
		t.Fatalf("Chtimes: %v", err)
	}

	summaries, err := DiscoverSessions(projectDir)
	if err != nil {
		t.Fatalf("DiscoverSessions: %v", err)
	}

	if len(summaries) != 2 {
		t.Fatalf("len(summaries) = %d, want 2", len(summaries))
	}

	// Most recently modified should be first.
	if summaries[0].ID != uuid2 {
		t.Errorf("summaries[0].ID = %q, want %q", summaries[0].ID, uuid2)
	}
	if summaries[1].ID != uuid1 {
		t.Errorf("summaries[1].ID = %q, want %q", summaries[1].ID, uuid1)
	}

	// Verify each summary has a non-empty TranscriptPath.
	for _, s := range summaries {
		if s.TranscriptPath == "" {
			t.Errorf("summary %q has empty TranscriptPath", s.ID)
		}
		if s.FileSize == 0 {
			t.Errorf("summary %q has FileSize=0", s.ID)
		}
	}
}

// TestParseProjectDirRoundtrip verifies the heuristic decode for common path patterns.
func TestParseProjectDirRoundtrip(t *testing.T) {
	cases := []struct {
		encodedDir string
		wantPrefix string // just check the path starts with /
	}{
		{"-home-user-dev-hitch", "/home/user/dev/hitch"},
		{"-home-user-exp-other", "/home/user/exp/other"},
	}

	for _, tc := range cases {
		dir := t.TempDir()
		projectDir := filepath.Join(dir, tc.encodedDir)
		if err := os.MkdirAll(projectDir, 0755); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}

		// ParseProjectDir takes a transcript path, not just a dir name.
		transcriptPath := filepath.Join(projectDir, "fake.jsonl")
		got := ParseProjectDir(transcriptPath)
		if got != tc.wantPrefix {
			t.Errorf("ParseProjectDir(%q) = %q, want %q", tc.encodedDir, got, tc.wantPrefix)
		}
	}
}

// TestDiscoverSessionsEmptyDir verifies that DiscoverSessions on a dir with no
// JSONL files returns an empty (non-nil) slice without error.
func TestDiscoverSessionsEmptyDir(t *testing.T) {
	projectDir := t.TempDir()
	summaries, err := DiscoverSessions(projectDir)
	if err != nil {
		t.Fatalf("DiscoverSessions on empty dir returned error: %v", err)
	}
	if len(summaries) != 0 {
		t.Errorf("len(summaries) = %d, want 0", len(summaries))
	}
}

// TestIsValidUUIDUppercase verifies that isValidUUID accepts uppercase hex letters
// (A-F) since the implementation allows them.
func TestIsValidUUIDUppercase(t *testing.T) {
	upper := "A1B2C3D4-E5F6-7890-ABCD-EF1234567890"
	if !isValidUUID(upper) {
		t.Errorf("isValidUUID(%q) = false, want true (uppercase hex should be valid)", upper)
	}
}

// TestDecodeProjectDirNameDoubleSlash verifies the double-slash collapse logic.
// An encoded name with leading hyphen results in "//" at the start, which must
// be collapsed to a single slash.
func TestDecodeProjectDirNameDoubleSlash(t *testing.T) {
	// "-home" decodes to "/home" (the leading - becomes / then combined with the
	// implicit leading /).
	got := decodeProjectDirName("-home-user")
	if got != "/home/user" {
		t.Errorf("decodeProjectDirName(-home-user) = %q, want /home/user", got)
	}
}

// TestParseProjectDirSessionsIndexJSON verifies sessions-index.json takes precedence.
func TestParseProjectDirSessionsIndexJSON(t *testing.T) {
	dir := t.TempDir()
	projectDir := filepath.Join(dir, "-home-user-dev-hitch")
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	indexContent := `{"originalPath":"/home/user/dev/hitch"}`
	if err := os.WriteFile(filepath.Join(projectDir, "sessions-index.json"), []byte(indexContent), 0600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	got := ParseProjectDir(filepath.Join(projectDir, "fake.jsonl"))
	if got != "/home/user/dev/hitch" {
		t.Errorf("ParseProjectDir = %q, want /home/user/dev/hitch", got)
	}
}
