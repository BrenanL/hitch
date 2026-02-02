package packages

func qualityPackage() *Package {
	return &Package{
		Name:        "quality",
		Description: "Quality gates — run tests and linting before stopping",
		Rules: []string{
			`on stop -> require tests-pass`,
			`on post-edit -> run "npm run lint" async`,
		},
	}
}
