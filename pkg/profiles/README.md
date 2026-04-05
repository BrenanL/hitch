# pkg/profiles

Profile management for Claude Code settings. Profiles are named collections of env vars, settings keys, and hooks that can be applied to and removed from a project's `settings.local.json`.

## Package Purpose

Profiles let users switch between pre-defined or custom Claude Code configurations without manually editing settings files. A profile captures a named set of:

- **Env vars** (`env` block) — applied to `settings.local.json`
- **Env deletions** (`env_deletes` list) — keys to remove on apply
- **Settings keys** (`settings` block) — arbitrary top-level keys in `settings.local.json`
- **Hooks** (`hooks` block) — hook configuration (stored in the profile record but not applied to settings.local.json directly)

When a profile is applied, hitch tracks every key it wrote. On reset, only tracked keys are removed — other keys in `settings.local.json` are untouched.

## Built-in Profiles

Five profiles are embedded in the binary:

| Name | Description |
|---|---|
| `default` | Balanced settings — the daily driver. `effortLevel=medium`. |
| `conservative` | Max safety — deny dangerous commands, strict permissions. Includes a `PreToolUse` Bash hook. |
| `autonomous` | Minimal friction — auto-approve safe operations. `effortLevel=high`. |
| `research` | Optimized for code exploration — verbose output, no auto-edits. `effortLevel=high`, `MAX_THINKING_TOKENS=16000`, thinking enabled. |
| `minimal` | Stripped down — no hooks, minimal settings. |

All built-in profiles carry the `"builtin"` tag.

## Profile Struct

```go
type Profile struct {
    Name        string            `json:"name"`
    Description string            `json:"description"`
    Settings    map[string]any    `json:"settings,omitempty"`
    Env         map[string]string `json:"env,omitempty"`
    EnvDeletes  []string          `json:"env_deletes,omitempty"`
    Hooks       map[string]any    `json:"hooks,omitempty"`
    Tags        []string          `json:"tags,omitempty"`
    Extends     string            `json:"extends,omitempty"`
}
```

`Name` and `Description` are required. `Validate` enforces this, plus rejects empty env keys/values and self-referential `Extends`.

## Loading Profiles

```go
// Load all profiles (builtins + user profiles).
profiles, err := profiles.LoadAll()

// Load a single profile by name.
p, err := profiles.Load("research")

// Validate a profile struct.
err := profiles.Validate(p)
```

`LoadAll` returns built-ins first (in their embedded order), followed by any user-only profiles. User profiles with the same name as a built-in shadow the built-in.

## Apply / Reset / Switch

```go
// Apply a profile to a project directory.
// Returns the list of tracked keys for rollback.
written, err := profiles.ApplyProfile(p, projectDir)

// Reset: remove all keys written by the profile.
err := profiles.ResetProfile(written, projectDir)

// Query the currently active profile name (empty string if none).
name, err := profiles.CurrentProfile(projectDir)
```

`ApplyProfile` writes to `{projectDir}/.claude/settings.local.json` and records the active profile in `{projectDir}/.hitch/active-profile.json`. Both directories are created automatically if they don't exist.

Tracked key format:

| Prefix | Meaning |
|---|---|
| `env:KEY` | env var written |
| `env_delete:KEY` | env var deleted on apply |
| `settings:KEY` | settings key written |
| `settings_delete:KEY` | settings key deleted on apply |

`ResetProfile` only removes `env:` and `settings:` entries — deletion operations (`env_delete:`, `settings_delete:`) are not reversed on reset.

### Switch pattern

To switch from one profile to another:

```go
written, _ := profiles.ApplyProfile(oldProfile, dir)
profiles.ResetProfile(written, dir)
profiles.ApplyProfile(newProfile, dir)
```

## User Profiles Directory

User profiles live at `~/.hitch/profiles/` as `.json` files. Any `.json` file in that directory is loaded as a profile. A user profile whose name matches a built-in name shadows the built-in in both `LoadAll` and `Load`.

Example user profile at `~/.hitch/profiles/myteam.json`:

```json
{
  "name": "myteam",
  "description": "Team configuration",
  "env": {
    "ANTHROPIC_MODEL": "claude-opus-4-5"
  },
  "tags": ["team"]
}
```
