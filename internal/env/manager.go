// Package env implements the multi-environment operations: git worktree +
// docker sandbox pairs, indexed by a user-chosen integer N. Ported 1:1 from
// legacy-bash/lib/env.sh and legacy-bash-era env-manager.sh so that muscle
package env

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/kentny/kennel/internal/config"
	"github.com/kentny/kennel/internal/sandbox"
	"github.com/kentny/kennel/internal/tui"
)

// envNumRe validates the ENV positional argument. Same rule the bash version
// used — any positive integer (no leading zeros enforced, no upper bound).
var envNumRe = regexp.MustCompile(`^[0-9]+$`)

func validateNum(s string) error {
	if !envNumRe.MatchString(s) {
		return fmt.Errorf("ENV number must be a positive integer (got: %q)", s)
	}
	return nil
}

func worktreePath(r *config.Resolved, envNum string) string {
	return filepath.Join(r.WorktreeParent, "env-"+envNum)
}

func sandboxName(r *config.Resolved, envNum string) string {
	return r.SandboxPrefix + "-" + envNum
}

// repoRoot returns the absolute path of the current git worktree. Errors out
// cleanly if invoked outside a worktree (e.g. from a tempdir).
func repoRoot() (string, error) {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("current directory is not inside a git worktree (needed for 'env' commands)")
	}
	return strings.TrimSpace(string(out)), nil
}

func branchExists(repo, branch string) bool {
	cmd := exec.Command("git", "-C", repo, "show-ref", "--verify", "--quiet", "refs/heads/"+branch)
	return cmd.Run() == nil
}

// Create runs: git worktree add + docker sandbox create + network apply.
// baseBranch is the `-b <base>` flag equivalent; empty means "HEAD".
func Create(ctx context.Context, r *config.Resolved, envNum, branch, baseBranch string) error {
	if err := validateNum(envNum); err != nil {
		return err
	}

	repo, err := repoRoot()
	if err != nil {
		return err
	}
	wt := worktreePath(r, envNum)
	name := sandboxName(r, envNum)

	if _, err := os.Stat(wt); err == nil {
		return fmt.Errorf("worktree already exists at %s", wt)
	}

	if err := os.MkdirAll(r.WorktreeParent, 0o755); err != nil {
		return fmt.Errorf("creating worktree parent: %w", err)
	}

	if branchExists(repo, branch) {
		if baseBranch != "" {
			tui.Warn("branch %q already exists — ignoring -b", branch)
		}
		tui.Info("creating worktree %s from existing branch %s", wt, branch)
		if err := streamCmd(ctx, "git", "-C", repo, "worktree", "add", wt, branch); err != nil {
			return err
		}
	} else {
		base := baseBranch
		if base == "" {
			base = "HEAD"
		}
		tui.Info("branch %q does not exist — creating new from %s", branch, base)
		if err := streamCmd(ctx, "git", "-C", repo, "worktree", "add", "-b", branch, wt, base); err != nil {
			return err
		}
	}

	// Check for an existing sandbox by the same name BEFORE create. Silently
	// swallowing `docker sandbox create` errors (as the bash prototype did)
	// is a cross-project privilege escalation vector: an attacker's
	// .kennel.yaml can pick a name that collides with a victim project's
	// sandbox, and the subsequent ApplyNetwork would overwrite that
	// sandbox's allow-list. Explicit existence check + refuse-to-reuse
	// makes the collision loud.
	exists, err := sandbox.SandboxExists(ctx, name)
	if err != nil {
		return fmt.Errorf("probing existing sandboxes: %w", err)
	}
	if exists {
		return fmt.Errorf("a sandbox named %q already exists — remove it first or pick a different sandbox.name_prefix in .kennel.yaml", name)
	}

	tui.Info("creating sandbox %s", name)
	if err := streamCmd(ctx,
		"docker", "sandbox", "create",
		"--name", name,
		"-t", r.SandboxTemplate,
		r.Agent, wt,
	); err != nil {
		return fmt.Errorf("docker sandbox create: %w", err)
	}

	if err := sandbox.ApplyNetwork(ctx, r, name); err != nil {
		return err
	}

	tui.Info("environment %s created", envNum)
	tui.Info("  worktree: %s", wt)
	tui.Info("  sandbox:  %s", name)
	tui.Info("  start with: kennel env start %s", envNum)
	return nil
}

