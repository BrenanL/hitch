package settings

import (
	"encoding/json"
)

// EffectiveSettings holds the merged result of all scopes.
type EffectiveSettings struct {
	Settings        // embedded merged settings
	Sources map[string]Scope // which scope each key came from
}

// mergeArrayKeys is the set of top-level keys whose arrays are concatenated
// across scopes rather than replaced.
var mergeArrayKeys = map[string]bool{
	"allowedHttpHookUrls":        true,
	"httpHookAllowedEnvVars":     true,
}

// Compute merges settings from all scopes. The input slice is ordered
// lowest-to-highest precedence (index 0 = User, index 1 = Project,
// index 2 = Local, index 3 = Managed). For non-merge keys the last
// (highest-precedence) scope that has the key wins. For hooks and env
// maps all scopes are merged. For array merge-type keys the arrays are
// concatenated and deduplicated (first occurrence preserved).
func Compute(all []*Settings) *EffectiveSettings {
	result := &EffectiveSettings{
		Settings: Settings{
			Hooks: make(map[string][]MatcherGroup),
			Env:   make(map[string]string),
			raw:   make(map[string]json.RawMessage),
		},
		Sources: make(map[string]Scope),
	}

	scopes := []Scope{ScopeUser, ScopeProject, ScopeLocal, ScopeManaged}

	// Pass 1: scalar and array-merge keys — iterate lowest to highest precedence
	// so that higher-precedence scopes overwrite lower ones.
	for i, s := range all {
		if s == nil {
			continue
		}
		scope := scopes[i]
		for key, val := range s.raw {
			if key == "hooks" || key == "env" {
				continue
			}
			if mergeArrayKeys[key] {
				// Array merge: accumulate across all scopes; deduplicate later.
				continue
			}
			// Scalar override: higher scope wins (last write wins since we go low→high).
			result.raw[key] = val
			result.Sources[key] = scope
		}
	}

	// Pass 2: array-merge keys — concatenate from all scopes, deduplicate.
	for key := range mergeArrayKeys {
		var combined []json.RawMessage
		seen := make(map[string]bool)
		for i, s := range all {
			if s == nil {
				continue
			}
			if val, ok := s.raw[key]; ok {
				var arr []json.RawMessage
				if err := json.Unmarshal(val, &arr); err == nil {
					for _, item := range arr {
						str := string(item)
						if !seen[str] {
							seen[str] = true
							combined = append(combined, item)
						}
					}
				}
				// Track the highest scope that contributed to this key.
				result.Sources[key] = scopes[i]
			}
		}
		if len(combined) > 0 {
			data, err := json.Marshal(combined)
			if err == nil {
				result.raw[key] = data
			}
		}
	}

	// Pass 3: env map merge — lower scopes first, higher scopes overwrite conflicts.
	for i, s := range all {
		if s == nil {
			continue
		}
		scope := scopes[i]
		for k, v := range s.Env {
			result.Env[k] = v
			result.Sources["env."+k] = scope
		}
	}
	if len(result.Env) > 0 {
		data, _ := json.Marshal(result.Env)
		result.raw["env"] = data
		result.Sources["env"] = scopes[len(all)-1] // highest scope that had env
		// Correct Sources["env"] to reflect the actual highest scope that had env.
		for i := len(all) - 1; i >= 0; i-- {
			if all[i] != nil && len(all[i].Env) > 0 {
				result.Sources["env"] = scopes[i]
				break
			}
		}
	}

	// Pass 4: hooks merge — collect per-scope hook maps then merge.
	scopeHooks := make([]map[string][]MatcherGroup, len(all))
	for i, s := range all {
		if s == nil {
			scopeHooks[i] = make(map[string][]MatcherGroup)
		} else {
			scopeHooks[i] = s.Hooks
		}
	}
	result.Hooks = MergeHooks(scopeHooks[3], scopeHooks[2], scopeHooks[1], scopeHooks[0])

	if len(result.Hooks) > 0 {
		data, _ := json.Marshal(result.Hooks)
		result.raw["hooks"] = data
		result.Sources["hooks"] = ScopeManaged // merged from all; attribute to highest present
		for i := len(all) - 1; i >= 0; i-- {
			if all[i] != nil && len(all[i].Hooks) > 0 {
				result.Sources["hooks"] = scopes[i]
				break
			}
		}
	}

	return result
}

// GetEffective returns the effective value for a key and which scope it came from.
// Returns (nil, ScopeUser, false) if the key is unset in all scopes.
func (e *EffectiveSettings) GetEffective(key string) (json.RawMessage, Scope, bool) {
	val, ok := e.raw[key]
	if !ok {
		return nil, ScopeUser, false
	}
	scope, _ := e.Sources[key]
	return val, scope, true
}

// GetEnv returns an effective env var value and the scope it came from.
// Returns ("", ScopeUser, false) if the variable is unset.
func (e *EffectiveSettings) GetEnv(key string) (string, Scope, bool) {
	val, ok := e.Env[key]
	if !ok {
		return "", ScopeUser, false
	}
	scope, _ := e.Sources["env."+key]
	return val, scope, true
}
