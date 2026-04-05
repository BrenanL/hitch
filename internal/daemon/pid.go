package daemon

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

// PIDInfo holds daemon process metadata written to the PID file.
type PIDInfo struct {
	PID       int    `json:"pid"`
	Port      int    `json:"port"`
	StartedAt string `json:"started_at"`
}

// DefaultPIDPath returns the standard PID file path (~/.hitch/daemon.pid).
func DefaultPIDPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".hitch", "daemon.pid")
}

// WritePID writes PID info to the given file path.
func WritePID(path string, info PIDInfo) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("creating PID directory: %w", err)
	}
	data, err := json.Marshal(info)
	if err != nil {
		return fmt.Errorf("marshaling PID info: %w", err)
	}
	return os.WriteFile(path, data, 0o644)
}

// ReadPID reads PID info from the given file path.
func ReadPID(path string) (*PIDInfo, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var info PIDInfo
	if err := json.Unmarshal(data, &info); err != nil {
		return nil, fmt.Errorf("parsing PID file: %w", err)
	}
	return &info, nil
}

// RemovePID removes the PID file at the given path.
func RemovePID(path string) error {
	return os.Remove(path)
}

// IsRunning checks whether the process described by info is alive.
func IsRunning(info *PIDInfo) bool {
	if info == nil {
		return false
	}
	proc, err := os.FindProcess(info.PID)
	if err != nil {
		return false
	}
	// Signal 0 checks if process exists without actually sending a signal
	return proc.Signal(syscall.Signal(0)) == nil
}

// UptimeString returns a human-readable uptime from the PID info's StartedAt.
func UptimeString(info *PIDInfo) string {
	started, err := time.Parse(time.RFC3339, info.StartedAt)
	if err != nil {
		return "unknown"
	}
	return formatDuration(time.Since(started))
}

func formatDuration(d time.Duration) string {
	d = d.Round(time.Second)
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60
	if h > 0 {
		return fmt.Sprintf("%dh%02dm%02ds", h, m, s)
	}
	if m > 0 {
		return fmt.Sprintf("%dm%02ds", m, s)
	}
	return fmt.Sprintf("%ds", s)
}