// Start launches the sandbox for env N, passing any trailing args through to
// `docker sandbox run` as agent args. Plugin directories configured in
// .kennel.yaml are always forwarded — appended alongside any caller-supplied
// extras, all after the `--` separator as docker requires.
func Start(ctx context.Context, r *config.Resolved, envNum string, extra []string) error {
	if err := validateNum(envNum); err != nil {
		return err
	}
	name := sandboxName(r, envNum)
	tui.Info("starting sandbox %s", name)

	args := []string{"sandbox", "run", name}
	pluginArgs := pluginDirArgs(r)
	if len(pluginArgs) > 0 || len(extra) > 0 {
		args = append(args, "--")
		args = append(args, pluginArgs...)
		args = append(args, extra...)
	}
	return streamCmd(ctx, "docker", args...)
}

// Stop pauses the sandbox without destroying the worktree.
func Stop(ctx context.Context, r *config.Resolved, envNum string) error {
	if err := validateNum(envNum); err != nil {
		return err
	}
	name := sandboxName(r, envNum)
	tui.Info("stopping sandbox %s", name)
	if err := streamCmd(ctx, "docker", "sandbox", "stop", name); err != nil {
		tui.Warn("sandbox %s may not be running", name)
	}
	return nil
}

// Bash opens a debug shell in env N's sandbox.
func Bash(ctx context.Context, r *config.Resolved, envNum string) error {
	if err := validateNum(envNum); err != nil {
		return err
	}
	name := sandboxName(r, envNum)
	tui.Info("opening bash in %s", name)
	return streamCmd(ctx, "docker", "sandbox", "exec", "-it", name, "bash")
}

// Rebuild destroys and recreates the sandbox while preserving the worktree.
// Used after `kennel build` changes the template image and you want env N to
// pick up the refresh without losing its working tree.
func Rebuild(ctx context.Context, r *config.Resolved, envNum string) error {
	if err := validateNum(envNum); err != nil {
		return err
	}
	wt := worktreePath(r, envNum)
	name := sandboxName(r, envNum)

	if _, err := os.Stat(wt); err != nil {
		return fmt.Errorf("worktree not found at %s — use 'env create' first", wt)
	}

	tui.Info("stopping sandbox %s", name)
	_ = exec.CommandContext(ctx, "docker", "sandbox", "stop", name).Run()
	tui.Info("removing sandbox %s", name)
	_ = exec.CommandContext(ctx, "docker", "sandbox", "rm", name).Run()

	tui.Info("recreating sandbox %s from template %s", name, r.SandboxTemplate)
	// `rm` above is best-effort (a non-existent sandbox returns non-zero);
	// the fresh `create` here is the load-bearing operation and must succeed.
	if err := streamCmd(ctx,
		"docker", "sandbox", "create",
		"--name", name,
		"-t", r.SandboxTemplate,
		r.Agent, wt,
	); err != nil {
		return fmt.Errorf("docker sandbox create: %w", err)
	}

	if err := sandbox.ApplyNetwork(ctx, r, name); err != nil {
		return err
	}

	tui.Info("environment %s rebuilt", envNum)
	tui.Info("  worktree: %s (preserved)", wt)
	tui.Info("  sandbox:  %s (recreated)", name)
	return nil
}

// Destroy removes both the worktree and the sandbox for env N.
func Destroy(ctx context.Context, r *config.Resolved, envNum string) error {
	if err := validateNum(envNum); err != nil {
		return err
	}
	repo, err := repoRoot()
	if err != nil {
		return err
	}
	wt := worktreePath(r, envNum)
	name := sandboxName(r, envNum)

	tui.Info("removing sandbox %s", name)
	if err := exec.CommandContext(ctx, "docker", "sandbox", "rm", name).Run(); err != nil {
		tui.Warn("sandbox %s not found or already removed", name)
	}

	if _, err := os.Stat(wt); err == nil {
		tui.Info("removing worktree %s", wt)
		if err := streamCmd(ctx, "git", "-C", repo, "worktree", "remove", wt, "--force"); err != nil {
			return err
		}
	} else {
		tui.Warn("worktree not found at %s", wt)
	}

	tui.Info("environment %s destroyed", envNum)
	return nil
}

