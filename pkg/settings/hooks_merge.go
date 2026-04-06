package settings

// MergeHooks merges hook arrays from all scopes. All hooks from all scopes
// are concatenated (not replaced). Within a scope, hooks run in definition
// order. Managed hooks cannot be overridden by lower scopes.
//
// The parameter order is managed, local, project, user — highest-precedence
// first. Managed hooks are placed first so they always run before any
// lower-scope hooks for the same event.
func MergeHooks(managed, local, project, user map[string][]MatcherGroup) map[string][]MatcherGroup {
	result := make(map[string][]MatcherGroup)

	// Process in order: managed first, then local, project, user.
	// For each event, managed hooks precede others.
	sources := []map[string][]MatcherGroup{managed, local, project, user}
	for _, scopeHooks := range sources {
		for event, groups := range scopeHooks {
			for _, group := range groups {
				addMatcherGroup(result, event, group)
			}
		}
	}

	pruneEmptyGroups(result)
	return result
}

// addMatcherGroup merges a MatcherGroup into result for the given event.
// If a group with the same matcher already exists, hooks are appended.
// If not, a new matcher group is created.
func addMatcherGroup(result map[string][]MatcherGroup, event string, group MatcherGroup) {
	if len(group.Hooks) == 0 {
		return
	}
	groups := result[event]
	for i, existing := range groups {
		if existing.Matcher == group.Matcher {
			groups[i].Hooks = append(groups[i].Hooks, group.Hooks...)
			result[event] = groups
			return
		}
	}
	result[event] = append(groups, MatcherGroup{
		Matcher: group.Matcher,
		Hooks:   append([]HookEntry(nil), group.Hooks...),
	})
}

// pruneEmptyGroups removes empty matcher groups and empty event entries.
func pruneEmptyGroups(hooks map[string][]MatcherGroup) {
	for event, groups := range hooks {
		var nonEmpty []MatcherGroup
		for _, g := range groups {
			if len(g.Hooks) > 0 {
				nonEmpty = append(nonEmpty, g)
			}
		}
		if len(nonEmpty) == 0 {
			delete(hooks, event)
		} else {
			hooks[event] = nonEmpty
		}
	}
}
