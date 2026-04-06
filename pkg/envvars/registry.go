package envvars

import "sort"

var registry = []EnvVar{
	// API Authentication
	{
		Name:        "ANTHROPIC_API_KEY",
		Category:    "API Authentication",
		Description: "API key sent as X-Api-Key header. When set, overrides Claude Pro/Max/Team/Enterprise subscription.",
	},
	{
		Name:        "ANTHROPIC_AUTH_TOKEN",
		Category:    "API Authentication",
		Description: "Custom value for the Authorization header (prefixed with Bearer).",
	},

	// API Configuration
	{
		Name:        "ANTHROPIC_BASE_URL",
		Category:    "API Configuration",
		Description: "Override API endpoint to route through proxy or gateway.",
	},
	{
		Name:        "ANTHROPIC_BETAS",
		Category:    "API Configuration",
		Description: "Comma-separated list of additional anthropic-beta header values.",
	},
	{
		Name:        "ANTHROPIC_CUSTOM_HEADERS",
		Category:    "API Configuration",
		Description: "Custom headers to add to requests (Name: Value format, newline-separated for multiple).",
	},

	// Model Configuration
	{
		Name:        "ANTHROPIC_MODEL",
		Category:    "Model Configuration",
		Description: "Name of model setting to use.",
	},
	{
		Name:        "ANTHROPIC_CUSTOM_MODEL_OPTION",
		Category:    "Model Configuration",
		Description: "Model ID to add as custom entry in /model picker without replacing built-in aliases.",
	},
	{
		Name:        "ANTHROPIC_CUSTOM_MODEL_OPTION_NAME",
		Category:    "Model Configuration",
		Description: "Display name for custom model.",
		Default:     "Defaults to model ID",
	},
	{
		Name:        "ANTHROPIC_CUSTOM_MODEL_OPTION_DESCRIPTION",
		Category:    "Model Configuration",
		Description: "Display description for custom model.",
		Default:     "Custom model (<model-id>)",
	},
	{
		Name:        "ANTHROPIC_DEFAULT_HAIKU_MODEL",
		Category:    "Model Configuration",
		Description: "Default Haiku-class model.",
	},
	{
		Name:        "ANTHROPIC_DEFAULT_HAIKU_MODEL_NAME",
		Category:    "Model Configuration",
		Description: "Display name for Haiku model.",
	},
	{
		Name:        "ANTHROPIC_DEFAULT_HAIKU_MODEL_DESCRIPTION",
		Category:    "Model Configuration",
		Description: "Display description for Haiku model.",
	},
	{
		Name:        "ANTHROPIC_DEFAULT_HAIKU_MODEL_SUPPORTED_CAPABILITIES",
		Category:    "Model Configuration",
		Description: "Capabilities for Haiku model.",
	},
	{
		Name:        "ANTHROPIC_DEFAULT_SONNET_MODEL",
		Category:    "Model Configuration",
		Description: "Default Sonnet-class model.",
	},
	{
		Name:        "ANTHROPIC_DEFAULT_SONNET_MODEL_NAME",
		Category:    "Model Configuration",
		Description: "Display name for Sonnet model.",
	},
	{
		Name:        "ANTHROPIC_DEFAULT_SONNET_MODEL_DESCRIPTION",
		Category:    "Model Configuration",
		Description: "Display description for Sonnet model.",
	},
	{
		Name:        "ANTHROPIC_DEFAULT_SONNET_MODEL_SUPPORTED_CAPABILITIES",
		Category:    "Model Configuration",
		Description: "Capabilities for Sonnet model.",
	},
	{
		Name:        "ANTHROPIC_DEFAULT_OPUS_MODEL",
		Category:    "Model Configuration",
		Description: "Default Opus-class model.",
	},
	{
		Name:        "ANTHROPIC_DEFAULT_OPUS_MODEL_NAME",
		Category:    "Model Configuration",
		Description: "Display name for Opus model.",
	},
	{
		Name:        "ANTHROPIC_DEFAULT_OPUS_MODEL_DESCRIPTION",
		Category:    "Model Configuration",
		Description: "Display description for Opus model.",
	},
	{
		Name:        "ANTHROPIC_DEFAULT_OPUS_MODEL_SUPPORTED_CAPABILITIES",
		Category:    "Model Configuration",
		Description: "Capabilities for Opus model.",
	},
	{
		Name:        "ANTHROPIC_SMALL_FAST_MODEL",
		Category:    "Model Configuration",
		Description: "[DEPRECATED] Haiku-class model for background tasks.",
		Deprecated:  true,
		ReplacedBy:  "ANTHROPIC_DEFAULT_HAIKU_MODEL",
	},
	{
		Name:        "ANTHROPIC_SMALL_FAST_MODEL_AWS_REGION",
		Category:    "Model Configuration",
		Description: "Override AWS region for Haiku-class model when using Bedrock.",
	},

	// Cloud Provider (Bedrock)
	{
		Name:        "ANTHROPIC_BEDROCK_BASE_URL",
		Category:    "Cloud Provider (Bedrock)",
		Description: "Override Bedrock endpoint URL for custom endpoints or LLM gateway.",
	},
	{
		Name:        "AWS_BEARER_TOKEN_BEDROCK",
		Category:    "Cloud Provider (Bedrock)",
		Description: "Bedrock API key for authentication.",
	},
	{
		Name:        "CLAUDE_CODE_USE_BEDROCK",
		Category:    "Cloud Provider (Bedrock)",
		Description: "Use Bedrock.",
	},
	{
		Name:        "CLAUDE_CODE_SKIP_BEDROCK_AUTH",
		Category:    "Cloud Provider (Bedrock)",
		Description: "Skip AWS authentication for Bedrock (e.g., when using LLM gateway).",
	},
	{
		Name:        "ENABLE_PROMPT_CACHING_1H_BEDROCK",
		Category:    "Cloud Provider (Bedrock)",
		Description: "Request 1-hour prompt cache TTL instead of default 5 minutes (Bedrock only).",
	},

	// Cloud Provider (Vertex AI)
	{
		Name:        "ANTHROPIC_VERTEX_BASE_URL",
		Category:    "Cloud Provider (Vertex AI)",
		Description: "Override Vertex AI endpoint URL for custom endpoints or LLM gateway.",
	},
	{
		Name:        "ANTHROPIC_VERTEX_PROJECT_ID",
		Category:    "Cloud Provider (Vertex AI)",
		Description: "GCP project ID for Vertex AI (required when using Vertex).",
	},
	{
		Name:        "CLAUDE_CODE_USE_VERTEX",
		Category:    "Cloud Provider (Vertex AI)",
		Description: "Use Vertex.",
	},
	{
		Name:        "CLAUDE_CODE_SKIP_VERTEX_AUTH",
		Category:    "Cloud Provider (Vertex AI)",
		Description: "Skip Google authentication for Vertex (e.g., when using LLM gateway).",
	},

	// Cloud Provider (Foundry)
	{
		Name:        "ANTHROPIC_FOUNDRY_API_KEY",
		Category:    "Cloud Provider (Foundry)",
		Description: "API key for Microsoft Foundry authentication.",
	},
	{
		Name:        "ANTHROPIC_FOUNDRY_BASE_URL",
		Category:    "Cloud Provider (Foundry)",
		Description: "Full base URL for Foundry resource.",
		Conflicts:   []string{"ANTHROPIC_FOUNDRY_RESOURCE"},
	},
	{
		Name:        "ANTHROPIC_FOUNDRY_RESOURCE",
		Category:    "Cloud Provider (Foundry)",
		Description: "Foundry resource name. Required if ANTHROPIC_FOUNDRY_BASE_URL not set.",
		Conflicts:   []string{"ANTHROPIC_FOUNDRY_BASE_URL"},
	},
	{
		Name:        "CLAUDE_CODE_USE_FOUNDRY",
		Category:    "Cloud Provider (Foundry)",
		Description: "Use Microsoft Foundry.",
	},
	{
		Name:        "CLAUDE_CODE_SKIP_FOUNDRY_AUTH",
		Category:    "Cloud Provider (Foundry)",
		Description: "Skip Azure authentication for Foundry (e.g., when using LLM gateway).",
	},

	// Authentication & OAuth
	{
		Name:        "CLAUDE_CODE_OAUTH_TOKEN",
		Category:    "Authentication & OAuth",
		Description: "OAuth access token for Claude.ai authentication.",
	},
	{
		Name:        "CLAUDE_CODE_OAUTH_REFRESH_TOKEN",
		Category:    "Authentication & OAuth",
		Description: "OAuth refresh token for Claude.ai authentication. Requires CLAUDE_CODE_OAUTH_SCOPES.",
		Requires:    []string{"CLAUDE_CODE_OAUTH_SCOPES"},
	},
	{
		Name:        "CLAUDE_CODE_OAUTH_SCOPES",
		Category:    "Authentication & OAuth",
		Description: "Space-separated OAuth scopes refresh token was issued with. Required when CLAUDE_CODE_OAUTH_REFRESH_TOKEN is set.",
	},

	// Thinking & Reasoning
	{
		Name:        "CLAUDE_CODE_DISABLE_THINKING",
		Category:    "Thinking & Reasoning",
		Description: "Force-disable extended thinking regardless of model support or settings.",
	},
	{
		Name:        "DISABLE_INTERLEAVED_THINKING",
		Category:    "Thinking & Reasoning",
		Description: "Prevent sending interleaved-thinking beta header.",
	},
	{
		Name:        "MAX_THINKING_TOKENS",
		Category:    "Thinking & Reasoning",
		Description: "Maximum thinking tokens for extended thinking models.",
	},
	{
		Name:        "CLAUDE_CODE_DISABLE_ADAPTIVE_THINKING",
		Category:    "Thinking & Reasoning",
		Description: "Disable adaptive reasoning for Opus 4.6 and Sonnet 4.6.",
	},
	{
		Name:        "CLAUDE_CODE_EFFORT_LEVEL",
		Category:    "Thinking & Reasoning",
		Description: "Set effort level for supported models.",
	},

	// Token & Output Limits
	{
		Name:        "CLAUDE_CODE_MAX_OUTPUT_TOKENS",
		Category:    "Token & Output Limits",
		Description: "Set maximum output tokens for most requests.",
	},
	{
		Name:        "CLAUDE_CODE_FILE_READ_MAX_OUTPUT_TOKENS",
		Category:    "Token & Output Limits",
		Description: "Override default token limit for file reads.",
	},
	{
		Name:        "API_TIMEOUT_MS",
		Category:    "Token & Output Limits",
		Description: "Timeout for API requests in milliseconds.",
		Default:     "600000",
	},

	// Bash & Shell
	{
		Name:        "BASH_DEFAULT_TIMEOUT_MS",
		Category:    "Bash & Shell",
		Description: "Default timeout for long-running bash commands.",
	},
	{
		Name:        "BASH_MAX_OUTPUT_LENGTH",
		Category:    "Bash & Shell",
		Description: "Maximum characters in bash outputs before middle-truncation.",
	},
	{
		Name:        "BASH_MAX_TIMEOUT_MS",
		Category:    "Bash & Shell",
		Description: "Maximum timeout model can set for long-running bash commands.",
	},
	{
		Name:        "CLAUDECODE",
		Category:    "Bash & Shell",
		Description: "Set to 1 in shell environments Claude Code spawns. Not set in hooks or status line commands.",
		Default:     "1",
	},
	{
		Name:        "CLAUDE_CODE_SHELL",
		Category:    "Bash & Shell",
		Description: "Override automatic shell detection.",
	},
	{
		Name:        "CLAUDE_CODE_SHELL_PREFIX",
		Category:    "Bash & Shell",
		Description: "Command prefix to wrap all bash commands (e.g., for logging/auditing).",
	},
	{
		Name:        "CLAUDE_CODE_GIT_BASH_PATH",
		Category:    "Bash & Shell",
		Description: "Windows only: path to Git Bash executable.",
	},
	{
		Name:        "CLAUDE_CODE_USE_POWERSHELL_TOOL",
		Category:    "Bash & Shell",
		Description: "Enable PowerShell tool on Windows (opt-in preview).",
	},
	{
		Name:        "CLAUDE_CODE_SUBPROCESS_ENV_SCRUB",
		Category:    "Bash & Shell",
		Description: "Strip Anthropic and cloud provider credentials from subprocess environments.",
	},
	{
		Name:        "CLAUDE_BASH_MAINTAIN_PROJECT_WORKING_DIR",
		Category:    "Bash & Shell",
		Description: "Return to original working directory after each Bash command.",
	},

	// Memory & Context
	{
		Name:        "CLAUDE_CODE_DISABLE_CLAUDE_MDS",
		Category:    "Memory & Context",
		Description: "Prevent loading any CLAUDE.md memory files.",
	},
	{
		Name:        "CLAUDE_CODE_DISABLE_AUTO_MEMORY",
		Category:    "Memory & Context",
		Description: "Disable auto memory.",
	},
	{
		Name:        "CLAUDE_CODE_CODE_ADDITIONAL_DIRECTORIES_CLAUDE_MD",
		Category:    "Memory & Context",
		Description: "Load CLAUDE.md from directories specified with --add-dir.",
	},
	{
		Name:        "CLAUDE_CODE_DISABLE_ATTACHMENTS",
		Category:    "Memory & Context",
		Description: "Disable attachment processing. File mentions with @ sent as plain text.",
	},
	{
		Name:        "DISABLE_AUTO_COMPACT",
		Category:    "Memory & Context",
		Description: "Disable automatic compaction when approaching context limit.",
	},
	{
		Name:        "DISABLE_COMPACT",
		Category:    "Memory & Context",
		Description: "Disable all compaction (automatic and manual /compact command).",
	},
	{
		Name:        "CLAUDE_AUTOCOMPACT_PCT_OVERRIDE",
		Category:    "Memory & Context",
		Description: "Set percentage of context capacity at which auto-compaction triggers.",
		Default:     "95",
	},
	{
		Name:        "CLAUDE_CODE_AUTO_COMPACT_WINDOW",
		Category:    "Memory & Context",
		Description: "Set context capacity in tokens for auto-compaction calculations.",
	},

	// Prompt Caching
	{
		Name:        "DISABLE_PROMPT_CACHING",
		Category:    "Prompt Caching",
		Description: "Disable prompt caching for all models.",
	},
	{
		Name:        "DISABLE_PROMPT_CACHING_HAIKU",
		Category:    "Prompt Caching",
		Description: "Disable prompt caching for Haiku models.",
	},
	{
		Name:        "DISABLE_PROMPT_CACHING_SONNET",
		Category:    "Prompt Caching",
		Description: "Disable prompt caching for Sonnet models.",
	},
	{
		Name:        "DISABLE_PROMPT_CACHING_OPUS",
		Category:    "Prompt Caching",
		Description: "Disable prompt caching for Opus models.",
	},

	// File & Directory
	{
		Name:        "CLAUDE_CONFIG_DIR",
		Category:    "File & Directory",
		Description: "Override configuration directory.",
		Default:     "~/.claude",
	},
	{
		Name:        "CLAUDE_CODE_TMPDIR",
		Category:    "File & Directory",
		Description: "Override temp directory for internal temp files.",
	},
	{
		Name:        "CLAUDE_CODE_PLUGIN_CACHE_DIR",
		Category:    "File & Directory",
		Description: "Override plugins root directory.",
		Default:     "~/.claude/plugins",
	},
	{
		Name:        "CLAUDE_CODE_PLUGIN_SEED_DIR",
		Category:    "File & Directory",
		Description: "Path to read-only plugin seed directories (: or ; separated).",
	},
	{
		Name:        "CLAUDE_CODE_GLOB_HIDDEN",
		Category:    "File & Directory",
		Description: "Exclude dotfiles from Glob tool results.",
	},
	{
		Name:        "CLAUDE_CODE_GLOB_NO_IGNORE",
		Category:    "File & Directory",
		Description: "Make Glob tool respect .gitignore patterns.",
	},
	{
		Name:        "CLAUDE_CODE_GLOB_TIMEOUT_SECONDS",
		Category:    "File & Directory",
		Description: "Timeout for Glob tool file discovery in seconds.",
		Default:     "20",
	},

	// Checkpointing
	{
		Name:        "CLAUDE_CODE_DISABLE_FILE_CHECKPOINTING",
		Category:    "Checkpointing",
		Description: "Disable file checkpointing. The /rewind command cannot restore code changes.",
	},

	// Debugging & Logging
	{
		Name:        "CLAUDE_CODE_DEBUG_LOGS_DIR",
		Category:    "Debugging & Logging",
		Description: "Override debug log file path. Requires debug mode enabled separately.",
		Default:     "~/.claude/debug/<session-id>.txt",
	},
	{
		Name:        "CLAUDE_CODE_DEBUG_LOG_LEVEL",
		Category:    "Debugging & Logging",
		Description: "Minimum log level for debug file.",
		Default:     "debug",
	},
	{
		Name:        "CLAUDE_ENABLE_STREAM_WATCHDOG",
		Category:    "Debugging & Logging",
		Description: "Abort API response streams stalling >90 seconds.",
	},
	{
		Name:        "CLAUDE_STREAM_IDLE_TIMEOUT_MS",
		Category:    "Debugging & Logging",
		Description: "Timeout in milliseconds before streaming idle watchdog closes stalled connection. Requires CLAUDE_ENABLE_STREAM_WATCHDOG=1.",
		Default:     "90000",
		Requires:    []string{"CLAUDE_ENABLE_STREAM_WATCHDOG"},
	},

	// Tools & Features
	{
		Name:        "CLAUDE_CODE_DISABLE_BACKGROUND_TASKS",
		Category:    "Tools & Features",
		Description: "Disable background task functionality.",
	},
	{
		Name:        "CLAUDE_AUTO_BACKGROUND_TASKS",
		Category:    "Tools & Features",
		Description: "Force-enable automatic backgrounding after ~2 minutes.",
	},
	{
		Name:        "CLAUDE_CODE_DISABLE_CRON",
		Category:    "Tools & Features",
		Description: "Disable scheduled tasks.",
	},
	{
		Name:        "CLAUDE_CODE_ENABLE_TASKS",
		Category:    "Tools & Features",
		Description: "Enable task tracking in non-interactive mode.",
	},
	{
		Name:        "CLAUDE_CODE_MAX_TOOL_USE_CONCURRENCY",
		Category:    "Tools & Features",
		Description: "Maximum parallel read-only tools/subagents.",
		Default:     "10",
	},
	{
		Name:        "ENABLE_TOOL_SEARCH",
		Category:    "Tools & Features",
		Description: "Controls MCP tool search.",
	},
	{
		Name:        "ENABLE_CLAUDEAI_MCP_SERVERS",
		Category:    "Tools & Features",
		Description: "Disable claude.ai MCP servers (enabled by default for logged-in users).",
	},

	// IDE Integration
	{
		Name:        "CLAUDE_CODE_AUTO_CONNECT_IDE",
		Category:    "IDE Integration",
		Description: "Override automatic IDE connection.",
	},
	{
		Name:        "CLAUDE_CODE_IDE_HOST_OVERRIDE",
		Category:    "IDE Integration",
		Description: "Override host address for IDE extension connection.",
	},
	{
		Name:        "CLAUDE_CODE_IDE_SKIP_AUTO_INSTALL",
		Category:    "IDE Integration",
		Description: "Skip auto-installation of IDE extensions.",
	},
	{
		Name:        "CLAUDE_CODE_IDE_SKIP_VALID_CHECK",
		Category:    "IDE Integration",
		Description: "Skip validation of IDE lockfile entries during connection.",
	},

	// UI & Display
	{
		Name:        "CLAUDE_CODE_NO_FLICKER",
		Category:    "UI & Display",
		Description: "Enable fullscreen rendering (research preview reducing flicker).",
	},
	{
		Name:        "CLAUDE_CODE_DISABLE_MOUSE",
		Category:    "UI & Display",
		Description: "Disable mouse tracking in fullscreen.",
	},
	{
		Name:        "CLAUDE_CODE_SCROLL_SPEED",
		Category:    "UI & Display",
		Description: "Mouse wheel scroll multiplier in fullscreen.",
	},
	{
		Name:        "CLAUDE_CODE_DISABLE_TERMINAL_TITLE",
		Category:    "UI & Display",
		Description: "Disable automatic terminal title updates.",
	},
	{
		Name:        "CLAUDE_CODE_CODE_ACCESSIBILITY",
		Category:    "UI & Display",
		Description: "Keep native terminal cursor visible, disable inverted-text indicator.",
	},
	{
		Name:        "CLAUDE_CODE_SYNTAX_HIGHLIGHT",
		Category:    "UI & Display",
		Description: "Disable syntax highlighting in diff output.",
	},
	{
		Name:        "CLAUDE_CODE_ENABLE_PROMPT_SUGGESTION",
		Category:    "UI & Display",
		Description: "Disable prompt suggestions (grayed-out predictions).",
	},

	// Git & Workflows
	{
		Name:        "CLAUDE_CODE_DISABLE_GIT_INSTRUCTIONS",
		Category:    "Git & Workflows",
		Description: "Remove git workflow instructions and status snapshot from system prompt.",
	},

	// Plugin Management
	{
		Name:        "CLAUDE_CODE_DISABLE_OFFICIAL_MARKETPLACE_AUTOINSTALL",
		Category:    "Plugin Management",
		Description: "Skip automatic addition of official plugin marketplace on first run.",
	},
	{
		Name:        "CLAUDE_CODE_PLUGIN_GIT_TIMEOUT_MS",
		Category:    "Plugin Management",
		Description: "Timeout for git operations during plugin install/update in milliseconds.",
		Default:     "120000",
	},
	{
		Name:        "CLAUDE_CODE_PLUGIN_KEEP_MARKETPLACE_ON_FAILURE",
		Category:    "Plugin Management",
		Description: "Keep existing marketplace cache on git pull failure (useful offline).",
	},
	{
		Name:        "CLAUDE_CODE_SYNC_PLUGIN_INSTALL",
		Category:    "Plugin Management",
		Description: "In non-interactive mode, wait for plugin installation completion.",
	},
	{
		Name:        "CLAUDE_CODE_SYNC_PLUGIN_INSTALL_TIMEOUT_MS",
		Category:    "Plugin Management",
		Description: "Timeout for synchronous plugin installation in milliseconds.",
	},

	// Session & Remote Control
	{
		Name:        "CLAUDE_CODE_RESUME_INTERRUPTED_TURN",
		Category:    "Session & Remote Control",
		Description: "Automatically resume if previous session ended mid-turn.",
	},
	{
		Name:        "CLAUDE_CODE_TASK_LIST_ID",
		Category:    "Session & Remote Control",
		Description: "Share task list across sessions with same ID.",
	},
	{
		Name:        "CLAUDE_CODE_TEAM_NAME",
		Category:    "Session & Remote Control",
		Description: "Name of agent team this teammate belongs to.",
	},
	{
		Name:        "CLAUDE_CODE_EXIT_AFTER_STOP_DELAY",
		Category:    "Session & Remote Control",
		Description: "Time in milliseconds to wait before auto-exiting in automated workflows.",
	},
	{
		Name:        "CLAUDE_REMOTE_CONTROL_SESSION_NAME_PREFIX",
		Category:    "Session & Remote Control",
		Description: "Prefix for auto-generated Remote Control session names.",
	},
	{
		Name:        "CLAUDE_CODE_NEW_INIT",
		Category:    "Session & Remote Control",
		Description: "Make /init run interactive setup flow.",
	},

	// Advanced Features
	{
		Name:        "CLAUDE_CODE_ENABLE_FINE_GRAINED_TOOL_STREAMING",
		Category:    "Advanced Features",
		Description: "Force-enable fine-grained tool input streaming.",
	},
	{
		Name:        "CLAUDE_CODE_DISABLE_FAST_MODE",
		Category:    "Advanced Features",
		Description: "Disable fast mode.",
	},
	{
		Name:        "CLAUDE_CODE_DISABLE_1M_CONTEXT",
		Category:    "Advanced Features",
		Description: "Disable 1M context window support.",
	},
	{
		Name:        "CLAUDE_CODE_DISABLE_EXPERIMENTAL_BETAS",
		Category:    "Advanced Features",
		Description: "Strip Anthropic-specific beta headers from API requests.",
	},
	{
		Name:        "CLAUDE_CODE_DISABLE_NONSTREAMING_FALLBACK",
		Category:    "Advanced Features",
		Description: "Disable non-streaming fallback when streaming fails.",
	},
	{
		Name:        "CLAUDE_CODE_SIMPLE",
		Category:    "Advanced Features",
		Description: "Run with minimal system prompt and only Bash/file tools.",
	},
	{
		Name:        "CLAUDE_AGENT_SDK_DISABLE_BUILTIN_AGENTS",
		Category:    "Advanced Features",
		Description: "Disable built-in subagent types (SDK, non-interactive only).",
	},
	{
		Name:        "CLAUDE_AGENT_SDK_MCP_NO_PREFIX",
		Category:    "Advanced Features",
		Description: "Skip mcp__<server>__ prefix on tool names (SDK only).",
	},
	{
		Name:        "CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS",
		Category:    "Advanced Features",
		Description: "Enable agent teams (experimental).",
	},

	// Network & Retry
	{
		Name:        "CLAUDE_CODE_MAX_RETRIES",
		Category:    "Network & Retry",
		Description: "Override number of API request retries.",
		Default:     "10",
	},
	{
		Name:        "CLAUDE_CODE_PROXY_RESOLVES_HOSTS",
		Category:    "Network & Retry",
		Description: "Allow proxy to perform DNS resolution.",
	},

	// Telemetry & Updates
	{
		Name:        "DISABLE_TELEMETRY",
		Category:    "Telemetry & Updates",
		Description: "Opt out of Statsig telemetry.",
	},
	{
		Name:        "DISABLE_ERROR_REPORTING",
		Category:    "Telemetry & Updates",
		Description: "Opt out of Sentry error reporting.",
	},
	{
		Name:        "DISABLE_AUTOUPDATER",
		Category:    "Telemetry & Updates",
		Description: "Disable automatic updates.",
	},
	{
		Name:        "CLAUDE_CODE_DISABLE_FEEDBACK_SURVEY",
		Category:    "Telemetry & Updates",
		Description: "Disable 'How is Claude doing?' surveys.",
	},
	{
		Name:        "CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC",
		Category:    "Telemetry & Updates",
		Description: "Equivalent to setting DISABLE_AUTOUPDATER, DISABLE_FEEDBACK_COMMAND, DISABLE_ERROR_REPORTING, DISABLE_TELEMETRY.",
	},

	// Command Visibility
	{
		Name:        "DISABLE_DOCTOR_COMMAND",
		Category:    "Command Visibility",
		Description: "Hide /doctor command.",
	},
	{
		Name:        "DISABLE_EXTRA_USAGE_COMMAND",
		Category:    "Command Visibility",
		Description: "Hide /extra-usage command.",
	},
	{
		Name:        "DISABLE_FEEDBACK_COMMAND",
		Category:    "Command Visibility",
		Description: "Disable /feedback command.",
	},
	{
		Name:        "DISABLE_INSTALLATION_CHECKS",
		Category:    "Command Visibility",
		Description: "Disable installation warnings.",
	},
	{
		Name:        "DISABLE_INSTALL_GITHUB_APP_COMMAND",
		Category:    "Command Visibility",
		Description: "Hide /install-github-app command.",
	},
	{
		Name:        "DISABLE_LOGIN_COMMAND",
		Category:    "Command Visibility",
		Description: "Hide /login command.",
	},
	{
		Name:        "DISABLE_LOGOUT_COMMAND",
		Category:    "Command Visibility",
		Description: "Hide /logout command.",
	},
	{
		Name:        "DISABLE_UPGRADE_COMMAND",
		Category:    "Command Visibility",
		Description: "Hide /upgrade command.",
	},
	{
		Name:        "DISABLE_COST_WARNINGS",
		Category:    "Command Visibility",
		Description: "Disable cost warning messages.",
	},

	// mTLS Authentication
	{
		Name:        "CLAUDE_CODE_CLIENT_CERT",
		Category:    "mTLS Authentication",
		Description: "Path to client certificate file for mTLS authentication.",
	},
	{
		Name:        "CLAUDE_CODE_CLIENT_KEY",
		Category:    "mTLS Authentication",
		Description: "Path to client private key file for mTLS authentication.",
	},
	{
		Name:        "CLAUDE_CODE_CLIENT_KEY_PASSPHRASE",
		Category:    "mTLS Authentication",
		Description: "Passphrase for encrypted CLAUDE_CODE_CLIENT_KEY (optional).",
	},

	// Environment & Hooks
	{
		Name:        "CLAUDE_ENV_FILE",
		Category:    "Environment & Hooks",
		Description: "Path to shell script sourced before each Bash command.",
	},
	{
		Name:        "CLAUDE_CODE_SESSIONEND_HOOKS_TIMEOUT_MS",
		Category:    "Environment & Hooks",
		Description: "Maximum time for SessionEnd hooks.",
		Default:     "1500",
	},

	// OpenTelemetry
	{
		Name:        "CLAUDE_CODE_ENABLE_TELEMETRY",
		Category:    "OpenTelemetry",
		Description: "Enable OpenTelemetry data collection (required before configuring exporters).",
	},
	{
		Name:        "CLAUDE_CODE_OTEL_FLUSH_TIMEOUT_MS",
		Category:    "OpenTelemetry",
		Description: "Timeout for flushing pending OTel spans in ms.",
		Default:     "5000",
		Requires:    []string{"CLAUDE_CODE_ENABLE_TELEMETRY"},
	},
	{
		Name:        "CLAUDE_CODE_OTEL_SHUTDOWN_TIMEOUT_MS",
		Category:    "OpenTelemetry",
		Description: "Timeout for OTel exporter shutdown in ms.",
		Default:     "2000",
		Requires:    []string{"CLAUDE_CODE_ENABLE_TELEMETRY"},
	},
	{
		Name:        "CLAUDE_CODE_OTEL_HEADERS_HELPER_DEBOUNCE_MS",
		Category:    "OpenTelemetry",
		Description: "Interval for refreshing dynamic OTel headers in ms.",
		Default:     "1740000",
		Requires:    []string{"CLAUDE_CODE_ENABLE_TELEMETRY"},
	},

	// API Key Helper
	{
		Name:        "CLAUDE_CODE_API_KEY_HELPER_TTL_MS",
		Category:    "API Key Helper",
		Description: "Interval for credential refresh when using apiKeyHelper.",
	},

	// Fallback & Overload
	{
		Name:        "FALLBACK_FOR_ALL_PRIMARY_MODELS",
		Category:    "Fallback & Overload",
		Description: "Trigger fallback after repeated overload errors on any primary model.",
	},
}

// All returns a copy of all registered environment variables.
func All() []EnvVar {
	result := make([]EnvVar, len(registry))
	copy(result, registry)
	return result
}

// Get returns the EnvVar for the given name.
// Returns (EnvVar{}, false) if not found.
func Get(name string) (EnvVar, bool) {
	for _, v := range registry {
		if v.Name == name {
			return v, true
		}
	}
	return EnvVar{}, false
}

// GetByCategory returns all variables in the given category.
func GetByCategory(category string) []EnvVar {
	var result []EnvVar
	for _, v := range registry {
		if v.Category == category {
			result = append(result, v)
		}
	}
	return result
}

// Categories returns a sorted list of all unique category names.
func Categories() []string {
	seen := make(map[string]bool)
	for _, v := range registry {
		seen[v.Category] = true
	}
	result := make([]string, 0, len(seen))
	for cat := range seen {
		result = append(result, cat)
	}
	sort.Strings(result)
	return result
}
