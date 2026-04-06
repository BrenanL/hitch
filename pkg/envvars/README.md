# pkg/envvars

Claude Code environment variable registry: 148 variables across 30 categories, with validation, dependency checking, and OS environment resolution.

## Package Purpose

Claude Code's behavior is governed by a large set of environment variables spanning API authentication, model selection, cloud providers, tooling, and UI preferences. This package provides:

- A typed registry of all 148 known variables, organized into 30 categories
- Lookup by name or category
- Validation of the current OS environment (deprecated vars, missing dependencies, conflicting providers)
- Resolution that merges settings-file env blocks with the live OS environment
- A utility to generate `export KEY=VALUE` shell blocks from a variable map

## Types

```go
type EnvVar struct {
    Name        string
    Category    string
    Description string
    Default     string   // empty if no default
    Deprecated  bool
    ReplacedBy  string   // populated when Deprecated is true
    Requires    []string // other vars this one depends on
    Conflicts   []string // vars that conflict with this one
}

type ValidationIssue struct {
    Var     string
    Level   string // "warning" or "error"
    Message string
}
```

## Registry Functions

```go
// All returns a copy of all 148 registered variables.
vars := envvars.All()

// Get looks up a variable by exact name.
v, ok := envvars.Get("ANTHROPIC_BASE_URL")

// GetByCategory returns all variables in a category.
authVars := envvars.GetByCategory("API Authentication")

// Categories returns a sorted list of all 30 category names.
cats := envvars.Categories()
```

## Categories

The 30 categories, sorted alphabetically:

| Category | Representative variables |
|---|---|
| API Authentication | `ANTHROPIC_API_KEY`, `ANTHROPIC_AUTH_TOKEN` |
| API Configuration | `ANTHROPIC_BASE_URL`, `ANTHROPIC_BETAS`, `ANTHROPIC_CUSTOM_HEADERS` |
| API Key Helper | `CLAUDE_CODE_API_KEY_HELPER_TTL_MS` |
| Advanced Features | `CLAUDE_CODE_DISABLE_FAST_MODE`, `CLAUDE_CODE_SIMPLE`, `CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS` |
| Authentication & OAuth | `CLAUDE_CODE_OAUTH_TOKEN`, `CLAUDE_CODE_OAUTH_REFRESH_TOKEN` |
| Bash & Shell | `BASH_DEFAULT_TIMEOUT_MS`, `CLAUDE_CODE_SHELL`, `CLAUDECODE` |
| Checkpointing | `CLAUDE_CODE_DISABLE_FILE_CHECKPOINTING` |
| Cloud Provider (Bedrock) | `CLAUDE_CODE_USE_BEDROCK`, `AWS_BEARER_TOKEN_BEDROCK` |
| Cloud Provider (Foundry) | `CLAUDE_CODE_USE_FOUNDRY`, `ANTHROPIC_FOUNDRY_BASE_URL` |
| Cloud Provider (Vertex AI) | `CLAUDE_CODE_USE_VERTEX`, `ANTHROPIC_VERTEX_PROJECT_ID` |
| Command Visibility | `DISABLE_DOCTOR_COMMAND`, `DISABLE_COST_WARNINGS` |
| Debugging & Logging | `CLAUDE_CODE_DEBUG_LOG_LEVEL`, `CLAUDE_ENABLE_STREAM_WATCHDOG` |
| Environment & Hooks | `CLAUDE_ENV_FILE`, `CLAUDE_CODE_SESSIONEND_HOOKS_TIMEOUT_MS` |
| Fallback & Overload | `FALLBACK_FOR_ALL_PRIMARY_MODELS` |
| File & Directory | `CLAUDE_CONFIG_DIR`, `CLAUDE_CODE_TMPDIR` |
| Git & Workflows | `CLAUDE_CODE_DISABLE_GIT_INSTRUCTIONS` |
| IDE Integration | `CLAUDE_CODE_AUTO_CONNECT_IDE`, `CLAUDE_CODE_IDE_HOST_OVERRIDE` |
| Memory & Context | `DISABLE_AUTO_COMPACT`, `CLAUDE_AUTOCOMPACT_PCT_OVERRIDE` |
| Model Configuration | `ANTHROPIC_MODEL`, `ANTHROPIC_DEFAULT_SONNET_MODEL` |
| Network & Retry | `CLAUDE_CODE_MAX_RETRIES`, `CLAUDE_CODE_PROXY_RESOLVES_HOSTS` |
| OpenTelemetry | `CLAUDE_CODE_ENABLE_TELEMETRY`, `CLAUDE_CODE_OTEL_FLUSH_TIMEOUT_MS` |
| Plugin Management | `CLAUDE_CODE_PLUGIN_GIT_TIMEOUT_MS`, `CLAUDE_CODE_SYNC_PLUGIN_INSTALL` |
| Prompt Caching | `DISABLE_PROMPT_CACHING`, `DISABLE_PROMPT_CACHING_HAIKU` |
| Session & Remote Control | `CLAUDE_CODE_RESUME_INTERRUPTED_TURN`, `CLAUDE_CODE_TEAM_NAME` |
| Telemetry & Updates | `DISABLE_TELEMETRY`, `DISABLE_ERROR_REPORTING`, `DISABLE_AUTOUPDATER` |
| Thinking & Reasoning | `MAX_THINKING_TOKENS`, `CLAUDE_CODE_EFFORT_LEVEL` |
| Token & Output Limits | `CLAUDE_CODE_MAX_OUTPUT_TOKENS`, `API_TIMEOUT_MS` |
| Tools & Features | `CLAUDE_CODE_MAX_TOOL_USE_CONCURRENCY`, `CLAUDE_CODE_DISABLE_CRON` |
| UI & Display | `CLAUDE_CODE_NO_FLICKER`, `CLAUDE_CODE_DISABLE_TERMINAL_TITLE` |
| mTLS Authentication | `CLAUDE_CODE_CLIENT_CERT`, `CLAUDE_CODE_CLIENT_KEY` |

