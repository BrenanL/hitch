# pkg/settings

Claude Code `settings.json` management: load, parse, merge, validate, and write settings files across all four configuration scopes.

## Package Purpose

Claude Code reads settings from up to four JSON files, each at a different scope. This package provides:

- Typed loading and parsing of each scope's file
- A deterministic merge algorithm that computes the effective configuration
- A schema registry describing every known settings key, its type, default, and constraints
- Validation that flags misplaced or invalid keys before writing
- Atomic writes (write-to-tmp then rename) to prevent partial updates
- Round-trip fidelity: unknown keys in existing files are preserved

## Scopes

| Constant | File | Precedence |
|---|---|---|
| `ScopeUser` | `~/.claude/settings.json` | Lowest |
| `ScopeProject` | `{projectDir}/.claude/settings.json` | Second |
| `ScopeLocal` | `{projectDir}/.claude/settings.local.json` | Third |
| `ScopeManaged` | `{projectDir}/.claude/settings.managed.json` | Highest |

Managed scope is intended for policy/IT-controlled settings. Keys marked `ManagedOnly` in the schema are only respected when they appear in the managed scope; the same key at a lower scope is a validation error.

## API Overview

### Loading

```go
// Load a single scope (returns empty Settings if file missing)
s, err := settings.LoadScope(settings.ScopeProject, "/path/to/project")

// Load all four scopes in order (User, Project, Local, Managed)
all, err := settings.LoadAll("/path/to/project")
```

### Parsing

```go
// Parse raw JSON bytes into a Settings struct
s, err := settings.ParseSettings(data)
```

`ParseSettings` preserves all unknown top-level keys in an internal raw map so they survive a round-trip through `MarshalSettings`.

### Computing the Effective Configuration

```go
// Merge all four scopes into a single EffectiveSettings
effective := settings.Compute(all)

// Look up a key and which scope it came from
raw, scope, ok := effective.GetEffective("model")

// Look up an env var
val, scope, ok := effective.GetEnv("ANTHROPIC_BASE_URL")
```

### Writing

```go
// Atomically write settings to a scope
err := settings.Write(s, settings.ScopeProject, "/path/to/project")
```

`Write` creates the `.claude/` directory if needed, serializes with `MarshalSettings`, and performs an atomic rename so concurrent readers never see a partial file.

### Validation

```go
// Validate a parsed Settings against its scope
issues := settings.Validate(s, settings.ScopeProject)
for _, issue := range issues {
    fmt.Printf("[%s] %s: %s\n", issue.Level, issue.Key, issue.Message)
}
```

Checks performed:
- Managed-only keys at project/local/user scope → error
- Global config keys placed in `settings.json` → warning (they belong in `~/.claude/config.json`)
- Invalid enum values → error
- `permissions.defaultMode` not in the accepted set → error
- `cleanupPeriodDays` ≤ 0 → error
- `feedbackSurveyRate` outside `[0.0, 1.0]` → error

### Schema

```go
// All known key definitions
defs := settings.Schema()

// Convenience lists
managedOnly := settings.ManagedOnlyKeys()
globalConfig := settings.GlobalConfigKeys()
```

`KeyDef` fields: `Name`, `Type` (`KeyTypeString`, `KeyTypeBool`, `KeyTypeInt`, `KeyTypeFloat`, `KeyTypeEnum`, `KeyTypeArray`, `KeyTypeMap`, `KeyTypeObject`), `Description`, `Default`, `EnumValues`, `ManagedOnly`, `GlobalConfig`.

## Merge Rules

`Compute` applies four passes over the slice `[User, Project, Local, Managed]`:

| Key type | Rule |
|---|---|
| Scalar / object | Last (highest-precedence) scope that sets the key wins |
| `env` map | All scopes merged; higher scope overwrites conflicts per-key |
| `hooks` map | All scopes concatenated; managed hooks placed first, ensuring they run before lower-scope hooks for the same event |
| Array-merge keys (`allowedHttpHookUrls`, `httpHookAllowedEnvVars`) | Concatenated across all scopes and deduplicated (first occurrence preserved) |

Hook merging preserves matcher grouping: if two scopes define hooks for the same event and matcher, the hooks are appended to the same `MatcherGroup` rather than creating a duplicate group.

## Global Config Support

Settings that belong in `~/.claude/config.json` (theme, IDE integration, editor mode, etc.) are tracked separately in `GlobalConfig`. Use `LoadGlobalConfig` and `WriteGlobalConfig` to read and write that file:

```go
gc, err := settings.LoadGlobalConfig()
gc.Theme = "dark"
err = settings.WriteGlobalConfig(gc)
```

Keys marked `GlobalConfig: true` in the schema are flagged by `Validate` if they appear in a `settings.json` file.

## Baseline

`DefaultBaseline` returns Hitch's recommended out-of-the-box settings map (effort level, telemetry flags, proxy URL). `LoadHitchDefaults` reads overrides from `~/.hitch/config.toml`.

```go
baseline := settings.DefaultBaseline("http://localhost:9800")
hitchDefaults, err := settings.LoadHitchDefaults("")
```

## Low-Level Helpers

```go
settings.SetKey(s, "model", "claude-opus-4-5")  // set any raw top-level key
settings.DeleteKey(s, "model")
settings.GetRaw(s, "model")                      // returns json.RawMessage

settings.SetEnv(s, "ANTHROPIC_BASE_URL", "http://localhost:9800")
settings.DeleteEnv(s, "ANTHROPIC_BASE_URL")
```
