package settings

// KeyType identifies the data type of a settings key.
type KeyType int

const (
	KeyTypeString KeyType = iota
	KeyTypeInt
	KeyTypeBool
	KeyTypeFloat
	KeyTypeEnum
	KeyTypeArray
	KeyTypeMap
	KeyTypeObject
)

// KeyDef describes a single settings.json key.
type KeyDef struct {
	Name         string
	Type         KeyType
	Description  string
	Default      any      // nil = no default
	EnumValues   []string // only for KeyTypeEnum
	ManagedOnly  bool     // true if only settable via managed/policy
	GlobalConfig bool     // true if this belongs in global config, not settings.json
}

// Schema returns all known settings keys with their definitions.
func Schema() []KeyDef {
	return []KeyDef{
		{
			Name:        "agent",
			Type:        KeyTypeString,
			Description: "Run the main thread as a named subagent; applies that subagent's system prompt, tool restrictions, and model",
		},
		{
			Name:        "allowedChannelPlugins",
			Type:        KeyTypeArray,
			Description: "Managed-only allowlist of channel plugins that may push messages",
			ManagedOnly: true,
		},
		{
			Name:        "allowedHttpHookUrls",
			Type:        KeyTypeArray,
			Description: "Allowlist of URL patterns that HTTP hooks may target",
		},
		{
			Name:        "allowedMcpServers",
			Type:        KeyTypeArray,
			Description: "Managed-only allowlist of MCP servers users can configure",
			ManagedOnly: true,
		},
		{
			Name:        "allowManagedHooksOnly",
			Type:        KeyTypeBool,
			Description: "Managed-only. Prevent loading of user, project, and plugin hooks",
			ManagedOnly: true,
		},
		{
			Name:        "allowManagedMcpServersOnly",
			Type:        KeyTypeBool,
			Description: "Managed-only. Only allowedMcpServers from managed settings are respected",
			ManagedOnly: true,
		},
		{
			Name:        "allowManagedPermissionRulesOnly",
			Type:        KeyTypeBool,
			Description: "Managed-only. Prevent user and project settings from defining allow, ask, or deny permission rules",
			ManagedOnly: true,
		},
		{
			Name:        "alwaysThinkingEnabled",
			Type:        KeyTypeBool,
			Description: "Enable extended thinking by default for all sessions",
		},
		{
			Name:        "apiKeyHelper",
			Type:        KeyTypeString,
			Description: "Custom script to generate an auth value sent as X-Api-Key and Authorization: Bearer",
		},
		{
			Name:        "attribution",
			Type:        KeyTypeObject,
			Description: "Customize attribution for git commits and pull requests",
		},
		{
			Name:        "autoMemoryDirectory",
			Type:        KeyTypeString,
			Description: "Custom directory for auto memory storage",
			Default:     "~/.claude/projects/",
		},
		{
			Name:        "autoMode",
			Type:        KeyTypeObject,
			Description: "Customize what the auto mode classifier blocks and allows",
		},
		{
			Name:        "autoUpdatesChannel",
			Type:        KeyTypeEnum,
			Description: "Release channel for updates",
			Default:     "latest",
			EnumValues:  []string{"stable", "latest"},
		},
		{
			Name:        "availableModels",
			Type:        KeyTypeArray,
			Description: "Restrict which models users can select",
		},
		{
			Name:        "awsAuthRefresh",
			Type:        KeyTypeString,
			Description: "Custom script that modifies the .aws directory",
		},
		{
			Name:        "awsCredentialExport",
			Type:        KeyTypeString,
			Description: "Custom script that outputs JSON with AWS credentials",
		},
		{
			Name:        "blockedMarketplaces",
			Type:        KeyTypeArray,
			Description: "Managed-only blocklist of marketplace sources",
			ManagedOnly: true,
		},
		{
			Name:        "channelsEnabled",
			Type:        KeyTypeBool,
			Description: "Managed-only. Allow channels for Team and Enterprise users",
			Default:     false,
			ManagedOnly: true,
		},
		{
			Name:        "cleanupPeriodDays",
			Type:        KeyTypeInt,
			Description: "Sessions inactive longer than this period are deleted at startup. Minimum 1.",
			Default:     30,
		},
		{
			Name:        "companyAnnouncements",
			Type:        KeyTypeArray,
			Description: "Announcement(s) to display to users at startup",
		},
		{
			Name:        "defaultShell",
			Type:        KeyTypeEnum,
			Description: "Default shell for input-box ! commands",
			Default:     "bash",
			EnumValues:  []string{"bash", "powershell"},
		},
		{
			Name:        "deniedMcpServers",
			Type:        KeyTypeArray,
			Description: "Managed-only denylist of MCP servers that are explicitly blocked",
			ManagedOnly: true,
		},
		{
			Name:        "disableAllHooks",
			Type:        KeyTypeBool,
			Description: "Disable all hooks and any custom status line",
		},
		{
			Name:        "disableAutoMode",
			Type:        KeyTypeEnum,
			Description: "Set to disable to prevent auto mode from being activated",
			EnumValues:  []string{"disable"},
		},
		{
			Name:        "disableDeepLinkRegistration",
			Type:        KeyTypeEnum,
			Description: "Set to disable to prevent Claude Code from registering the claude-cli:// protocol handler",
			EnumValues:  []string{"disable"},
		},
		{
			Name:        "disabledMcpjsonServers",
			Type:        KeyTypeArray,
			Description: "List of specific MCP servers from .mcp.json files to reject",
		},
		{
			Name:        "disableSkillShellExecution",
			Type:        KeyTypeBool,
			Description: "Disable inline shell execution for skill and custom command blocks",
		},
		{
			Name:        "effortLevel",
			Type:        KeyTypeEnum,
			Description: "Persist effort level across sessions",
			EnumValues:  []string{"low", "medium", "high"},
		},
		{
			Name:        "enableAllProjectMcpServers",
			Type:        KeyTypeBool,
			Description: "Automatically approve all MCP servers defined in project .mcp.json files",
		},
		{
			Name:        "enabledMcpjsonServers",
			Type:        KeyTypeArray,
			Description: "List of specific MCP servers from .mcp.json files to approve",
		},
		{
			Name:        "enabledPlugins",
			Type:        KeyTypeMap,
			Description: "Controls which plugins are enabled",
		},
		{
			Name:        "env",
			Type:        KeyTypeMap,
			Description: "Environment variables applied to every session",
		},
		{
			Name:        "extraKnownMarketplaces",
			Type:        KeyTypeMap,
			Description: "Defines additional plugin marketplaces available for the repository",
		},
		{
			Name:        "fastModePerSessionOptIn",
			Type:        KeyTypeBool,
			Description: "When true, fast mode does not persist across sessions",
		},
		{
			Name:        "feedbackSurveyRate",
			Type:        KeyTypeFloat,
			Description: "Probability (0-1) that the session quality survey appears when eligible",
		},
		{
			Name:        "fileSuggestion",
			Type:        KeyTypeObject,
			Description: "Configure a custom script for @ file autocomplete",
		},
		{
			Name:        "forceLoginMethod",
			Type:        KeyTypeEnum,
			Description: "Restrict login method",
			EnumValues:  []string{"claudeai", "console"},
			ManagedOnly: true,
		},
		{
			Name:        "forceLoginOrgUUID",
			Type:        KeyTypeString,
			Description: "Require login to belong to a specific organization UUID",
			ManagedOnly: true,
		},
		{
			Name:        "forceRemoteSettingsRefresh",
			Type:        KeyTypeBool,
			Description: "Managed-only. Block CLI startup until remote managed settings are freshly fetched",
			ManagedOnly: true,
		},
		{
			Name:        "hooks",
			Type:        KeyTypeObject,
			Description: "Configure custom commands to run at lifecycle events",
		},
		{
			Name:        "httpHookAllowedEnvVars",
			Type:        KeyTypeArray,
			Description: "Allowlist of environment variable names HTTP hooks may interpolate into headers",
		},
		{
			Name:        "includeCoAuthoredBy",
			Type:        KeyTypeBool,
			Description: "Deprecated: Use attribution instead. Whether to include co-authored-by Claude byline",
			Default:     true,
		},
		{
			Name:        "includeGitInstructions",
			Type:        KeyTypeBool,
			Description: "Include built-in commit and PR workflow instructions and git status snapshot in system prompt",
			Default:     true,
		},
		{
			Name:        "language",
			Type:        KeyTypeString,
			Description: "Configure Claude's preferred response language",
		},
		{
			Name:        "model",
			Type:        KeyTypeString,
			Description: "Override the default model to use",
		},
		{
			Name:        "modelOverrides",
			Type:        KeyTypeMap,
			Description: "Map Anthropic model IDs to provider-specific model IDs",
		},
		{
			Name:        "otelHeadersHelper",
			Type:        KeyTypeString,
			Description: "Script to generate dynamic OpenTelemetry headers",
		},
		{
			Name:        "outputStyle",
			Type:        KeyTypeString,
			Description: "Configure an output style to adjust the system prompt",
		},
		{
			Name:        "permissions",
			Type:        KeyTypeObject,
			Description: "Permission rules object",
		},
		{
			Name:        "plansDirectory",
			Type:        KeyTypeString,
			Description: "Customize where plan files are stored",
			Default:     "~/.claude/plans",
		},
		{
			Name:        "pluginTrustMessage",
			Type:        KeyTypeString,
			Description: "Managed-only. Custom message appended to the plugin trust warning",
			ManagedOnly: true,
		},
		{
			Name:        "prefersReducedMotion",
			Type:        KeyTypeBool,
			Description: "Reduce or disable UI animations for accessibility",
		},
		{
			Name:        "respectGitignore",
			Type:        KeyTypeBool,
			Description: "Control whether the @ file picker respects .gitignore patterns",
			Default:     true,
		},
		{
			Name:        "sandbox",
			Type:        KeyTypeObject,
			Description: "Sandbox configuration object",
		},
		{
			Name:        "showClearContextOnPlanAccept",
			Type:        KeyTypeBool,
			Description: "Show the clear context option on the plan accept screen",
			Default:     false,
		},
		{
			Name:        "showThinkingSummaries",
			Type:        KeyTypeBool,
			Description: "Show extended thinking summaries in interactive sessions",
			Default:     false,
		},
		{
			Name:        "spinnerTipsEnabled",
			Type:        KeyTypeBool,
			Description: "Show tips in the spinner while Claude is working",
			Default:     true,
		},
		{
			Name:        "spinnerTipsOverride",
			Type:        KeyTypeObject,
			Description: "Override spinner tips",
		},
		{
			Name:        "spinnerVerbs",
			Type:        KeyTypeObject,
			Description: "Customize action verbs in spinner and turn duration messages",
		},
		{
			Name:        "statusLine",
			Type:        KeyTypeObject,
			Description: "Configure a custom status line to display context",
		},
		{
			Name:        "strictKnownMarketplaces",
			Type:        KeyTypeArray,
			Description: "Managed-only allowlist of plugin marketplaces users can add",
			ManagedOnly: true,
		},
		{
			Name:        "useAutoModeDuringPlan",
			Type:        KeyTypeBool,
			Description: "Whether plan mode uses auto mode semantics when auto mode is available",
			Default:     true,
		},
		{
			Name:        "voiceEnabled",
			Type:        KeyTypeBool,
			Description: "Enable push-to-talk voice dictation",
		},
		{
			Name:        "worktree.symlinkDirectories",
			Type:        KeyTypeArray,
			Description: "Directories to symlink from main repository into each worktree",
		},
		{
			Name:        "worktree.sparsePaths",
			Type:        KeyTypeArray,
			Description: "Directories to check out in each worktree via git sparse-checkout",
		},
		// Global config keys (belong in ~/.claude/config.json, not settings.json)
		{
			Name:         "autoConnectIde",
			Type:         KeyTypeBool,
			Description:  "Automatically connect to a running IDE when Claude Code starts",
			Default:      false,
			GlobalConfig: true,
		},
		{
			Name:         "autoInstallIdeExtension",
			Type:         KeyTypeBool,
			Description:  "Automatically install the Claude Code IDE extension when running from a VS Code terminal",
			Default:      true,
			GlobalConfig: true,
		},
		{
			Name:         "editorMode",
			Type:         KeyTypeEnum,
			Description:  "Key binding mode for the input prompt",
			Default:      "normal",
			EnumValues:   []string{"normal", "vim"},
			GlobalConfig: true,
		},
		{
			Name:         "showTurnDuration",
			Type:         KeyTypeBool,
			Description:  "Show turn duration messages after responses",
			Default:      true,
			GlobalConfig: true,
		},
		{
			Name:         "terminalProgressBarEnabled",
			Type:         KeyTypeBool,
			Description:  "Show the terminal progress bar in supported terminals",
			Default:      true,
			GlobalConfig: true,
		},
		{
			Name:         "teammateMode",
			Type:         KeyTypeEnum,
			Description:  "How agent team teammates display",
			EnumValues:   []string{"auto", "in-process", "tmux"},
			GlobalConfig: true,
		},
		{
			Name:         "mcpServers",
			Type:         KeyTypeMap,
			Description:  "Personal MCP server configurations",
			GlobalConfig: true,
		},
		{
			Name:         "projects",
			Type:         KeyTypeMap,
			Description:  "Per-project state including trust-dialog acceptance and last-session metrics",
			GlobalConfig: true,
		},
		{
			Name:         "theme",
			Type:         KeyTypeString,
			Description:  "UI theme preference",
			GlobalConfig: true,
		},
	}
}

// ManagedOnlyKeys returns keys that are managed-only (only settable via managed/policy scope).
func ManagedOnlyKeys() []string {
	var keys []string
	for _, def := range Schema() {
		if def.ManagedOnly {
			keys = append(keys, def.Name)
		}
	}
	return keys
}

// GlobalConfigKeys returns keys that belong in global config (~/.claude/config.json), not settings.json.
func GlobalConfigKeys() []string {
	var keys []string
	for _, def := range Schema() {
		if def.GlobalConfig {
			keys = append(keys, def.Name)
		}
	}
	return keys
}
