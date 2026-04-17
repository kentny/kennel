package cli

import (
	"github.com/spf13/cobra"

	"github.com/kentny/kennel/internal/sandbox"
)

var buildCmd = &cobra.Command{
	Use:   "build",
	Short: "Build the sandbox image from .kennel.yaml",
	RunE: func(cmd *cobra.Command, _ []string) error {
		r, err := loadResolved("")
		if err != nil {
			return err
		}
		return sandbox.Build(cmd.Context(), r)
	},
}

func init() {
	rootCmd.AddCommand(buildCmd)
}
