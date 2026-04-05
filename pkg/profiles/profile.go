package profiles

// Profile is the in-memory representation of a profile JSON file.
type Profile struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Settings    map[string]any    `json:"settings,omitempty"`
	Env         map[string]string `json:"env,omitempty"`
	Hooks       map[string]any    `json:"hooks,omitempty"`
	Tags        []string          `json:"tags,omitempty"`
	Extends     string            `json:"extends,omitempty"`
}
