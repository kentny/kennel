package sandbox

import (
	"strings"
	"testing"

	"github.com/kentny/kennel/internal/config"
)

// baseResolved is the minimal Resolved that still produces a coherent
// Dockerfile — essentials only, no plugins / apt / extra.
func baseResolved() *config.Resolved {
	return &config.Resolved{
		ConfigPath:      "/tmp/.kennel.yaml",
		Agent:           "claude",
		BaseImage:       "docker/sandbox-templates:claude-code",
		SandboxTemplate: "tb-claude-sandbox",
		SandboxPrefix:   "claude-tb",
		InstallPlugins:  false,
	}
}

func TestRenderDockerfile_Minimal(t *testing.T) {
	t.Parallel()
	out, err := RenderDockerfile(baseResolved())
	if err != nil {
		t.Fatalf("RenderDockerfile: %v", err)
	}
	mustContain(t, out, []string{
		"ARG BASE_IMAGE=docker/sandbox-templates:claude-code",
		"FROM ${BASE_IMAGE}",
		"ARG AGENT=claude",
		"ARG INSTALL_PLUGINS=false",
		"COPY init.sh /etc/sandbox-persistent.sh",
		"USER agent",
		`mkdir -p ~/.claude`,
	})
	mustNotContain(t, out, []string{
		"apt-get",
		"git clone",
		"dockerfile_extra",
	})
}

func TestRenderDockerfile_WithAptPackages(t *testing.T) {
	t.Parallel()
	r := baseResolved()
	r.AptPackages = []string{"openjdk-21-jdk", "maven"}
	out, err := RenderDockerfile(r)
	if err != nil {
		t.Fatalf("RenderDockerfile: %v", err)
	}
	mustContain(t, out, []string{
		"RUN apt-get update && apt-get install -y",
		"openjdk-21-jdk",
		"maven",
		"&& rm -rf /var/lib/apt/lists/*",
	})
}

func TestRenderDockerfile_WithPlugins(t *testing.T) {
	t.Parallel()
	r := baseResolved()
	r.InstallPlugins = true
	r.PluginRepos = []config.PluginRepo{
		{URL: "https://github.com/a/b.git", Path: "b"},
		{URL: "https://github.com/c/d.git", Path: "d"},
	}
	out, err := RenderDockerfile(r)
	if err != nil {
		t.Fatalf("RenderDockerfile: %v", err)
	}
	mustContain(t, out, []string{
		"ARG INSTALL_PLUGINS=true",
		"RUN mkdir -p /opt/plugins && \\",
		"git clone --depth 1 https://github.com/a/b.git /opt/plugins/b && \\",
		"git clone --depth 1 https://github.com/c/d.git /opt/plugins/d",
	})
	// No trailing && \ on the final plugin line.
	if strings.Contains(out, "/opt/plugins/d && \\") {
		t.Errorf("final plugin line should not end with && \\, got:\n%s", out)
	}
}

func mustContain(t *testing.T, s string, needles []string) {
	t.Helper()
	for _, n := range needles {
		if !strings.Contains(s, n) {
			t.Errorf("output missing substring %q\n--- full output ---\n%s", n, s)
		}
	}
}

func mustNotContain(t *testing.T, s string, needles []string) {
	t.Helper()
	for _, n := range needles {
		if strings.Contains(s, n) {
			t.Errorf("output unexpectedly contained %q\n--- full output ---\n%s", n, s)
		}
	}
}
