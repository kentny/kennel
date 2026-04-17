package cli

import (
	"github.com/spf13/cobra"

	"github.com/kentny/kennel/internal/sandbox"
)

var bashCmd = &cobra.Command{
	Use:   "bash",
	Short: "Open an interactive bash shell inside the running sandbox",
	RunE: func(cmd *cobra.Command, _ []string) error {
		r, err := loadResolved("")
		if err != nil {
			return err
		}
		return sandbox.Bash(cmd.Context(), r)
	},
}

func init() {
	rootCmd.AddCommand(bashCmd)
}
