package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/kentny/kennel/internal/paths"
)

// LoadOptions configure one call to Load.
type LoadOptions struct {
	// ConfigPath, if non-empty, skips upward discovery and loads exactly this
	// file. Used by the --config flag. Missing file is an error.
	ConfigPath string

	// StartDir is where upward discovery begins. Leave empty to use CWD.
	StartDir string

	// AgentOverride corresponds to the CLI --agent flag. Wins over
	// DefaultAgent in config when non-empty.
	AgentOverride string

	// EnvNumber populates the {env} token. Used by `kennel env *` commands;
	// single-sandbox commands leave it empty.
	EnvNumber string
}

// Load discovers .kennel.yaml, parses it, merges in agent defaults, expands
// tokens, and returns a Resolved snapshot the rest of the codebase reads.
func Load(opts LoadOptions) (*Resolved, error) {
	path, err := discover(opts)
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}

	var raw Config
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}

	if raw.Project.Name == "" {
		return nil, fmt.Errorf("project.name is required in %s", path)
	}

	// Resolve agent up-front — validateConfig needs to know which agent the
	// caller picked so it can stay silent about unused agents in a mixed-use
	// config. We redo the identical decision below to stay side-effect free.
	agentForValidation := opts.AgentOverride
	if agentForValidation == "" {
		agentForValidation = raw.DefaultAgent
	}
	if agentForValidation == "" {
		agentForValidation = defaultAgent
	}
	if err := validateConfig(&raw, agentForValidation); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}

	root := filepath.Dir(path)

	// Agent choice: CLI override > config default > hardcoded "claude".
	agent := opts.AgentOverride
	if agent == "" {
		agent = raw.DefaultAgent
	}
	if agent == "" {
		agent = defaultAgent
	}

	tokens := Tokens{
		Project: raw.Project.Name,
		Agent:   agent,
		Env:     opts.EnvNumber,
	}

	// Base image: agent block override > built-in.
	baseImage := raw.Agents[agent].BaseImage
	if baseImage == "" {
		baseImage = defaultBaseImage(agent)
	}

	// Sandbox naming — fall back to defaults, then expand tokens.
	tplRaw := raw.Sandbox.TemplateName
	if tplRaw == "" {
		tplRaw = defaultSandboxTemplate
	}
	prefixRaw := raw.Sandbox.NamePrefix
	if prefixRaw == "" {
		prefixRaw = defaultSandboxPrefix
	}

	// Worktree parent — expand tokens, then resolve against config dir.
	// worktree.parent can legitimately point outside the repo (the shipped
	// default `../{project}-wt` does exactly that), so we only block paths
	// that land in system directories. The user still owns where kennel
	// writes files within HOME / /tmp / user projects.
	wtRaw := raw.Worktree.Parent
	if wtRaw == "" {
		wtRaw = defaultWorktreeParent
	}
	wtRaw = ExpandTokens(wtRaw, tokens)
	worktreeParent := paths.Resolve(wtRaw, root)
	if err := paths.EnsureSafe(worktreeParent); err != nil {
		return nil, fmt.Errorf("worktree.parent: %w", err)
	}

	// Plugins — inherit the agent's `enabled` flag; list its repos verbatim.
	plugins := raw.Agents[agent].Plugins

	// Allow lists: common (network.allow_*) ∪ per-agent, preserving order and
	// deduping. We match the bash impl's `awk 'NF && !seen[$0]++' behavior.
	hosts := mergeDedup(raw.Network.AllowHosts, raw.Agents[agent].AllowHosts)
	cidrs := mergeDedup(raw.Network.AllowCidrs, raw.Agents[agent].AllowCidrs)

	// Network policy: explicit > default. validateConfig has already asserted
	// the value is "", "deny", or "allow" so this switch is exhaustive.
	policy := raw.Network.DefaultPolicy
	if policy == "" {
		policy = defaultNetworkPolicy
	}

	// Init script: declared env vars as export KEY=VAL lines, then the raw
	// init_script block. Keys are sorted so the output is deterministic for
	// golden tests.
	initScript := composeInitScript(raw.Env, raw.InitScript)

	// dockerfile_extra: strict — must be a real file under ConfigRoot. The
	// content is appended verbatim to the generated Dockerfile as root, so we
	// refuse any path that could read files the user didn't commit alongside
	// .kennel.yaml (think `~/.ssh/id_rsa` exfiltration into image layers).
	var extraAbs string
	if raw.DockerfileExtra != "" {
		extraAbs = paths.Resolve(raw.DockerfileExtra, root)
		if err := paths.EnsureUnder(extraAbs, root); err != nil {
			return nil, fmt.Errorf("dockerfile_extra: %w", err)
		}
		if _, err := os.Stat(extraAbs); err != nil {
			return nil, fmt.Errorf("dockerfile_extra %q: %w", raw.DockerfileExtra, err)
		}
	}

	return &Resolved{
		Raw:        &raw,
		ConfigPath: path,
		ConfigRoot: root,

		ProjectName:     raw.Project.Name,
		Agent:           agent,
		BaseImage:       baseImage,
		SandboxTemplate: ExpandTokens(tplRaw, tokens),
		SandboxPrefix:   ExpandTokens(prefixRaw, tokens),
		WorktreeParent:  worktreeParent,

		InstallPlugins: plugins.Enabled,
		PluginRepos:    plugins.Repos,

		NetworkPolicy: policy,
		AllowHosts:    hosts,
		AllowCidrs:    cidrs,
		AptPackages:   raw.AptPackages,

		InitScript:      initScript,
		DockerfileExtra: extraAbs,
	}, nil
}

