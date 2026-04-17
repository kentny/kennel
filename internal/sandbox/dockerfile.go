// Package sandbox owns the Dockerfile generation + docker CLI wrappers that
// power `kennel build / run / bash / rm`. Dockerfile output is produced by
// executing an embedded text/template against the Resolved config, so changes
// to the template are reviewable as text diffs and easy to golden-test.
package sandbox

import (
	"bytes"
	_ "embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"text/template"

	"github.com/kentny/kennel/internal/config"
	"github.com/kentny/kennel/internal/version"
)

//go:embed dockerfile.tmpl
var dockerfileTmpl string

// BuildContextInputs is what the template needs. Kept separate from the
// Resolved config so the template view stays explicit.
type BuildContextInputs struct {
	Version             string
	ConfigPath          string
	BaseImage           string
	Agent               string
	InstallPlugins      bool
	AptPackages         []string
	PluginRepos         []config.PluginRepo
	DockerfileExtra     string // verbatim file contents
	DockerfileExtraPath string // path displayed in the inserted comment
}

// WriteBuildContext creates a temp directory containing `Dockerfile` and
// `init.sh`, ready for `docker build`. The caller is responsible for calling
// the returned cleanup func (typically via defer).
//
// We generate into a tempdir — not inside the user's repo — so a botched
// build never leaves stray files or gets committed accidentally. Docker
// needs a real directory path for its build context, not a tarball stream.
func WriteBuildContext(r *config.Resolved) (dir string, cleanup func(), err error) {
	dir, err = os.MkdirTemp("", "kennel-build-*")
	if err != nil {
		return "", nil, err
	}
	cleanup = func() { _ = os.RemoveAll(dir) }

	if err := writeInitScript(dir, r.InitScript); err != nil {
		cleanup()
		return "", nil, err
	}

	dfContent, err := RenderDockerfile(r)
	if err != nil {
		cleanup()
		return "", nil, err
	}
	if err := os.WriteFile(filepath.Join(dir, "Dockerfile"), []byte(dfContent), 0o644); err != nil {
		cleanup()
		return "", nil, err
	}
	return dir, cleanup, nil
}

// RenderDockerfile returns the generated Dockerfile as a string. Exposed
// separately from WriteBuildContext so golden tests can diff the output
// without touching the filesystem.
func RenderDockerfile(r *config.Resolved) (string, error) {
	inputs := BuildContextInputs{
		Version:        version.Version,
		ConfigPath:     r.ConfigPath,
		BaseImage:      r.BaseImage,
		Agent:          r.Agent,
		InstallPlugins: r.InstallPlugins,
		AptPackages:    r.AptPackages,
		PluginRepos:    r.PluginRepos,
	}
	if r.DockerfileExtra != "" {
		content, err := os.ReadFile(r.DockerfileExtra)
		if err != nil {
			return "", fmt.Errorf("reading dockerfile_extra %q: %w", r.DockerfileExtra, err)
		}
		inputs.DockerfileExtra = string(content)
		inputs.DockerfileExtraPath = r.DockerfileExtra
	}

	tmpl, err := template.New("dockerfile").Funcs(template.FuncMap{
		// sub is only used by {{$last := sub (len .PluginRepos) 1}} to
		// mark the final repo so we emit the trailing `&& \` correctly.
		"sub": func(a, b int) int { return a - b },
	}).Parse(dockerfileTmpl)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, inputs); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// writeInitScript emits the persistent init script that will be copied to
// /etc/sandbox-persistent.sh inside the image. We always include the banner
// print lines so users can verify the script ran; the config-supplied
// additions sit between them.
func writeInitScript(dir, body string) error {
	content := "#!/bin/sh\n" +
		"echo \"--- kennel persistent init script ---\"\n"
	if body != "" {
		content += body
		if body[len(body)-1] != '\n' {
			content += "\n"
		}
	}
	content += "echo \"--- kennel persistent init script end ---\"\n"
	return os.WriteFile(filepath.Join(dir, "init.sh"), []byte(content), 0o755)
}

// ensureWritable is a sanity helper for deferred-error cases — exported so
// cli/ tests can check the build dir exists and is usable before shelling
// out to docker.
func ensureWritable(dir string) error {
	info, err := os.Stat(dir)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("%s is not a directory", dir)
	}
	if info.Mode().Perm()&fs.FileMode(0o200) == 0 {
		return fmt.Errorf("%s is not writable", dir)
	}
	return nil
}
