package adapters

import (
	"fmt"
	"sort"
)

// AdapterFactory is a function that creates a new adapter from config.
type AdapterFactory func(config map[string]string) (Adapter, error)

var registry = map[string]AdapterFactory{}

// Register adds an adapter factory to the registry.
func Register(name string, factory AdapterFactory) {
	registry[name] = factory
}

// NewAdapter creates a new adapter by name with the given config.
func NewAdapter(name string, config map[string]string) (Adapter, error) {
	factory, ok := registry[name]
	if !ok {
		return nil, fmt.Errorf("unknown adapter: %q (available: %v)", name, AvailableAdapters())
	}
	return factory(config)
}

// AvailableAdapters returns the names of all registered adapters.
func AvailableAdapters() []string {
	names := make([]string, 0, len(registry))
	for name := range registry {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func init() {
	Register("ntfy", NewNtfyAdapter)
	Register("discord", NewDiscordAdapter)
	Register("slack", NewSlackAdapter)
	Register("desktop", NewDesktopAdapter)
}
