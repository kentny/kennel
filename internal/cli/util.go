package cli

import (
	"bytes"
	"os/exec"
)

// execOutput executes name with args and returns the combined stdout output.
// Used for small lookups (git, docker queries) where we only need the text;
// for long-running / streaming commands use sandbox.docker directly. Named
// execOutput rather than runCmd to avoid colliding with run.go's `runCmd`
// cobra.Command variable.
func execOutput(name string, args ...string) (string, error) {
	var stdout bytes.Buffer
	cmd := exec.Command(name, args...)
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		return "", err
	}
	return stdout.String(), nil
}
