package envvars

import (
	"fmt"
	"os"
	"sort"
	"strings"
)

// GetCurrent returns the current OS value for a registered variable.
// Returns ("", false) if not set or not in the registry.
func GetCurrent(name string) (string, bool) {
	if _, ok := Get(name); !ok {
		return "", false
	}
	val, set := os.LookupEnv(name)
	if !set {
		return "", false
	}
	return val, true
}

// GetAllCurrent returns all registry variables that are currently set in the OS environment.
func GetAllCurrent() map[string]string {
	result := make(map[string]string)
	for _, v := range registry {
		if val, ok := os.LookupEnv(v.Name); ok {
			result[v.Name] = val
		}
	}
	return result
}

// Validate checks the current OS environment for problems with registered variables.
// Checks for: required vars not met, deprecated vars in use, conflicting providers.
func Validate() []ValidationIssue {
	current := GetAllCurrent()
	return validateVars(current)
}

// validateVars is the internal implementation, accepting an explicit map for testability.
func validateVars(vars map[string]string) []ValidationIssue {
	var issues []ValidationIssue

	for name := range vars {
		v, ok := Get(name)
		if !ok {
			continue
		}

		// Check deprecated vars
		if v.Deprecated {
			msg := fmt.Sprintf("%s is deprecated", name)
			if v.ReplacedBy != "" {
				msg += fmt.Sprintf("; use %s instead", v.ReplacedBy)
			}
			issues = append(issues, ValidationIssue{
				Var:     name,
				Level:   "warning",
				Message: msg,
			})
		}

		// Check requires
		for _, req := range v.Requires {
			if _, set := vars[req]; !set {
				issues = append(issues, ValidationIssue{
					Var:     name,
					Level:   "warning",
					Message: fmt.Sprintf("%s requires %s to be set", name, req),
				})
			}
		}
	}

	// Check conflicting cloud providers
	providers := []string{"CLAUDE_CODE_USE_BEDROCK", "CLAUDE_CODE_USE_VERTEX", "CLAUDE_CODE_USE_FOUNDRY"}
	var activeProviders []string
	for _, p := range providers {
		if _, ok := vars[p]; ok {
			activeProviders = append(activeProviders, p)
		}
	}
	if len(activeProviders) > 1 {
		issues = append(issues, ValidationIssue{
			Var:     activeProviders[0],
			Level:   "error",
			Message: fmt.Sprintf("conflicting cloud providers set: %s", strings.Join(activeProviders, ", ")),
		})
	}

	return issues
}

// GenerateEnvBlock generates an `export KEY=VALUE` block from the given vars map.
// Keys are sorted for deterministic output.
func GenerateEnvBlock(vars map[string]string) string {
	keys := make([]string, 0, len(vars))
	for k := range vars {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var sb strings.Builder
	for _, k := range keys {
		fmt.Fprintf(&sb, "export %s=%s\n", k, vars[k])
	}
	return sb.String()
}

// ResolveEffective merges settings env with the current OS environment.
// OS environment takes precedence over settings values.
func ResolveEffective(settings map[string]string) map[string]string {
	result := make(map[string]string, len(settings))
	for k, v := range settings {
		result[k] = v
	}
	// OS env wins over settings
	for _, v := range registry {
		if val, ok := os.LookupEnv(v.Name); ok {
			result[v.Name] = val
		}
	}
	return result
}
