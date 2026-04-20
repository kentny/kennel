package sandbox

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/kentny/kennel/internal/config"
	"github.com/kentny/kennel/internal/tui"
)

// SandboxExists reports whether a sandbox with the given name shows up in
// `docker sandbox ls` output. Parsing the text table is hacky but docker's
// CLI does not currently expose a structured existence check for sandboxes,
// and this is the same approach env.List uses to surface status columns.
// Exported so the env package can reuse it for collision-detection in
// Create/Rebuild — reusing one implementation keeps the two ls-parsers from
// drifting apart.
func SandboxExists(ctx context.Context, name string) (bool, error) {
	cmd := exec.CommandContext(ctx, "docker", "sandbox", "ls")
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return false, err
	}
	scanner := bufio.NewScanner(&stdout)
	// First line is the header ("NAME STATUS ..."); skip it by checking the
	// first column against the literal "NAME" as an extra guard.
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) == 0 || fields[0] == "NAME" {
			continue
		}
		if fields[0] == name {
			return true, nil
		}
	}
	return false, nil
}

// Build runs `docker build` over a freshly-written build context.
// Streams docker's stdout/stderr through so the user sees real-time progress.
func Build(ctx context.Context, r *config.Resolved) error {
	tui.Info("building image %s (agent=%s, base=%s)", r.SandboxTemplate, r.Agent, r.BaseImage)

	dir, cleanup, err := WriteBuildContext(r)
	if err != nil {
		return fmt.Errorf("preparing build context: %w", err)
	}
	defer cleanup()

	tui.Dim("  build context: %s", dir)

	args := []string{
		"build",
		"--build-arg", "BASE_IMAGE=" + r.BaseImage,
		"--build-arg", "AGENT=" + r.Agent,
		"--build-arg", fmt.Sprintf("INSTALL_PLUGINS=%t", r.InstallPlugins),
		"-t", r.SandboxTemplate,
		dir,
	}
	return runDocker(ctx, args...)
}

// Run is the top-level launcher for `kennel run`: it makes sure the sandbox
// exists (creating it on first call, reusing it on subsequent calls),
// reapplies network policy every time, and then shells into
// `docker sandbox run`.
//
// Re-running `kennel run` against a still-existing sandbox is explicitly
// supported — it's the normal "I closed the agent and want to reconnect"
// workflow. Only when the sandbox is missing do we go through `create`.
func Run(ctx context.Context, r *config.Resolved, workspace string, extraArgs []string) error {
	name := r.SandboxPrefix

	exists, err := SandboxExists(ctx, name)
	if err != nil {
		return fmt.Errorf("probing existing sandboxes: %w", err)
	}

	if exists {
		tui.Info("reusing existing sandbox %s", name)
	} else {
		tui.Info("creating sandbox %s from template %s", name, r.SandboxTemplate)
		// `docker sandbox create` dispatches on a POSITIONAL agent subcommand
		// (claude / codex / shell / ...) — not a flag. Omitting it drops you
		// into docker's own help screen with exit 0, which looked like success
		// but left no sandbox behind. Order is strict: flags, then agent,
		// then workspace path.
		if err := runDocker(ctx,
			"sandbox", "create",
			"--template", r.SandboxTemplate,
			"--name", name,
			r.Agent,
			workspace,
		); err != nil {
			return err
		}
	}

	// Network apply is idempotent and cheap, so we re-push on every run —
	// this keeps the sandbox policy in sync with .kennel.yaml edits without
	// forcing `kennel network apply` as a separate step.
	if err := ApplyNetwork(ctx, r, name); err != nil {
		return err
	}

	tui.Info("launching sandbox %s", name)
	// docker sandbox run places agent-specific flags AFTER `--`, not before
	// the SANDBOX name. Extra args from `kennel run -- ...` are forwarded to
	// the same agent args list.
	return runDocker(ctx, buildRunArgs(name, pluginDirArgs(r), extraArgs)...)
}

// Bash drops the user into a shell inside the already-running sandbox.
func Bash(ctx context.Context, r *config.Resolved) error {
	name := r.SandboxPrefix
	tui.Info("opening bash in %s", name)
	return runDocker(ctx, "sandbox", "exec", "-it", name, "bash")
}

