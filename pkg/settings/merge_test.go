package settings

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// makeSettings is a helper that parses JSON into a *Settings.
func makeSettings(t *testing.T, raw string) *Settings {
	t.Helper()
	s, err := ParseSettings([]byte(raw))
	if err != nil {
		t.Fatalf("ParseSettings(%q): %v", raw, err)
	}
	return s
}

// TestCompute_Precedence verifies that a managed key overrides a user key.
func TestCompute_Precedence(t *testing.T) {
	user := makeSettings(t, `{"effortLevel":"low"}`)
	project := makeSettings(t, `{}`)
	local := makeSettings(t, `{}`)
	managed := makeSettings(t, `{"effortLevel":"high"}`)

	es := Compute([]*Settings{user, project, local, managed})

	val, scope, ok := es.GetEffective("effortLevel")
	if !ok {
		t.Fatal("effortLevel not found in effective settings")
	}
	var level string
	if err := json.Unmarshal(val, &level); err != nil {
		t.Fatalf("unmarshal effortLevel: %v", err)
	}
	if level != "high" {
		t.Errorf("effortLevel = %q, want %q", level, "high")
	}
	if scope != ScopeManaged {
		t.Errorf("scope = %v, want managed", scope)
	}
}

// TestCompute_ArrayMerge verifies that arrays from different scopes are concatenated.
func TestCompute_ArrayMerge(t *testing.T) {
	user := makeSettings(t, `{"allowedHttpHookUrls":["http://user.example.com"]}`)
	project := makeSettings(t, `{"allowedHttpHookUrls":["http://project.example.com"]}`)
	local := makeSettings(t, `{}`)
	managed := makeSettings(t, `{}`)

	es := Compute([]*Settings{user, project, local, managed})

	val, _, ok := es.GetEffective("allowedHttpHookUrls")
	if !ok {
		t.Fatal("allowedHttpHookUrls not found in effective settings")
	}
	var urls []string
	if err := json.Unmarshal(val, &urls); err != nil {
		t.Fatalf("unmarshal allowedHttpHookUrls: %v", err)
	}
	if len(urls) != 2 {
		t.Fatalf("len(urls) = %d, want 2; urls = %v", len(urls), urls)
	}
	// Both user and project entries should be present.
	found := map[string]bool{}
	for _, u := range urls {
		found[u] = true
	}
	if !found["http://user.example.com"] {
		t.Error("user URL not found in merged result")
	}
	if !found["http://project.example.com"] {
		t.Error("project URL not found in merged result")
	}
}

// TestCompute_EnvMapMerge verifies that env maps are merged and higher scope wins on conflict.
func TestCompute_EnvMapMerge(t *testing.T) {
	user := makeSettings(t, `{"env":{"KEY":"user-value","USER_ONLY":"from-user"}}`)
	project := makeSettings(t, `{"env":{"KEY":"project-value","PROJECT_ONLY":"from-project"}}`)
	local := makeSettings(t, `{}`)
	managed := makeSettings(t, `{"env":{"KEY":"managed-value"}}`)

	es := Compute([]*Settings{user, project, local, managed})

	// Managed wins on KEY.
	val, scope, ok := es.GetEnv("KEY")
	if !ok {
		t.Fatal("KEY not found in effective env")
	}
	if val != "managed-value" {
		t.Errorf("KEY = %q, want %q", val, "managed-value")
	}
	if scope != ScopeManaged {
		t.Errorf("KEY scope = %v, want managed", scope)
	}

	// USER_ONLY should survive.
	userOnly, _, ok := es.GetEnv("USER_ONLY")
	if !ok {
		t.Fatal("USER_ONLY not found in effective env")
	}
	if userOnly != "from-user" {
		t.Errorf("USER_ONLY = %q, want %q", userOnly, "from-user")
	}

	// PROJECT_ONLY should survive.
	projectOnly, _, ok := es.GetEnv("PROJECT_ONLY")
	if !ok {
		t.Fatal("PROJECT_ONLY not found in effective env")
	}
	if projectOnly != "from-project" {
		t.Errorf("PROJECT_ONLY = %q, want %q", projectOnly, "from-project")
	}
}

