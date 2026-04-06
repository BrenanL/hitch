package daemon

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWriteAndReadPID(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.pid")

	info := PIDInfo{PID: 12345, Port: 9801, StartedAt: "2026-04-05T10:00:00Z"}
	if err := WritePID(path, info); err != nil {
		t.Fatalf("WritePID: %v", err)
	}

	got, err := ReadPID(path)
	if err != nil {
		t.Fatalf("ReadPID: %v", err)
	}
	if got.PID != 12345 {
		t.Errorf("PID = %d, want 12345", got.PID)
	}
	if got.Port != 9801 {
		t.Errorf("Port = %d, want 9801", got.Port)
	}
	if got.StartedAt != "2026-04-05T10:00:00Z" {
		t.Errorf("StartedAt = %q, want %q", got.StartedAt, "2026-04-05T10:00:00Z")
	}
}

func TestReadPIDMissingFile(t *testing.T) {
	_, err := ReadPID(filepath.Join(t.TempDir(), "nonexistent.pid"))
	if err == nil {
		t.Error("expected error for missing PID file")
	}
}

func TestRemovePID(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.pid")

	WritePID(path, PIDInfo{PID: 1, Port: 9801, StartedAt: "2026-04-05T10:00:00Z"})
	if err := RemovePID(path); err != nil {
		t.Fatalf("RemovePID: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("PID file should be removed")
	}
}

func TestIsRunningCurrentProcess(t *testing.T) {
	info := &PIDInfo{PID: os.Getpid()}
	if !IsRunning(info) {
		t.Error("current process should be detected as running")
	}
}

func TestIsRunningDeadProcess(t *testing.T) {
	// PID 99999999 is extremely unlikely to exist
	info := &PIDInfo{PID: 99999999}
	if IsRunning(info) {
		t.Error("non-existent PID should not be running")
	}
}

func TestIsRunningNilInfo(t *testing.T) {
	if IsRunning(nil) {
		t.Error("nil info should not be running")
	}
}

func TestUptimeString(t *testing.T) {
	// 1 hour ago
	info := &PIDInfo{StartedAt: "invalid-time"}
	if UptimeString(info) != "unknown" {
		t.Errorf("invalid time should return 'unknown', got %q", UptimeString(info))
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		seconds int
		want    string
	}{
		{5, "5s"},
		{65, "1m05s"},
		{3661, "1h01m01s"},
	}
	for _, tt := range tests {
		got := formatDuration(durationFromSeconds(tt.seconds))
		if got != tt.want {
			t.Errorf("formatDuration(%ds) = %q, want %q", tt.seconds, got, tt.want)
		}
	}
}

func TestWritePIDCreatesDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "dir")
	path := filepath.Join(dir, "test.pid")

	if err := WritePID(path, PIDInfo{PID: 1, Port: 9801}); err != nil {
		t.Fatalf("WritePID: %v", err)
	}

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("PID file should exist: %v", err)
	}
}

func durationFromSeconds(s int) time.Duration {
	return time.Duration(s) * time.Second
}
