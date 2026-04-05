package envvars

// EnvVar describes a single Claude Code environment variable.
type EnvVar struct {
	Name        string
	Category    string
	Description string
	Default     string   // default value, empty if none
	Deprecated  bool     // true if deprecated
	ReplacedBy  string   // name of replacement var if deprecated
	Requires    []string // other vars this depends on
	Conflicts   []string // vars that conflict with this one
}

// ValidationIssue describes a problem found during validation.
type ValidationIssue struct {
	Var     string
	Level   string // "warning" or "error"
	Message string
}