// Rm deletes the sandbox (image stays intact). Non-existent sandboxes are
// surfaced as a warning, not an error, matching the bash behavior.
func Rm(ctx context.Context, r *config.Resolved) error {
	name := r.SandboxPrefix
	tui.Info("removing sandbox %s", name)
	if err := runDocker(ctx, "sandbox", "rm", name); err != nil {
		tui.Warn("sandbox %s was not present", name)
		return nil
	}
	return nil
}

// ApplyNetwork pushes the configured network policy into the named sandbox.
// Called both from `Run` (after create) and from `kennel network apply` (on
// demand after editing .kennel.yaml).
//
// Branches on r.NetworkPolicy:
//   - "allow" → `docker sandbox network proxy --policy allow`, no allow-list
//     flags (the sandbox has full outbound). Any allow_hosts / allow_cidrs
//     configured in .kennel.yaml are ignored at runtime; they're preserved on
//     disk so flipping back to "deny" is a one-line edit.
//   - "deny"  → deny-by-default with the merged allow-list. Empty list means
//     full isolation, which we warn about since it's almost certainly a
//     misconfiguration.
func ApplyNetwork(ctx context.Context, r *config.Resolved, sandboxName string) error {
	policy := r.NetworkPolicy
	if policy == "" {
		policy = "deny"
	}

	if policy == "allow" {
		tui.Info("applying network policy to %s (allow — full outbound)", sandboxName)
		if len(r.AllowHosts) > 0 || len(r.AllowCidrs) > 0 {
			tui.Warn("network.default_policy is 'allow' — allow_hosts / allow_cidrs are ignored at runtime")
		}
		return runDocker(ctx, "sandbox", "network", "proxy", sandboxName, "--policy", "allow")
	}

	tui.Info("applying network policy to %s (deny by default)", sandboxName)
	if len(r.AllowHosts) == 0 && len(r.AllowCidrs) == 0 {
		tui.Warn("no allow_hosts or allow_cidrs configured — sandbox will be fully isolated")
	}

	args := []string{"sandbox", "network", "proxy", sandboxName, "--policy", "deny"}
	for _, h := range r.AllowHosts {
		args = append(args, "--allow-host", h)
	}
	for _, c := range r.AllowCidrs {
		args = append(args, "--allow-cidr", c)
	}
	return runDocker(ctx, args...)
}

// buildRunArgs assembles argv for `docker sandbox run`. The docker help text:
//
//	docker sandbox run SANDBOX [-- AGENT_ARGS...]
//
// …requires agent-specific flags (--plugin-dir and anything forwarded via
// `kennel run -- …`) to sit AFTER the `--` separator. Emitting them before
// SANDBOX used to produce "unknown flag: --plugin-dir" because docker treats
// everything up to SANDBOX as its own flag set.
func buildRunArgs(sandboxName string, pluginArgs, extraArgs []string) []string {
	args := []string{"sandbox", "run", sandboxName}
	if len(pluginArgs) == 0 && len(extraArgs) == 0 {
		return args
	}
	args = append(args, "--")
	args = append(args, pluginArgs...)
	args = append(args, extraArgs...)
	return args
}

// pluginDirArgs returns `--plugin-dir /opt/plugins/<path>` pairs for each
// enabled plugin repo. Empty slice when plugins are disabled or none were
// declared — callers can append the result without conditional checks.
func pluginDirArgs(r *config.Resolved) []string {
	if !r.InstallPlugins || len(r.PluginRepos) == 0 {
		return nil
	}
	out := make([]string, 0, len(r.PluginRepos)*2)
	for _, p := range r.PluginRepos {
		if p.Path == "" {
			continue
		}
		out = append(out, "--plugin-dir", "/opt/plugins/"+p.Path)
	}
	return out
}

// runDocker executes `docker ARGS…` with the parent process's stdio. Cobra
// commands pass their context in so `^C` during a long build cancels the
// exec promptly.
func runDocker(ctx context.Context, args ...string) error {
	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker %v: %w", args, err)
	}
	return nil
}
