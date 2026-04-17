package cli

import (
	"github.com/spf13/cobra"

	"github.com/kentny/kennel/internal/sandbox"
)

var rmCmd = &cobra.Command{
	Use:   "rm",
	Short: "Delete the sandbox (image stays intact)",
	RunE: func(cmd *cobra.Command, _ []string) error {
		r, err := loadResolved("")
		if err != nil {
			return err
		}
		return sandbox.Rm(cmd.Context(), r)
	},
}

func init() {
	rootCmd.AddCommand(rmCmd)
}