## Validation

`Validate` checks the current OS environment against the registry and returns a slice of issues:

```go
issues := envvars.Validate()
for _, issue := range issues {
    fmt.Printf("[%s] %s: %s\n", issue.Level, issue.Var, issue.Message)
}
```

Issue levels:

| Level | Trigger |
|---|---|
| `warning` | Variable is deprecated (use `ReplacedBy` instead) |
| `warning` | Variable is set but a variable it `Requires` is not set |
| `error` | Multiple conflicting cloud providers set simultaneously (`CLAUDE_CODE_USE_BEDROCK`, `CLAUDE_CODE_USE_VERTEX`, `CLAUDE_CODE_USE_FOUNDRY`) |

Example warning: setting `CLAUDE_CODE_OAUTH_REFRESH_TOKEN` without `CLAUDE_CODE_OAUTH_SCOPES` triggers a "requires" warning because the refresh token flow needs the scopes.

Example deprecation: `ANTHROPIC_SMALL_FAST_MODEL` is deprecated; use `ANTHROPIC_DEFAULT_HAIKU_MODEL` instead.

## Reading Current Values

```go
// Get the live OS value for a registered variable.
val, ok := envvars.GetCurrent("ANTHROPIC_BASE_URL")

// Get all registered variables that are currently set.
active := envvars.GetAllCurrent() // map[string]string
```

`GetCurrent` returns `("", false)` for both unset variables and unregistered names.

## Resolving Effective Values

`ResolveEffective` merges a settings-file `env` block with the live OS environment. OS values take precedence:

```go
// settingsEnv comes from settings.Settings.Env
settingsEnv := map[string]string{
    "ANTHROPIC_BASE_URL": "http://localhost:9800",
}
effective := envvars.ResolveEffective(settingsEnv)
// If ANTHROPIC_BASE_URL is set in the OS environment, the OS value wins.
```

This function only copies OS values for variables in the registry; arbitrary OS variables are not included.

## Generating Shell Exports

```go
block := envvars.GenerateEnvBlock(map[string]string{
    "DISABLE_TELEMETRY":    "1",
    "ANTHROPIC_BASE_URL":   "http://localhost:9800",
})
// Output (keys sorted):
// export ANTHROPIC_BASE_URL=http://localhost:9800
// export DISABLE_TELEMETRY=1
```