// TestCompute_HooksMerge verifies that hooks from all scopes are present in the result.
func TestCompute_HooksMerge(t *testing.T) {
	user := makeSettings(t, `{"hooks":{"PreToolUse":[{"matcher":"Bash","hooks":[{"type":"command","command":"user-hook"}]}]}}`)
	project := makeSettings(t, `{"hooks":{"PreToolUse":[{"matcher":"Bash","hooks":[{"type":"command","command":"project-hook"}]}]}}`)
	local := makeSettings(t, `{}`)
	managed := makeSettings(t, `{"hooks":{"PostToolUse":[{"matcher":"Bash","hooks":[{"type":"command","command":"managed-hook"}]}]}}`)

	es := Compute([]*Settings{user, project, local, managed})

	// PreToolUse should have hooks from both user and project scopes.
	preGroups, ok := es.Hooks["PreToolUse"]
	if !ok {
		t.Fatal("PreToolUse hooks not found")
	}
	var allCmds []string
	for _, g := range preGroups {
		for _, h := range g.Hooks {
			allCmds = append(allCmds, h.Command)
		}
	}
	foundUser, foundProject := false, false
	for _, cmd := range allCmds {
		if cmd == "user-hook" {
			foundUser = true
		}
		if cmd == "project-hook" {
			foundProject = true
		}
	}
	if !foundUser {
		t.Error("user-hook not found in merged PreToolUse hooks")
	}
	if !foundProject {
		t.Error("project-hook not found in merged PreToolUse hooks")
	}

	// PostToolUse should have the managed hook.
	postGroups, ok := es.Hooks["PostToolUse"]
	if !ok {
		t.Fatal("PostToolUse hooks not found")
	}
	if len(postGroups) == 0 || len(postGroups[0].Hooks) == 0 {
		t.Fatal("PostToolUse hooks empty")
	}
	if postGroups[0].Hooks[0].Command != "managed-hook" {
		t.Errorf("PostToolUse command = %q, want %q", postGroups[0].Hooks[0].Command, "managed-hook")
	}
}

// TestCompute_ManagedWinsAll verifies that managed scope always overrides all other scopes.
func TestCompute_ManagedWinsAll(t *testing.T) {
	user := makeSettings(t, `{"model":"user-model","effortLevel":"low"}`)
	project := makeSettings(t, `{"model":"project-model","effortLevel":"medium"}`)
	local := makeSettings(t, `{"model":"local-model","effortLevel":"high"}`)
	managed := makeSettings(t, `{"model":"managed-model","effortLevel":"high"}`)

	es := Compute([]*Settings{user, project, local, managed})

	for _, key := range []string{"model", "effortLevel"} {
		_, scope, ok := es.GetEffective(key)
		if !ok {
			t.Fatalf("%s not found", key)
		}
		if scope != ScopeManaged {
			t.Errorf("%s scope = %v, want managed", key, scope)
		}
	}

	val, _, _ := es.GetEffective("model")
	var model string
	if err := json.Unmarshal(val, &model); err != nil {
		t.Fatalf("unmarshal model: %v", err)
	}
	if model != "managed-model" {
		t.Errorf("model = %q, want %q", model, "managed-model")
	}
}

// TestLoadAll_AllScopesPresent verifies that LoadAll loads all 4 scope files.
func TestLoadAll_AllScopesPresent(t *testing.T) {
	dir := t.TempDir()
	claudeDir := filepath.Join(dir, ".claude")
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	files := map[string]string{
		filepath.Join(claudeDir, "settings.json"):         `{"model":"project-model"}`,
		filepath.Join(claudeDir, "settings.local.json"):   `{"model":"local-model"}`,
		filepath.Join(claudeDir, "settings.managed.json"): `{"model":"managed-model"}`,
	}
	for path, content := range files {
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("writing %s: %v", path, err)
		}
	}

	all, err := LoadAll(dir)
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}
	if len(all) != 4 {
		t.Fatalf("len(all) = %d, want 4", len(all))
	}
	// Index 1 = Project, 2 = Local, 3 = Managed
	checkModel := func(idx int, want string) {
		t.Helper()
		raw, ok := GetRaw(all[idx], "model")
		if !ok {
			t.Errorf("all[%d] has no model key", idx)
			return
		}
		var m string
		if err := json.Unmarshal(raw, &m); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if m != want {
			t.Errorf("all[%d].model = %q, want %q", idx, m, want)
		}
	}
	checkModel(1, "project-model")
	checkModel(2, "local-model")
	checkModel(3, "managed-model")
}

