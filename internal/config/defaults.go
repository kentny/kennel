package config

// Built-in per-agent defaults. Mirrors lib/config.sh::_kennel_default_base_image.
// Users can override per-agent in their .kennel.yaml via agents.<name>.base_image.
//
// These are not kept in share/templates because they are code — the CLI has to
// know them even when the user's config is minimal (no `agents:` block at all).
var defaultBaseImages = map[string]string{
	"claude": "docker/sandbox-templates:claude-code",
	"codex":  "docker/sandbox-templates:codex-universal",
}

// defaultBaseImage returns the Docker image to use when no override is
// provided. Unknown agents fall through to the claude template since that is
// what kennel has most experience with; users can always override.
func defaultBaseImage(agent string) string {
	if img, ok := defaultBaseImages[agent]; ok {
		return img
	}
	return defaultBaseImages["claude"]
}

// Default template values applied when .kennel.yaml leaves the sandbox /
// worktree fields empty. The token forms intentionally mirror the bundled
// share/templates/kennel.yaml so fresh-init'd projects behave the same as
// projects on a completely blank config.
const (
	defaultSandboxTemplate = "{project}-{agent}-sandbox"
	defaultSandboxPrefix   = "{agent}-{project}"
	defaultWorktreeParent  = "../{project}-wt"
	defaultAgent           = "claude"
	// defaultNetworkPolicy is applied when network.default_policy is empty.
	// Deny is the secure default and matches README / init template copy.
	defaultNetworkPolicy = "deny"
)
