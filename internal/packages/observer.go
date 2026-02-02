package packages

func observerPackage() *Package {
	return &Package{
		Name:        "observer",
		Description: "Event logging — record all hook events for auditing and analysis",
		Rules: []string{
			`on pre-bash -> log`,
			`on post-edit -> log`,
			`on stop -> log`,
		},
	}
}
