package profiles

import (
	"embed"
	"encoding/json"
	"fmt"
)

//go:embed builtin/*.json
var builtinFS embed.FS

// builtinProfiles returns all profiles embedded in the binary.
func builtinProfiles() ([]Profile, error) {
	entries, err := builtinFS.ReadDir("builtin")
	if err != nil {
		return nil, fmt.Errorf("reading builtin profiles: %w", err)
	}

	var profiles []Profile
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		data, err := builtinFS.ReadFile("builtin/" + e.Name())
		if err != nil {
			return nil, fmt.Errorf("reading builtin/%s: %w", e.Name(), err)
		}
		var p Profile
		if err := json.Unmarshal(data, &p); err != nil {
			return nil, fmt.Errorf("parsing builtin/%s: %w", e.Name(), err)
		}
		profiles = append(profiles, p)
	}
	return profiles, nil
}
