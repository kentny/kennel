package config

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// writeTempConfig drops a .kennel.yaml into a fresh tempdir and returns the
// tempdir path. Callers remove it via t.TempDir's automatic cleanup.
func writeTempConfig(t *testing.T, yaml string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".kennel.yaml"), []byte(yaml), 0o644); err != nil {
		t.Fatalf("writing fixture: %v", err)
	}
	return dir
}

func TestLoad_RequiresProjectName(t *testing.T) {
	t.Parallel()
	dir := writeTempConfig(t, `version: 1
default_agent: claude
`)
	_, err := Load(LoadOptions{StartDir: dir})
	if err == nil || !strings.Contains(err.Error(), "project.name is required") {
		t.Fatalf("expected project.name error, got %v", err)
	}
}

func TestLoad_AgentOverrideWins(t *testing.T) {
	t.Parallel()
	dir := writeTempConfig(t, `version: 1
project:
  name: tb
default_agent: claude
`)
	got, err := Load(LoadOptions{StartDir: dir, AgentOverride: "codex"})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.Agent != "codex" {
		t.Errorf("Agent = %q, want codex", got.Agent)
	}
	if got.BaseImage != "docker/sandbox-templates:codex-universal" {
		t.Errorf("BaseImage = %q, want codex default", got.BaseImage)
	}
}

func TestLoad_BaseImageUserOverride(t *testing.T) {
	t.Parallel()
	dir := writeTempConfig(t, `version: 1
project:
  name: tb
default_agent: claude
agents:
  claude:
    base_image: custom/image:tag
`)
	got, err := Load(LoadOptions{StartDir: dir})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.BaseImage != "custom/image:tag" {
		t.Errorf("BaseImage = %q, want custom/image:tag", got.BaseImage)
	}
}

func TestLoad_TokensExpanded(t *testing.T) {
	t.Parallel()
	dir := writeTempConfig(t, `version: 1
project:
  name: tb
default_agent: claude
sandbox:
  template_name: "{project}-{agent}-sandbox"
  name_prefix:   "{agent}-{project}"
worktree:
  parent: "../{project}-wt"
`)
	got, err := Load(LoadOptions{StartDir: dir})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.SandboxTemplate != "tb-claude-sandbox" {
		t.Errorf("SandboxTemplate = %q", got.SandboxTemplate)
	}
	if got.SandboxPrefix != "claude-tb" {
		t.Errorf("SandboxPrefix = %q", got.SandboxPrefix)
	}
	// WorktreeParent is "../tb-wt" resolved against the config dir.
	wantWt := filepath.Clean(filepath.Join(dir, "..", "tb-wt"))
	if got.WorktreeParent != wantWt {
		t.Errorf("WorktreeParent = %q, want %q", got.WorktreeParent, wantWt)
	}
}

func TestLoad_MergeDedup(t *testing.T) {
	t.Parallel()
	dir := writeTempConfig(t, `version: 1
project:
  name: tb
default_agent: claude
network:
  allow_hosts:
    - host.docker.internal
    - github.com
  allow_cidrs:
    - 172.16.0.0/12
agents:
  claude:
    allow_hosts:
      - github.com          # dupe — must be dropped
      - "*.anthropic.com"
    allow_cidrs:
      - 10.0.0.0/8
`)
	got, err := Load(LoadOptions{StartDir: dir})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	wantHosts := []string{"host.docker.internal", "github.com", "*.anthropic.com"}
	if !reflect.DeepEqual(got.AllowHosts, wantHosts) {
		t.Errorf("AllowHosts = %v, want %v", got.AllowHosts, wantHosts)
	}
	wantCidrs := []string{"172.16.0.0/12", "10.0.0.0/8"}
	if !reflect.DeepEqual(got.AllowCidrs, wantCidrs) {
		t.Errorf("AllowCidrs = %v, want %v", got.AllowCidrs, wantCidrs)
	}
}

func TestLoad_NetworkPolicyDefaultsToDeny(t *testing.T) {
	t.Parallel()
	dir := writeTempConfig(t, `version: 1
project: { name: tb }
`)
	got, err := Load(LoadOptions{StartDir: dir})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.NetworkPolicy != "deny" {
		t.Errorf("NetworkPolicy = %q, want deny", got.NetworkPolicy)
	}
}

func TestLoad_NetworkPolicyExplicit(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, policy string
	}{
		{"deny", "deny"},
		{"allow", "allow"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := writeTempConfig(t, `version: 1
project: { name: tb }
network:
  default_policy: `+tc.policy+`
`)
			got, err := Load(LoadOptions{StartDir: dir})
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			if got.NetworkPolicy != tc.policy {
				t.Errorf("NetworkPolicy = %q, want %q", got.NetworkPolicy, tc.policy)
			}
		})
	}
}

func TestLoad_NetworkPolicyRejectsUnknown(t *testing.T) {
	t.Parallel()
	dir := writeTempConfig(t, `version: 1
project: { name: tb }
network:
  default_policy: permissive
`)
	_, err := Load(LoadOptions{StartDir: dir})
	if err == nil || !strings.Contains(err.Error(), "network.default_policy") {
		t.Fatalf("expected default_policy validation error, got %v", err)
	}
}

