// Package paths holds small path-resolution helpers shared by config loading
// and subcommands. Keeping them separate avoids a cyclic import between
// config and sandbox when both need to resolve `dockerfile_extra` paths.
package paths

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Resolve returns an absolute path for raw, with:
//   - leading `~` expanded to $HOME
//   - relative paths resolved against base (not against CWD — config-relative
//     semantics matter when kennel is run from a subdirectory)
//
// Mirrors lib/common.sh::kennel_resolve_path.
func Resolve(raw, base string) string {
	if raw == "" {
		return ""
	}
	if strings.HasPrefix(raw, "~") {
		if home, err := os.UserHomeDir(); err == nil {
			raw = filepath.Join(home, strings.TrimPrefix(raw, "~"))
		}
	}
	if filepath.IsAbs(raw) {
		return filepath.Clean(raw)
	}
	return filepath.Clean(filepath.Join(base, raw))
}

// sensitiveRoots is the explicit system-path denylist used by EnsureSafe.
// Scoped to directories that (a) are owned by the OS / root and (b) have no
// legitimate user-writable subtree worth accommodating. `/var` and `/root`
// are intentionally left OFF the list: macOS puts $TMPDIR under /var/folders
// (which tests need), and /root is irrelevant on systems where the user
// isn't root and inaccessible where they are.
var sensitiveRoots = []string{
	"/etc", "/usr", "/bin", "/sbin", "/boot", "/sys", "/proc", "/dev",
	"/System", // macOS
}

// EnsureSafe verifies `abs` does not land inside a well-known system
// directory. It is NOT a containment check — worktree.parent defaults to one
// level up from the config root, so full containment would reject the
// default. This is a narrower safety-net: reject paths where kennel writing
// files is unambiguously wrong (/etc, /usr, /boot, ...) and let the user own
// the decision for everything else (HOME, tmp, other project dirs).
func EnsureSafe(abs string) error {
	if abs == "" {
		return nil
	}
	clean := filepath.Clean(abs)
	if clean == "/" {
		return fmt.Errorf("refusing to use %q as a kennel path (filesystem root)", abs)
	}
	for _, root := range sensitiveRoots {
		if clean == root || strings.HasPrefix(clean, root+"/") {
			return fmt.Errorf("refusing to use %q as a kennel path (system directory %s)", abs, root)
		}
	}
	return nil
}

// EnsureUnder returns an error if `abs` does not sit inside `base`. Used for
// strict-containment fields (dockerfile_extra) where escape-the-config-root
// is always a red flag. base and abs should already be absolute / clean.
func EnsureUnder(abs, base string) error {
	rel, err := filepath.Rel(base, abs)
	if err != nil {
		return fmt.Errorf("resolving %q against %q: %w", abs, base, err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("%q escapes config root %q", abs, base)
	}
	return nil
}
