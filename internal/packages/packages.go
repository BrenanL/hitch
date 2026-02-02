package packages

// Package represents a pre-built bundle of DSL rules.
type Package struct {
	Name        string
	Description string
	Rules       []string // DSL rule strings
}

var registry = map[string]*Package{}

// Register adds a package to the registry.
func Register(pkg *Package) {
	registry[pkg.Name] = pkg
}

// Get returns a package by name.
func Get(name string) *Package {
	return registry[name]
}

// List returns all registered packages.
func List() []*Package {
	pkgs := make([]*Package, 0, len(registry))
	for _, pkg := range registry {
		pkgs = append(pkgs, pkg)
	}
	return pkgs
}

func init() {
	Register(notifierPackage())
	Register(safetyPackage())
	Register(qualityPackage())
	Register(observerPackage())
}