// TestLoadAll_MissingFilesSkipped verifies that missing scope files don't cause errors.
// The user scope (index 0) reads from ~/.claude/settings.json which may exist on the
// host machine, so we only verify that project/local/managed scopes are empty when
// their respective files are absent.
func TestLoadAll_MissingFilesSkipped(t *testing.T) {
	dir := t.TempDir()
	// No .claude directory at all — project/local/managed scopes are missing.
	all, err := LoadAll(dir)
	if err != nil {
		t.Fatalf("LoadAll with no files: %v", err)
	}
	if len(all) != 4 {
		t.Fatalf("len(all) = %d, want 4", len(all))
	}
	// Indices 1=Project, 2=Local, 3=Managed should be empty (no files present).
	for _, i := range []int{1, 2, 3} {
		s := all[i]
		if s == nil {
			t.Errorf("all[%d] is nil, want empty Settings", i)
			continue
		}
		if len(s.Hooks) != 0 {
			t.Errorf("all[%d].Hooks not empty", i)
		}
		if len(s.Env) != 0 {
			t.Errorf("all[%d].Env not empty", i)
		}
	}
}

// TestMergeHooks_AddsNewEntries verifies that hooks from different events merge.
func TestMergeHooks_AddsNewEntries(t *testing.T) {
	managed := map[string][]MatcherGroup{
		"PreToolUse": {{Matcher: "Bash", Hooks: []HookEntry{{Type: "command", Command: "managed-pre"}}}},
	}
	local := map[string][]MatcherGroup{}
	project := map[string][]MatcherGroup{
		"PostToolUse": {{Matcher: "Bash", Hooks: []HookEntry{{Type: "command", Command: "project-post"}}}},
	}
	user := map[string][]MatcherGroup{
		"Notification": {{Matcher: "", Hooks: []HookEntry{{Type: "command", Command: "user-notify"}}}},
	}

	result := MergeHooks(managed, local, project, user)

	if _, ok := result["PreToolUse"]; !ok {
		t.Error("PreToolUse missing from merged hooks")
	}
	if _, ok := result["PostToolUse"]; !ok {
		t.Error("PostToolUse missing from merged hooks")
	}
	if _, ok := result["Notification"]; !ok {
		t.Error("Notification missing from merged hooks")
	}
}

// TestMergeHooks_PreservesUnowned verifies that non-hitch hooks from all scopes are preserved.
func TestMergeHooks_PreservesUnowned(t *testing.T) {
	managed := map[string][]MatcherGroup{}
	local := map[string][]MatcherGroup{}
	project := map[string][]MatcherGroup{
		"PreToolUse": {{Matcher: "Bash", Hooks: []HookEntry{{Type: "command", Command: "non-hitch-hook"}}}},
	}
	user := map[string][]MatcherGroup{
		"PreToolUse": {{Matcher: "Bash", Hooks: []HookEntry{{Type: "command", Command: "another-hook"}}}},
	}

	result := MergeHooks(managed, local, project, user)

	groups, ok := result["PreToolUse"]
	if !ok {
		t.Fatal("PreToolUse not found")
	}
	var cmds []string
	for _, g := range groups {
		for _, h := range g.Hooks {
			cmds = append(cmds, h.Command)
		}
	}
	found := map[string]bool{}
	for _, c := range cmds {
		found[c] = true
	}
	if !found["non-hitch-hook"] {
		t.Error("non-hitch-hook not preserved")
	}
	if !found["another-hook"] {
		t.Error("another-hook not preserved")
	}
}

// TestMergeHooks_PrunesEmpty verifies that empty hook arrays are cleaned up.
func TestMergeHooks_PrunesEmpty(t *testing.T) {
	managed := map[string][]MatcherGroup{
		"PreToolUse": {{Matcher: "Bash", Hooks: []HookEntry{}}},
	}
	local := map[string][]MatcherGroup{}
	project := map[string][]MatcherGroup{}
	user := map[string][]MatcherGroup{}

	result := MergeHooks(managed, local, project, user)

	if _, ok := result["PreToolUse"]; ok {
		t.Error("PreToolUse with empty hooks should have been pruned")
	}
}
