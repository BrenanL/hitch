package packages

func safetyPackage() *Package {
	return &Package{
		Name:        "safety",
		Description: "Safety guards — block destructive commands, protect sensitive files, prevent force pushes",
		Rules: []string{
			`on pre-bash -> deny if matches deny-list:destructive`,
			`on pre-edit -> deny "protected file" if file matches "\\.env"`,
			`on pre-bash -> deny "force push blocked" if command matches "git push --force"`,
		},
	}
}
