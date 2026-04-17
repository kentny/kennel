// Command kennel is a portable sandbox manager for AI coding agents.
// It wraps Docker Desktop's `docker sandbox` CLI and layers config-driven
// Dockerfile generation, host-based network policies, and git worktree
// multi-environment support on top. See README.md.
package main

import (
	"os"

	"github.com/kentny/kennel/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		// Cobra already printed the error (SilenceErrors is false); we just
		// surface a non-zero exit code so shell pipelines / Make targets can
		// react. No extra newline — cobra already ended with one.
		os.Exit(1)
	}
}
