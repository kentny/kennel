// Package config models the on-disk .kennel.yaml schema and produces a
// Resolved snapshot that every other package reads. The YAML shape mirrors
// the v0.1 bash implementation exactly — existing config files parse
// unchanged under this Go rewrite.
package config

// Config is the raw .kennel.yaml as authored by the user.
// Unknown keys are ignored by yaml.v3 so future additions are backwards-compatible.
type Config struct {
	Version         int               `yaml:"version"`
	Project         Project           `yaml:"project"`
	DefaultAgent    string            `yaml:"default_agent"`
	Worktree        Worktree          `yaml:"worktree"`
	Sandbox         SandboxNaming     `yaml:"sandbox"`
	Network         Network           `yaml:"network"`
	Agents          map[string]Agent  `yaml:"agents"`
	AptPackages     []string          `yaml:"apt_packages"`
	Env             map[string]string `yaml:"env"`
	InitScript      string            `yaml:"init_script"`
	DockerfileExtra string            `yaml:"dockerfile_extra"`
}

// Project holds required fields identifying the repo kennel is bound to.
type Project struct {
	Name string `yaml:"name"`
}

// Worktree controls where `kennel env create` places its git worktrees.
// `Parent` supports tokens ({project}) and ~ expansion; relative paths
// resolve against the directory containing .kennel.yaml.
type Worktree struct {
	Parent string `yaml:"parent"`
}

// SandboxNaming controls the generated docker template image name and
// runtime sandbox name. Templates support {project}/{agent}/{env} tokens.
type SandboxNaming struct {
	TemplateName string `yaml:"template_name"`
	NamePrefix   string `yaml:"name_prefix"`
}

// Network is the shared allow-list applied to every agent. Per-agent
// entries in Agent.AllowHosts / Agent.AllowCidrs are merged on top.
// DefaultPolicy is informational in v0.1 — docker sandbox always runs with
// --policy deny and kennel honors this field for config-template generation.
type Network struct {
	DefaultPolicy string   `yaml:"default_policy"` // "deny" (default) | "allow"
	AllowHosts    []string `yaml:"allow_hosts"`
	AllowCidrs    []string `yaml:"allow_cidrs"`
}

// Agent holds per-agent overrides: base image, plugin repos, additional
// allow-hosts / allow-cidrs. All fields are optional — unset fields fall
// back to built-in defaults in defaults.go.
type Agent struct {
	BaseImage  string   `yaml:"base_image"`
	Plugins    Plugins  `yaml:"plugins"`
	AllowHosts []string `yaml:"allow_hosts"`
	AllowCidrs []string `yaml:"allow_cidrs"`
}

// Plugins describes git repos cloned into /opt/plugins/<path>/ during
// `kennel build`. The sandbox runtime (`kennel run`) forwards one
// --plugin-dir flag per entry when Enabled is true.
type Plugins struct {
	Enabled bool         `yaml:"enabled"`
	Repos   []PluginRepo `yaml:"repos"`
}

// PluginRepo is one line in agents.<name>.plugins.repos.
type PluginRepo struct {
	URL  string `yaml:"url"`
	Path string `yaml:"path"`
}

// Resolved is the flattened, post-merge view that every other package uses.
// It's produced by Load() and carries absolute paths + the chosen agent's
// merged allow-list so callers never have to re-do the merge / expansion
// dance.
type Resolved struct {
	Raw *Config // pointer to the original, in case callers need unmerged data

	ConfigPath string // absolute path to the .kennel.yaml that was loaded
	ConfigRoot string // directory containing ConfigPath (for relative path resolution)

	ProjectName string
	Agent       string // post-override (CLI --agent > default_agent)

	BaseImage       string
	SandboxTemplate string // tokens expanded
	SandboxPrefix   string // tokens expanded
	WorktreeParent  string // absolute, tokens expanded, ~ resolved

	InstallPlugins bool
	PluginRepos    []PluginRepo

	AllowHosts  []string // network.allow_hosts ∪ agents.<agent>.allow_hosts, order-preserving dedup
	AllowCidrs  []string // network.allow_cidrs ∪ agents.<agent>.allow_cidrs
	AptPackages []string

	InitScript      string // composed: `export K=V` lines from .env + raw init_script block
	DockerfileExtra string // absolute path, or "" when unset
}
