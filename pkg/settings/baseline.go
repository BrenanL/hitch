package settings

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// DefaultBaseline returns Hitch's recommended out-of-the-box configuration as a
// plain map[string]interface{}. The map is structured to mirror the shape of
// settings.json: top-level keys map directly to settings keys, and "env" is a
// nested map[string]string.
//
// If proxyURL is empty, "env.ANTHROPIC_BASE_URL" is omitted from the returned map.
//
// Keys returned:
//
//	effortLevel                          = "medium"
//	showThinkingSummaries                = true
//	env.ANTHROPIC_BASE_URL               = proxyURL  (omitted if proxyURL == "")
//	env.CLAUDE_CODE_ENABLE_TELEMETRY     = "1"
//	env.CLAUDE_ENABLE_STREAM_WATCHDOG    = "1"
//	env.DISABLE_TELEMETRY                = "1"
//	env.DISABLE_ERROR_REPORTING          = "1"
//	env.CLAUDE_CODE_DEBUG_LOG_LEVEL      = "info"
func DefaultBaseline(proxyURL string) map[string]interface{} {
	env := map[string]string{
		"CLAUDE_CODE_ENABLE_TELEMETRY":  "1",
		"CLAUDE_ENABLE_STREAM_WATCHDOG": "1",
		"DISABLE_TELEMETRY":             "1",
		"DISABLE_ERROR_REPORTING":       "1",
		"CLAUDE_CODE_DEBUG_LOG_LEVEL":   "info",
	}
	if proxyURL != "" {
		env["ANTHROPIC_BASE_URL"] = proxyURL
	}
	return map[string]interface{}{
		"effortLevel":          "medium",
		"showThinkingSummaries": true,
		"env":                  env,
	}
}

// LoadHitchDefaults reads the file at hitchConfigPath (typically ~/.hitch/config.toml)
// and returns the env vars and settings keys it specifies as a plain map.
// Returns an empty map (not an error) if the file does not exist.
//
// The returned map has the same shape as DefaultBaseline: top-level keys are
// settings keys and "env" is a nested map[string]string.
//
// The config.toml format uses simple key = value pairs:
//
//	effortLevel = "medium"
//	showThinkingSummaries = true
//
//	[env]
//	ANTHROPIC_BASE_URL = "http://localhost:9800"
//	DISABLE_TELEMETRY = "1"
func LoadHitchDefaults(hitchConfigPath string) (map[string]interface{}, error) {
	if hitchConfigPath == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return map[string]interface{}{}, nil
		}
		hitchConfigPath = filepath.Join(home, ".hitch", "config.toml")
	}

	data, err := os.ReadFile(hitchConfigPath)
	if os.IsNotExist(err) {
		return map[string]interface{}{}, nil
	}
	if err != nil {
		return nil, err
	}

	result := map[string]interface{}{}
	env := map[string]string{}
	inEnvSection := false

	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if line == "[env]" {
			inEnvSection = true
			continue
		}
		if strings.HasPrefix(line, "[") {
			inEnvSection = false
			continue
		}
		key, val, ok := parseTomlLine(line)
		if !ok {
			continue
		}
		if inEnvSection {
			env[key] = stripQuotes(val)
		} else {
			result[key] = tomlValue(val)
		}
	}

	if len(env) > 0 {
		result["env"] = env
	}
	return result, nil
}

// parseTomlLine parses a "key = value" line. Returns (key, rawValue, true) or ("", "", false).
func parseTomlLine(line string) (string, string, bool) {
	idx := strings.IndexByte(line, '=')
	if idx < 0 {
		return "", "", false
	}
	key := strings.TrimSpace(line[:idx])
	val := strings.TrimSpace(line[idx+1:])
	if key == "" {
		return "", "", false
	}
	// Strip inline comments
	if i := strings.Index(val, " #"); i >= 0 {
		val = strings.TrimSpace(val[:i])
	}
	return key, val, true
}

// tomlValue converts a raw TOML value string to a Go interface{} value.
// Handles quoted strings, booleans, and integers. Anything else is returned as a string.
func tomlValue(raw string) interface{} {
	if len(raw) >= 2 && raw[0] == '"' && raw[len(raw)-1] == '"' {
		return raw[1 : len(raw)-1]
	}
	if raw == "true" {
		return true
	}
	if raw == "false" {
		return false
	}
	return raw
}

// stripQuotes removes surrounding double-quotes from a string if present.
func stripQuotes(s string) string {
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		return s[1 : len(s)-1]
	}
	return s
}
