package packages

func notifierPackage() *Package {
	return &Package{
		Name:        "notifier",
		Description: "Notifications for session events — stop alerts, permission requests, idle detection",
		Rules: []string{
			`on stop -> notify ntfy if elapsed > 30s`,
			`on notification:permission -> notify ntfy "Claude needs permission"`,
			`on notification:idle -> notify ntfy "Claude is waiting for input"`,
		},
	}
}
