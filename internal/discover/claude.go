// Package discover inspects the user's local agent caches (~/.claude,
// ~/.codex) to surface plugins / skills that `kennel init` can offer in its
// MultiSelect. It never mutates those directories.
package discover

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Name grammar for plugins we will propose to the user. Sourced from
// ~/.claude JSON written by Claude Code — treated as semi-trusted because
// anything else on the host could have poisoned those files. Anything not
// matching is silently dropped so a weird plugin name doesn't become a
// Dockerfile injection via init template (see SEC review H3).
var claudePluginNameRe = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

// ClaudePlugin is one entry from ~/.claude/plugins/installed_plugins.json
// joined against ~/.claude/plugins/known_marketplaces.json so callers get the
// git URL they can plug into .kennel.yaml's plugins.repos.
type ClaudePlugin struct {
	Name        string // plugin name (the part before `@` in the registry key)
	Marketplace string // marketplace id (the part after `@`)
	URL         string // git clone URL; empty if marketplace did not resolve
	Version     string // informational only — displayed in TUI
}

// ClaudePluginsAt scans the given Claude home directory. Passing the path
// explicitly (instead of always reading $HOME) keeps the function unit-
// testable and lets future code point at an alternate profile if needed.
//
// A nil / empty result with a nil error is the "nothing to show" case —
// missing files are expected for users who have never installed a plugin.
func ClaudePluginsAt(claudeHome string) ([]ClaudePlugin, error) {
	installed := filepath.Join(claudeHome, "plugins", "installed_plugins.json")
	markets := filepath.Join(claudeHome, "plugins", "known_marketplaces.json")

	if _, err := os.Stat(installed); os.IsNotExist(err) {
		return nil, nil
	}
	if _, err := os.Stat(markets); os.IsNotExist(err) {
		return nil, nil
	}

	var inst struct {
		Plugins map[string][]struct {
			Version string `json:"version"`
		} `json:"plugins"`
	}
	if err := readJSON(installed, &inst); err != nil {
		return nil, err
	}

	var market map[string]struct {
		Source struct {
			Source string `json:"source"`
			Repo   string `json:"repo"` // for github
			URL    string `json:"url"`  // for git
		} `json:"source"`
	}
	if err := readJSON(markets, &market); err != nil {
		return nil, err
	}

	out := make([]ClaudePlugin, 0, len(inst.Plugins))
	for key, versions := range inst.Plugins {
		idx := strings.Index(key, "@")
		if idx < 0 {
			// Malformed key (no `@`) — skip rather than surface something we
			// cannot turn into a git URL.
			continue
		}
		name := key[:idx]
		mk := key[idx+1:]
		// Drop entries that would be unsafe to emit into the generated
		// .kennel.yaml. Better silent filter than TOCTOU on validation later.
		if !claudePluginNameRe.MatchString(name) || !claudePluginNameRe.MatchString(mk) {
			continue
		}
		src := market[mk].Source

		url := ""
		switch src.Source {
		case "github":
			// Repo must look like "owner/name" with no whitespace / metas so
			// the constructed URL can't carry a YAML-breakout character.
			if src.Repo != "" && isSafeGithubRepo(src.Repo) {
				url = "https://github.com/" + src.Repo + ".git"
			}
		case "git":
			if isSafeHTTPSURL(src.URL) {
				url = src.URL
			}
		}

		version := ""
		if len(versions) > 0 {
			version = versions[0].Version
		}
		out = append(out, ClaudePlugin{
			Name:        name,
			Marketplace: mk,
			URL:         url,
			Version:     version,
		})
	}
	return out, nil
}

// githubRepoRe is the owner/name shape we accept from known_marketplaces.json.
// No whitespace, no `"`, no URL separators beyond the single `/`.
var githubRepoRe = regexp.MustCompile(`^[A-Za-z0-9._-]+/[A-Za-z0-9._-]+$`)

func isSafeGithubRepo(s string) bool {
	return githubRepoRe.MatchString(s)
}

// isSafeHTTPSURL narrows to literal-https URLs with no shell / YAML metas so
// the resulting string is safe to drop into the init template without further
// quoting (quoting is done by the template anyway — this is belt-and-braces).
func isSafeHTTPSURL(s string) bool {
	if !strings.HasPrefix(s, "https://") {
		return false
	}
	if strings.ContainsAny(s, " \t\n\r\"'`$&;|<>\\") {
		return false
	}
	return true
}

// ClaudePlugins scans the user's default Claude home (~/.claude).
func ClaudePlugins() ([]ClaudePlugin, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	return ClaudePluginsAt(filepath.Join(home, ".claude"))
}

func readJSON(path string, out any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, out)
}
