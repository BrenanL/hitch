package credentials

import (
	"os"
	"strings"
)

// EnvGet checks for a credential in environment variables.
// Keys are mapped to env vars as HT_<ADAPTER>_<FIELD>, e.g.,
// "ntfy.topic" → HT_NTFY_TOPIC, "discord.webhook_url" → HT_DISCORD_WEBHOOK_URL.
func EnvGet(key string) string {
	envKey := keyToEnv(key)
	return os.Getenv(envKey)
}

// keyToEnv converts a dotted key to an env var name.
// "ntfy.topic" → "HT_NTFY_TOPIC"
func keyToEnv(key string) string {
	s := strings.ToUpper(key)
	s = strings.ReplaceAll(s, ".", "_")
	return "HT_" + s
}
