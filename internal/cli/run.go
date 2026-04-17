package cli

import (
	"github.com/spf13/cobra"

	"github.com/kentny/kennel/internal/sandbox"
)

var runCmd = &cobra.Command{
	Use:                "run [-- extra-docker-sandbox-run-args]",
	Short:              "Create the sandbox, apply network policy, and launch the agent",
	DisableFlagParsing: false, // we want --agent / --config to work
	RunE: func(cmd *cobra.Command, args []string) error {
		r, err := loadResolved("")
		if err != nil {
			return err
		}
		// Default workspace is the directory containing .kennel.yaml — same
		// semantics as the bash implementation.
		workspace := r.ConfigRoot
		return sandbox.Run(cmd.Context(), r, workspace, args)
	},
}

func init() {
	rootCmd.AddCommand(runCmd)
}
