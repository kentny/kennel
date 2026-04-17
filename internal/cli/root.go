// Package cli wires cobra subcommands together. The root command owns the
// two global flags (--config, --agent) so every subcommand sees them without
// re-declaring. Subcommand files add themselves via init() so main.go stays a
// one-liner.
package cli

import (
	"github.com/spf13/cobra"

	"github.com/kentny/kennel/internal/version"
)

// Persistent flag values shared across subcommands. Package-level vars are
// the simplest wiring with cobra — alternative is per-command flag sets, but
// these two are truly global (every command that talks to docker / config
// reads them).
var (
	flagConfig string
	flagAgent  string
)

// rootCmd is the `kennel` entry point. Subcommands attach during init() in
// their own files, so ordering in Execute is irrelevant.
var rootCmd = &cobra.Command{
	Use:     "kennel",
	Short:   "Portable sandbox manager for AI coding agents",
	Long:    `kennel wraps Docker Desktop's "docker sandbox" CLI with config-driven Dockerfile generation, deny-by-default network policies, and git worktree multi-environment support. See https://github.com/kentny/kennel.`,
	Version: version.String(),
	// Runtime errors (docker failed, config missing, ...) already read well
	// on their own; we don't want cobra to reprint the usage block for them.
	SilenceUsage: true,
	// SilenceErrors intentionally left false — cobra prints "Error: ..." to
	// stderr, which is the signal users need. An earlier revision set this to
	// true on the assumption main.go would reprint via tui.Error; that never
	// happened and `kennel env create` with zero args produced silent output.
}

func init() {
	rootCmd.PersistentFlags().StringVar(&flagConfig, "config", "", "path to .kennel.yaml (default: search upward from CWD)")
	rootCmd.PersistentFlags().StringVar(&flagAgent, "agent", "", "override default_agent from config (e.g. claude, codex)")

	// `kennel version` as an explicit subcommand in addition to `kennel --version`
	// since the bash implementation shipped it and muscle memory matters.
	rootCmd.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "Print version",
		Run: func(cmd *cobra.Command, _ []string) {
			cmd.Println(version.String())
		},
	})
}

// Execute runs the root command. Called once from cmd/kennel/main.go.
func Execute() error {
	return rootCmd.Execute()
}
