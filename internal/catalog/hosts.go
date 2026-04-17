// Package catalog serves the curated host allow-list that `kennel init` uses
// to populate network.allow_hosts interactively. Entries are grouped by
// purpose and marked `Recommended` when they belong to the default set.
//
// Per-agent overlays (Claude / Codex) are appended to the core list when the
// init flow is scoped to that agent. Unknown agents see the core list only.
//
// Content is ported verbatim from lib/host_catalog.sh.
package catalog

// Entry describes one selectable host pattern in the init TUI.
type Entry struct {
	Host        string
	Group       string
	Description string
	Recommended bool
}

// core lists hosts that apply regardless of which agent the user picks.
// Order matters — it controls display order in the MultiSelect.
var core = []Entry{
	{Host: "host.docker.internal", Group: "essentials", Description: "Docker host access (localhost services via this alias)", Recommended: true},
	{Host: "github.com", Group: "essentials", Description: "GitHub web + git over HTTPS", Recommended: true},
	{Host: "*.githubusercontent.com", Group: "essentials", Description: "GitHub raw content / release tarballs", Recommended: true},
	{Host: "api.github.com", Group: "essentials", Description: "GitHub REST API (used by `gh`)", Recommended: true},
	{Host: "ghcr.io", Group: "essentials", Description: "GitHub Container Registry", Recommended: false},

	{Host: "*.npmjs.org", Group: "packages", Description: "npm registry", Recommended: true},
	{Host: "registry.npmjs.org", Group: "packages", Description: "npm registry (primary host)", Recommended: true},
	{Host: "*.yarnpkg.com", Group: "packages", Description: "Yarn registry / classic metadata", Recommended: true},
	{Host: "pypi.org", Group: "packages", Description: "Python package index (metadata)", Recommended: true},
	{Host: "*.pythonhosted.org", Group: "packages", Description: "Python package distribution (.whl, sdist)", Recommended: true},
	{Host: "*.gradle.org", Group: "packages", Description: "Gradle distribution + plugin portal", Recommended: false},
	{Host: "repo.maven.apache.org", Group: "packages", Description: "Maven Central", Recommended: false},
	{Host: "*.maven.org", Group: "packages", Description: "Maven mirrors", Recommended: false},
	{Host: "rubygems.org", Group: "packages", Description: "Ruby gems", Recommended: false},
	{Host: "crates.io", Group: "packages", Description: "Rust crates metadata", Recommended: false},
	{Host: "static.crates.io", Group: "packages", Description: "Rust crate downloads", Recommended: false},
	{Host: "proxy.golang.org", Group: "packages", Description: "Go module proxy", Recommended: false},
	{Host: "sum.golang.org", Group: "packages", Description: "Go checksum database", Recommended: false},

	{Host: "registry-1.docker.io", Group: "registries", Description: "Docker Hub registry", Recommended: false},
	{Host: "*.docker.io", Group: "registries", Description: "Docker Hub CDN", Recommended: false},
	{Host: "*.docker.com", Group: "registries", Description: "Docker website / downloads", Recommended: false},

	{Host: "deb.debian.org", Group: "os", Description: "Debian apt mirrors", Recommended: false},
	{Host: "security.debian.org", Group: "os", Description: "Debian security updates", Recommended: false},
	{Host: "archive.ubuntu.com", Group: "os", Description: "Ubuntu apt mirrors", Recommended: false},
	{Host: "security.ubuntu.com", Group: "os", Description: "Ubuntu security updates", Recommended: false},

	{Host: "*.datadoghq.com", Group: "observability", Description: "Datadog APIs (metrics, traces)", Recommended: false},

	{Host: "*.amazonaws.com", Group: "cloud", Description: "AWS service endpoints", Recommended: false},
	{Host: "docs.aws.amazon.com", Group: "cloud", Description: "AWS documentation", Recommended: false},

	{Host: "*.spring.io", Group: "build", Description: "Spring Initializr + docs", Recommended: false},
	{Host: "*.apache.org", Group: "build", Description: "Apache projects (Tomcat, Kafka, ...)", Recommended: false},
}

var claudeOverlay = []Entry{
	{Host: "*.anthropic.com", Group: "agent", Description: "Anthropic API (required for Claude)", Recommended: true},
	{Host: "platform.claude.com", Group: "agent", Description: "Claude Code platform endpoints", Recommended: true},
}

var codexOverlay = []Entry{
	{Host: "api.openai.com", Group: "agent", Description: "OpenAI API (required for Codex)", Recommended: true},
	{Host: "*.openai.com", Group: "agent", Description: "OpenAI domains (auth, metrics)", Recommended: true},
}

// ForAgent returns the core catalog plus the overlay for the named agent.
// Unknown agent names yield just the core list; the caller can always add
// arbitrary hosts post-init by editing .kennel.yaml.
func ForAgent(agent string) []Entry {
	out := make([]Entry, 0, len(core)+2)
	out = append(out, core...)
	switch agent {
	case "claude":
		out = append(out, claudeOverlay...)
	case "codex":
		out = append(out, codexOverlay...)
	}
	return out
}

// DefaultsForAgent returns the hosts marked Recommended — this is the starting
// selection for the TUI and the final allow-list for non-interactive init.
func DefaultsForAgent(agent string) []string {
	entries := ForAgent(agent)
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.Recommended {
			out = append(out, e.Host)
		}
	}
	return out
}
