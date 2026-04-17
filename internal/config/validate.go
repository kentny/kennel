package config

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

// Validation is the second line of defense after argv-based exec (first line:
// all docker/git calls use exec.Command with a slice of args — no shell, no
// injection). The review found that a malicious .kennel.yaml can smuggle
// newlines / shell metacharacters into Dockerfile RUN instructions and init.sh
// `export` lines because text/template does not quote.  Everything that ends
// up as generated shell or Dockerfile content is validated here; anything
// consumed by exec.Command directly (argv) is left alone since Go handles
// argv-safety for us.
//
// Rules are deliberately strict — kennel's target workflow does not need
// exotic package names / image tags / branch identifiers, and the cost of
// loosening later (bug report → regex widened) is much smaller than the cost
// of a successful injection today.
var (
	projectNameRe = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)
	agentNameRe   = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)
	// Docker image reference: host[:port]/namespace/name[:tag][@digest].
	// This is permissive on purpose — admins use registries with odd hostnames.
	imageRefRe   = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:/-]*(@sha256:[0-9a-f]{64})?$`)
	aptPackageRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9.+-]*$`)
	// Plugin path = directory name under /opt/plugins/. Forbid "..".
	pluginPathRe = regexp.MustCompile(`^[A-Za-z0-9._-][A-Za-z0-9._/-]*$`)
	// POSIX-ish shell identifier.
	envKeyRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
	// CIDR: basic 1-3 digit octets + /prefix. docker sandbox does the real parse.
	cidrRe = regexp.MustCompile(`^[0-9]{1,3}(\.[0-9]{1,3}){3}/[0-9]{1,2}$`)
	// Hostname / wildcard. Allows leading *., IPv4, and hostnames with dots.
	hostPatternRe = regexp.MustCompile(`^(\*\.)?[A-Za-z0-9][A-Za-z0-9.-]*(:[0-9]{1,5})?$`)
)

// containsControlOrShell rejects newline / carriage return / NUL and shell
// separators that would slip past template interpolation unnoticed. Values
// that legitimately need these characters belong in init_script or
// dockerfile_extra (both of which are explicitly free-form).
func containsControlOrShell(s string) bool {
	for _, r := range s {
		switch {
		case r == '\n', r == '\r', r == 0:
			return true
		case r < 0x20 || r == 0x7F:
			return true
		}
	}
	return false
}

// validateConfig runs every check that must pass before config.Load returns a
// Resolved to the rest of the codebase. It mutates nothing; errors are
// pass-through so the caller can wrap them with file context.
func validateConfig(c *Config, agent string) error {
	if !projectNameRe.MatchString(c.Project.Name) {
		return fmt.Errorf("project.name %q has invalid characters (allowed: A-Z a-z 0-9 . _ -)", c.Project.Name)
	}
	if c.DefaultAgent != "" && !agentNameRe.MatchString(c.DefaultAgent) {
		return fmt.Errorf("default_agent %q has invalid characters", c.DefaultAgent)
	}
	if !agentNameRe.MatchString(agent) {
		return fmt.Errorf("agent %q has invalid characters", agent)
	}

	for i, p := range c.AptPackages {
		if !aptPackageRe.MatchString(p) {
			return fmt.Errorf("apt_packages[%d] %q is not a valid Debian package name", i, p)
		}
	}

	for k, v := range c.Env {
		if !envKeyRe.MatchString(k) {
			return fmt.Errorf("env key %q is not a valid shell identifier (^[A-Za-z_][A-Za-z0-9_]*$)", k)
		}
		if containsControlOrShell(v) {
			return fmt.Errorf("env[%s] value contains newline or control character", k)
		}
	}

	for i, h := range c.Network.AllowHosts {
		if !hostPatternRe.MatchString(h) {
			return fmt.Errorf("network.allow_hosts[%d] %q is not a valid host pattern", i, h)
		}
	}
	for i, c := range c.Network.AllowCidrs {
		if !cidrRe.MatchString(c) {
			return fmt.Errorf("network.allow_cidrs[%d] %q is not a valid CIDR", i, c)
		}
	}

	for name, a := range c.Agents {
		if !agentNameRe.MatchString(name) {
			return fmt.Errorf("agents.%s: invalid agent name", name)
		}
		if a.BaseImage != "" && !imageRefRe.MatchString(a.BaseImage) {
			return fmt.Errorf("agents.%s.base_image %q is not a valid docker image reference", name, a.BaseImage)
		}
		for i, h := range a.AllowHosts {
			if !hostPatternRe.MatchString(h) {
				return fmt.Errorf("agents.%s.allow_hosts[%d] %q is not a valid host pattern", name, i, h)
			}
		}
		for i, cidr := range a.AllowCidrs {
			if !cidrRe.MatchString(cidr) {
				return fmt.Errorf("agents.%s.allow_cidrs[%d] %q is not a valid CIDR", name, i, cidr)
			}
		}
		for i, repo := range a.Plugins.Repos {
			if err := validatePluginURL(repo.URL); err != nil {
				return fmt.Errorf("agents.%s.plugins.repos[%d].url: %w", name, i, err)
			}
			if !pluginPathRe.MatchString(repo.Path) {
				return fmt.Errorf("agents.%s.plugins.repos[%d].path %q has invalid characters", name, i, repo.Path)
			}
			if strings.Contains(repo.Path, "..") {
				return fmt.Errorf("agents.%s.plugins.repos[%d].path may not contain '..'", name, i)
			}
		}
	}

	// init_script and dockerfile_extra are intentionally free-form shell /
	// Dockerfile content — users opting into those fields know what they are.
	// We still drop NUL bytes as a minimum sanity check.
	if strings.ContainsRune(c.InitScript, 0) {
		return fmt.Errorf("init_script contains NUL byte")
	}
	return nil
}

// validatePluginURL keeps git clone URLs tame: must be https (so they can't
// read local paths), must parse as a URL, must not contain shell-hostile
// characters that would break the Dockerfile RUN line even though the field
// is already quoted by the renderer.
func validatePluginURL(raw string) error {
	if raw == "" {
		return fmt.Errorf("plugin url is empty")
	}
	if strings.ContainsAny(raw, " \t\n\r\"'`$&;|<>\\") {
		return fmt.Errorf("plugin url contains disallowed characters: %q", raw)
	}
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("not a URL: %w", err)
	}
	if u.Scheme != "https" {
		return fmt.Errorf("must use https (got scheme %q)", u.Scheme)
	}
	if u.Host == "" {
		return fmt.Errorf("missing host component")
	}
	return nil
}
