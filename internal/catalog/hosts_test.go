package catalog

import (
	"slices"
	"testing"
)

func TestForAgent_ClaudeAppendsOverlay(t *testing.T) {
	t.Parallel()
	entries := ForAgent("claude")
	if !containsHost(entries, "*.anthropic.com") {
		t.Error("expected claude overlay to include *.anthropic.com")
	}
	if containsHost(entries, "api.openai.com") {
		t.Error("claude should not include codex hosts")
	}
}

func TestForAgent_UnknownAgentOmitsOverlay(t *testing.T) {
	t.Parallel()
	entries := ForAgent("mystery")
	if containsHost(entries, "*.anthropic.com") || containsHost(entries, "api.openai.com") {
		t.Error("unknown agent must not inherit an agent overlay")
	}
}

func TestDefaultsForAgent_ContainsEssentials(t *testing.T) {
	t.Parallel()
	got := DefaultsForAgent("claude")
	// Sanity: the recommended set is what the bash implementation's regression
	// test verified (11 entries for claude). Hard-coding the count would be
	// fragile if we tune the catalog, so assert presence of a required trio
	// instead — these are the ones most commonly depended on.
	for _, must := range []string{"host.docker.internal", "github.com", "*.anthropic.com"} {
		if !slices.Contains(got, must) {
			t.Errorf("defaults missing required host %q (got %v)", must, got)
		}
	}
}

func containsHost(entries []Entry, host string) bool {
	for _, e := range entries {
		if e.Host == host {
			return true
		}
	}
	return false
}