// discover walks upward from StartDir until it finds .kennel.yaml, mirroring
// git rev-parse --show-toplevel semantics. ConfigPath override short-circuits
// the search.
func discover(opts LoadOptions) (string, error) {
	if opts.ConfigPath != "" {
		abs, err := filepath.Abs(opts.ConfigPath)
		if err != nil {
			return "", fmt.Errorf("resolving --config path: %w", err)
		}
		if _, err := os.Stat(abs); err != nil {
			return "", fmt.Errorf("config not found: %s", opts.ConfigPath)
		}
		return abs, nil
	}

	start := opts.StartDir
	if start == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return "", fmt.Errorf("getting working directory: %w", err)
		}
		start = cwd
	}
	dir, err := filepath.Abs(start)
	if err != nil {
		return "", err
	}

	for {
		candidate := filepath.Join(dir, ".kennel.yaml")
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break // hit filesystem root
		}
		dir = parent
	}
	return "", errors.New("no .kennel.yaml found in " + start + " or any parent — run 'kennel init' to create one")
}

// mergeDedup concatenates the inputs in order and returns a new slice with
// duplicates removed but original ordering preserved for the first occurrence
// of each value. Empty strings are dropped.
func mergeDedup(lists ...[]string) []string {
	seen := make(map[string]struct{})
	out := make([]string, 0)
	for _, list := range lists {
		for _, s := range list {
			if s == "" {
				continue
			}
			if _, ok := seen[s]; ok {
				continue
			}
			seen[s] = struct{}{}
			out = append(out, s)
		}
	}
	return out
}

// composeInitScript emits deterministic env-var exports followed by the
// user's raw init_script content.
//
// Values are single-quoted so the resulting shell script treats them as
// literal strings, even if the YAML value contains `$`, backtick, quotes,
// etc. Users who need shell-expanding values should write them into
// init_script instead, which is emitted verbatim. This is a security-
// motivated departure from the bash implementation's unquoted `export K=V`
// shape (see `validateConfig`'s rejection of control chars in env values).
func composeInitScript(env map[string]string, raw string) string {
	if len(env) == 0 {
		return raw
	}

	keys := make([]string, 0, len(env))
	for k := range env {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var b strings.Builder
	for _, k := range keys {
		b.WriteString("export ")
		b.WriteString(k)
		b.WriteString("=")
		b.WriteString(shellQuote(env[k]))
		b.WriteString("\n")
	}
	if raw != "" {
		b.WriteString(raw)
	}
	return b.String()
}

// shellQuote wraps s in single quotes, using the standard POSIX `'\”`
// sequence to escape embedded single quotes. The output is safe for direct
// concatenation after `export KEY=` in a bourne-compatible shell.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
