package discover

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
)

// setupClaudeFixture writes the two JSON files the scanner expects and
// returns the synthetic ~/.claude directory.
func setupClaudeFixture(t *testing.T, installed, markets string) string {
	t.Helper()
	home := t.TempDir()
	dir := filepath.Join(home, "plugins")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "installed_plugins.json"), []byte(installed), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "known_marketplaces.json"), []byte(markets), 0o644); err != nil {
		t.Fatal(err)
	}
	return home
}

func TestClaudePluginsAt_HappyPath(t *testing.T) {
	t.Parallel()
	home := setupClaudeFixture(t, `{
  "plugins": {
    "everything-claude-code@everything-claude-code": [
      {"version": "1.2.0"}
    ],
    "claude-mem@thedotmack": [
      {"version": "10.2.5"}
    ]
  }
}`, `{
  "everything-claude-code": {
    "source": {"source": "github", "repo": "kentny/everything-claude-code"}
  },
  "thedotmack": {
    "source": {"source": "github", "repo": "thedotmack/claude-mem"}
  }
}`)

	got, err := ClaudePluginsAt(home)
	if err != nil {
		t.Fatalf("ClaudePluginsAt: %v", err)
	}
	// Map ordering is non-deterministic; sort for stable comparison.
	sort.Slice(got, func(i, j int) bool { return got[i].Name < got[j].Name })

	if len(got) != 2 {
		t.Fatalf("got %d plugins, want 2", len(got))
	}
	if got[0].Name != "claude-mem" || got[0].URL != "https://github.com/thedotmack/claude-mem.git" {
		t.Errorf("claude-mem entry = %+v", got[0])
	}
	if got[1].Name != "everything-claude-code" || got[1].URL != "https://github.com/kentny/everything-claude-code.git" {
		t.Errorf("everything-claude-code entry = %+v", got[1])
	}
}

func TestClaudePluginsAt_MissingFilesReturnNil(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	got, err := ClaudePluginsAt(home)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty slice when JSON files missing, got %v", got)
	}
}

func TestClaudePluginsAt_UnresolvedMarketplaceEmitsEmptyURL(t *testing.T) {
	t.Parallel()
	home := setupClaudeFixture(t, `{
  "plugins": {
    "stranded@unknown-marketplace": [{"version": "0.1"}]
  }
}`, `{}`)
	got, err := ClaudePluginsAt(home)
	if err != nil {
		t.Fatalf("ClaudePluginsAt: %v", err)
	}
	if len(got) != 1 || got[0].URL != "" {
		t.Errorf("expected empty URL for unresolved marketplace, got %+v", got)
	}
}

func TestCodexSkillsAt(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	skillsDir := filepath.Join(home, "skills")
	for _, name := range []string{"test-driven-development", ".system", "code-review"} {
		if err := os.MkdirAll(filepath.Join(skillsDir, name), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	got, err := CodexSkillsAt(home)
	if err != nil {
		t.Fatalf("CodexSkillsAt: %v", err)
	}
	sort.Slice(got, func(i, j int) bool { return got[i].Name < got[j].Name })

	if len(got) != 2 {
		t.Fatalf("got %d skills, want 2 (ignoring .system)", len(got))
	}
	if got[0].Name != "code-review" || got[1].Name != "test-driven-development" {
		t.Errorf("unexpected skill list: %+v", got)
	}
	if !filepath.IsAbs(got[0].Path) {
		t.Errorf("expected absolute path, got %q", got[0].Path)
	}
}

func TestCodexSkillsAt_MissingDirReturnsNil(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	got, err := CodexSkillsAt(home)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty slice, got %v", got)
	}
}