func TestLoad_InitScriptComposition(t *testing.T) {
	t.Parallel()
	dir := writeTempConfig(t, `version: 1
project:
  name: tb
env:
  JAVA_HOME: /usr/lib/jvm/java-21-openjdk-amd64
  PATH_APPEND: $JAVA_HOME/bin
init_script: |
  export no_proxy="${no_proxy},172.17.0.1"
`)
	got, err := Load(LoadOptions{StartDir: dir})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	// env values are single-quoted so injection via $(cmd), backticks, or
	// newlines can't escape the `export KEY=...` line. Shell expansion is
	// intentionally off for env values — users who want it go to init_script.
	want := "export JAVA_HOME='/usr/lib/jvm/java-21-openjdk-amd64'\n" +
		"export PATH_APPEND='$JAVA_HOME/bin'\n" +
		"export no_proxy=\"${no_proxy},172.17.0.1\"\n"
	if got.InitScript != want {
		t.Errorf("InitScript mismatch\n got: %q\nwant: %q", got.InitScript, want)
	}
}

func TestLoad_RejectsInjections(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		yaml string
		msg  string
	}{
		{
			name: "newline in project.name",
			yaml: "version: 1\nproject: { name: \"tb\\nRUN evil\" }\n",
			msg:  "project.name",
		},
		{
			name: "shell metachars in env value",
			yaml: "version: 1\nproject: { name: tb }\nenv: { X: \"a\\nRUN bad\" }\n",
			msg:  "newline or control",
		},
		{
			name: "apt package with space",
			yaml: "version: 1\nproject: { name: tb }\napt_packages: [\"pkg; rm -rf /\"]\n",
			msg:  "apt_packages",
		},
		{
			name: "plugin url is ssh, not https",
			yaml: `version: 1
project: { name: tb }
agents:
  claude:
    plugins:
      enabled: true
      repos:
        - { url: "git@github.com:x/y.git", path: y }
`,
			msg: "https",
		},
		{
			name: "plugin path with ..",
			yaml: `version: 1
project: { name: tb }
agents:
  claude:
    plugins:
      enabled: true
      repos:
        - { url: "https://github.com/x/y.git", path: "y/../../etc" }
`,
			msg: "'..'",
		},
		{
			name: "base_image with newline",
			yaml: `version: 1
project: { name: tb }
agents:
  claude:
    base_image: "img\nRUN evil"
`,
			msg: "base_image",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := writeTempConfig(t, tc.yaml)
			_, err := Load(LoadOptions{StartDir: dir})
			if err == nil {
				t.Fatalf("expected validation error for %s; got nil", tc.name)
			}
			if !strings.Contains(err.Error(), tc.msg) {
				t.Errorf("error %q did not mention %q", err.Error(), tc.msg)
			}
		})
	}
}

func TestLoad_DiscoversUpward(t *testing.T) {
	t.Parallel()
	root := writeTempConfig(t, `version: 1
project:
  name: tb
`)
	nested := filepath.Join(root, "a", "b", "c")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := Load(LoadOptions{StartDir: nested})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.ConfigRoot != root {
		t.Errorf("ConfigRoot = %q, want %q", got.ConfigRoot, root)
	}
}

func TestLoad_ExplicitConfigPath(t *testing.T) {
	t.Parallel()
	dir := writeTempConfig(t, `version: 1
project: { name: tb }
`)
	explicit := filepath.Join(dir, ".kennel.yaml")
	// Start from a completely different directory so upward search would fail.
	otherDir := t.TempDir()
	got, err := Load(LoadOptions{StartDir: otherDir, ConfigPath: explicit})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.ConfigPath != explicit {
		t.Errorf("ConfigPath = %q, want %q", got.ConfigPath, explicit)
	}
}

func TestLoad_MissingConfigIsClearError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	_, err := Load(LoadOptions{StartDir: dir})
	if err == nil || !strings.Contains(err.Error(), "no .kennel.yaml found") {
		t.Fatalf("expected discovery error, got %v", err)
	}
}

func TestLoad_DockerfileExtraResolved(t *testing.T) {
	t.Parallel()
	dir := writeTempConfig(t, `version: 1
project: { name: tb }
dockerfile_extra: extra.Dockerfile
`)
	if err := os.WriteFile(filepath.Join(dir, "extra.Dockerfile"), []byte("RUN true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := Load(LoadOptions{StartDir: dir})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	want := filepath.Join(dir, "extra.Dockerfile")
	if got.DockerfileExtra != want {
		t.Errorf("DockerfileExtra = %q, want %q", got.DockerfileExtra, want)
	}
}

func TestLoad_DockerfileExtraMissingIsError(t *testing.T) {
	t.Parallel()
	dir := writeTempConfig(t, `version: 1
project: { name: tb }
dockerfile_extra: missing.Dockerfile
`)
	_, err := Load(LoadOptions{StartDir: dir})
	if err == nil {
		t.Fatal("expected error for missing dockerfile_extra")
	}
}
