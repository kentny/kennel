package paths

import (
	"strings"
	"testing"
)

func TestEnsureSafe(t *testing.T) {
	t.Parallel()
	ok := []string{
		"/Users/someone/project",
		"/tmp/foo",
		"/var/folders/xyz/abc", // macOS user tmp
		"/home/alice/projects",
		"/opt/work",
	}
	bad := []string{
		"/",
		"/etc",
		"/etc/ssh",
		"/usr/local/bin",
		"/bin/sh",
		"/boot",
		"/System/Library", // macOS
		"/proc/self",
	}
	for _, p := range ok {
		if err := EnsureSafe(p); err != nil {
			t.Errorf("EnsureSafe(%q) unexpectedly rejected: %v", p, err)
		}
	}
	for _, p := range bad {
		if err := EnsureSafe(p); err == nil {
			t.Errorf("EnsureSafe(%q) accepted a sensitive path", p)
		}
	}
}

func TestEnsureUnder(t *testing.T) {
	t.Parallel()
	base := "/home/alice/proj"
	ok := []string{
		"/home/alice/proj",
		"/home/alice/proj/Dockerfile.extra",
		"/home/alice/proj/sub/dir/file",
	}
	bad := []string{
		"/home/alice",       // parent of base
		"/home/alice/other", // sibling
		"/etc/passwd",
		"/home/alice/proj/../x", // unresolved, but after Clean it becomes /home/alice/x
	}
	for _, p := range ok {
		if err := EnsureUnder(p, base); err != nil {
			t.Errorf("EnsureUnder(%q, %q) unexpectedly rejected: %v", p, base, err)
		}
	}
	for _, p := range bad {
		if err := EnsureUnder(p, base); err == nil {
			t.Errorf("EnsureUnder(%q, %q) accepted an escape", p, base)
		} else if !strings.Contains(err.Error(), "escape") && !strings.Contains(err.Error(), "config root") {
			t.Errorf("EnsureUnder(%q): unexpected error shape: %v", p, err)
		}
	}
}
