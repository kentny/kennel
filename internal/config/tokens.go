package config

import "strings"

// Tokens is the name→replacement map used by ExpandTokens.
// Order is not significant — replacements are non-overlapping.
type Tokens struct {
	Project string
	Agent   string
	Env     string
}

// ExpandTokens performs `{project}` / `{agent}` / `{env}` substitution,
// matching lib/common.sh::kennel_expand_tokens exactly. Empty token values
// leave the placeholder unresolved — callers can use it to detect "env
// number required but not supplied" style mistakes.
func ExpandTokens(s string, t Tokens) string {
	if t.Project != "" {
		s = strings.ReplaceAll(s, "{project}", t.Project)
	}
	if t.Agent != "" {
		s = strings.ReplaceAll(s, "{agent}", t.Agent)
	}
	if t.Env != "" {
		s = strings.ReplaceAll(s, "{env}", t.Env)
	}
	return s
}
