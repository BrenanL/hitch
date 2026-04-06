package settings

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ValidationIssue describes a settings validation problem.
type ValidationIssue struct {
	Key     string
	Level   string // "error" or "warning"
	Message string
}

// validDefaultModes lists accepted values for permissions.defaultMode.
var validDefaultModes = []string{
	"default",
	"acceptEdits",
	"plan",
	"auto",
	"dontAsk",
	"bypassPermissions",
}

// managedOnlySet is a fast lookup set for managed-only keys.
var managedOnlySet map[string]bool

// globalConfigSet is a fast lookup set for global-config keys.
var globalConfigSet map[string]bool

// enumValues maps key names to their accepted enum values.
var enumValues map[string][]string

func init() {
	managedOnlySet = make(map[string]bool)
	globalConfigSet = make(map[string]bool)
	enumValues = make(map[string][]string)

	for _, def := range Schema() {
		if def.ManagedOnly {
			managedOnlySet[def.Name] = true
		}
		if def.GlobalConfig {
			globalConfigSet[def.Name] = true
		}
		if def.Type == KeyTypeEnum && len(def.EnumValues) > 0 {
			enumValues[def.Name] = def.EnumValues
		}
	}
}

// Validate checks settings for schema violations given the scope they came from.
// Checks performed:
//   - Managed-only keys in project/local/user scope → error
//   - Global config keys in settings.json → warning
//   - Invalid enum values → error
//   - Invalid permissions.defaultMode → error
//   - cleanupPeriodDays zero or negative → error
//   - feedbackSurveyRate outside 0.0-1.0 → error
func Validate(s *Settings, scope Scope) []ValidationIssue {
	var issues []ValidationIssue

	for key := range s.raw {
		// Check managed-only keys in non-managed scope.
		if managedOnlySet[key] && scope != ScopeManaged {
			issues = append(issues, ValidationIssue{
				Key:     key,
				Level:   "error",
				Message: fmt.Sprintf("key %q is managed-only and has no effect at %s scope", key, scope),
			})
		}

		// Check global-config keys placed in settings.json.
		if globalConfigSet[key] {
			issues = append(issues, ValidationIssue{
				Key:     key,
				Level:   "warning",
				Message: fmt.Sprintf("key %q belongs in ~/.claude/config.json (global config), not settings.json", key),
			})
		}

		// Check enum values.
		if accepted, ok := enumValues[key]; ok {
			raw := s.raw[key]
			var val string
			if err := json.Unmarshal(raw, &val); err == nil {
				if !stringInSlice(val, accepted) {
					issues = append(issues, ValidationIssue{
						Key:   key,
						Level: "error",
						Message: fmt.Sprintf("key %q has invalid value %q; accepted values: %s",
							key, val, strings.Join(accepted, ", ")),
					})
				}
			}
		}
	}

	// Check permissions.defaultMode if permissions object is present.
	if permRaw, ok := s.raw["permissions"]; ok {
		var perms map[string]json.RawMessage
		if err := json.Unmarshal(permRaw, &perms); err == nil {
			if modeRaw, ok := perms["defaultMode"]; ok {
				var mode string
				if err := json.Unmarshal(modeRaw, &mode); err == nil {
					if !stringInSlice(mode, validDefaultModes) {
						issues = append(issues, ValidationIssue{
							Key:   "permissions.defaultMode",
							Level: "error",
							Message: fmt.Sprintf("permissions.defaultMode has invalid value %q; accepted values: %s",
								mode, strings.Join(validDefaultModes, ", ")),
						})
					}
				}
			}
		}
	}

	// Check cleanupPeriodDays.
	if raw, ok := s.raw["cleanupPeriodDays"]; ok {
		var days int
		if err := json.Unmarshal(raw, &days); err == nil {
			if days <= 0 {
				issues = append(issues, ValidationIssue{
					Key:     "cleanupPeriodDays",
					Level:   "error",
					Message: fmt.Sprintf("cleanupPeriodDays must be >= 1, got %d", days),
				})
			}
		}
	}

	// Check feedbackSurveyRate.
	if raw, ok := s.raw["feedbackSurveyRate"]; ok {
		var rate float64
		if err := json.Unmarshal(raw, &rate); err == nil {
			if rate < 0.0 || rate > 1.0 {
				issues = append(issues, ValidationIssue{
					Key:     "feedbackSurveyRate",
					Level:   "error",
					Message: fmt.Sprintf("feedbackSurveyRate must be between 0.0 and 1.0, got %g", rate),
				})
			}
		}
	}

	return issues
}

func stringInSlice(s string, list []string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
