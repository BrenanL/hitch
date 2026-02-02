package platform

import (
	"os"
	"runtime"
	"strings"
	"sync"
)

// Platform represents the detected platform.
type Platform int

const (
	PlatformLinux Platform = iota
	PlatformMacOS
	PlatformWSL
)

var (
	detectedPlatform Platform
	detectOnce       sync.Once
)

// DetectOS returns the detected platform.
func DetectOS() Platform {
	detectOnce.Do(func() {
		switch runtime.GOOS {
		case "darwin":
			detectedPlatform = PlatformMacOS
		case "linux":
			if IsWSL() {
				detectedPlatform = PlatformWSL
			} else {
				detectedPlatform = PlatformLinux
			}
		default:
			detectedPlatform = PlatformLinux
		}
	})
	return detectedPlatform
}

// IsWSL returns true if running under Windows Subsystem for Linux.
func IsWSL() bool {
	data, err := os.ReadFile("/proc/version")
	if err != nil {
		return false
	}
	return strings.Contains(strings.ToLower(string(data)), "microsoft")
}

// IsMacOS returns true if running on macOS.
func IsMacOS() bool {
	return runtime.GOOS == "darwin"
}

// IsLinux returns true if running on native Linux (not WSL).
func IsLinux() bool {
	return runtime.GOOS == "linux" && !IsWSL()
}

// String returns the platform name.
func (p Platform) String() string {
	switch p {
	case PlatformMacOS:
		return "macOS"
	case PlatformWSL:
		return "WSL"
	default:
		return "Linux"
	}
}