// List prints two tables: managed envs (worktrees under {parent}/env-N/) and
// other worktrees in the repo. Mirrors the bash env-manager output format so
// scripts/scrapers keyed on column positions continue to work.
func List(ctx context.Context, r *config.Resolved) error {
	repo, err := repoRoot()
	if err != nil {
		return err
	}
	entries, err := listWorktrees(repo)
	if err != nil {
		return err
	}

	managedRe := regexp.MustCompile(`^` + regexp.QuoteMeta(r.WorktreeParent) + `/env-([0-9]+)$`)

	fmt.Println("Managed Environments:")
	fmt.Println("=====================")
	fmt.Printf("%-5s %-12s %-20s %s\n", "ENV", "STATUS", "BRANCH", "PATH")
	fmt.Printf("%-5s %-12s %-20s %s\n", "---", "------", "------", "----")

	managedCount := 0
	for _, e := range entries {
		m := managedRe.FindStringSubmatch(e.Path)
		if m == nil {
			continue
		}
		managedCount++
		status := sandboxStatus(ctx, sandboxName(r, m[1]))
		fmt.Printf("%-5s %-12s %-20s %s\n", m[1], status, e.Branch, e.Path)
	}
	if managedCount == 0 {
		fmt.Println("(no managed environments)")
	}

	fmt.Println()
	fmt.Println("Other Worktrees:")
	fmt.Println("================")
	fmt.Printf("%-12s %-20s %s\n", "COMMIT", "BRANCH", "PATH")
	fmt.Printf("%-12s %-20s %s\n", "------", "------", "----")
	for _, e := range entries {
		if managedRe.MatchString(e.Path) {
			continue
		}
		fmt.Printf("%-12s %-20s %s\n", e.Commit, e.Branch, e.Path)
	}
	return nil
}

// Entry is one `git worktree list` row.
type Entry struct {
	Path   string
	Commit string
	Branch string
}

// listWorktrees parses `git worktree list` output. Format per line:
//
//	<path>  <commit-sha>  [<branch>]
//
// Any of the fields may contain spaces if the user gave their worktree a path
// with spaces, so we use the trailing bracketed branch as an anchor.
func listWorktrees(repo string) ([]Entry, error) {
	cmd := exec.Command("git", "-C", repo, "worktree", "list")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	var out_entries []Entry
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 2 {
			continue
		}
		e := Entry{Path: fields[0], Commit: fields[1]}
		if len(fields) >= 3 {
			e.Branch = strings.Trim(fields[2], "[]")
		}
		out_entries = append(out_entries, e)
	}
	return out_entries, nil
}

// sandboxStatus reads `docker sandbox ls` once per env to figure out if the
// sandbox is running / stopped / missing. Returns "no sandbox" when not found.
func sandboxStatus(ctx context.Context, name string) string {
	cmd := exec.CommandContext(ctx, "docker", "sandbox", "ls")
	out, err := cmd.Output()
	if err != nil {
		return "no sandbox"
	}
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) >= 3 && fields[0] == name {
			return fields[2]
		}
	}
	return "no sandbox"
}

// streamCmd runs `name args...` with stdio wired to the parent. Used for any
// docker / git invocation that produces user-visible output.
func streamCmd(ctx context.Context, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

// pluginDirArgs mirrors sandbox.pluginDirArgs but lives here to avoid the
// sandbox → env import cycle (sandbox already imports env's parent via
// ApplyNetwork callers). Keeping a second copy is fine; it's four lines.
func pluginDirArgs(r *config.Resolved) []string {
	if !r.InstallPlugins || len(r.PluginRepos) == 0 {
		return nil
	}
	out := make([]string, 0, len(r.PluginRepos)*2)
	for _, p := range r.PluginRepos {
		if p.Path != "" {
			out = append(out, "--plugin-dir", "/opt/plugins/"+p.Path)
		}
	}
	return out
}

// ParseEnvArg is a tiny helper for CLI flag validation at the boundary — we
// don't need strconv for the arithmetic, just for the "did they pass something
// numeric?" check already in validateNum, but exporting this lets the cobra
// command generate a decent error message before dispatch.
func ParseEnvArg(s string) (int, error) {
	n, err := strconv.Atoi(s)
	if err != nil || n < 0 {
		return 0, fmt.Errorf("ENV number must be a non-negative integer (got: %q)", s)
	}
	return n, nil
}
