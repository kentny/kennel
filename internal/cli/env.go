package cli

import (
	"github.com/spf13/cobra"

	"github.com/kentny/kennel/internal/env"
)

var envCmd = &cobra.Command{
	Use:   "env",
	Short: "Manage git worktree + sandbox pairs, indexed by ENV number",
}

// Flag for `env create`. Defined as package-level so the closure in the
// cobra.Command sees the most-recent value after flag parsing.
var envCreateBaseFlag string

var envCreateCmd = &cobra.Command{
	Use:   "create <N> <branch>",
	Short: "Create worktree + sandbox for env N",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		r, err := loadResolved(args[0])
		if err != nil {
			return err
		}
		return env.Create(cmd.Context(), r, args[0], args[1], envCreateBaseFlag)
	},
}

var envStartCmd = &cobra.Command{
	Use:   "start <N> [-- extra-docker-sandbox-run-args]",
	Short: "Launch env N's sandbox",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		r, err := loadResolved(args[0])
		if err != nil {
			return err
		}
		return env.Start(cmd.Context(), r, args[0], args[1:])
	},
}

var envStopCmd = &cobra.Command{
	Use:   "stop <N>",
	Short: "Stop env N's sandbox",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		r, err := loadResolved(args[0])
		if err != nil {
			return err
		}
		return env.Stop(cmd.Context(), r, args[0])
	},
}

var envBashCmd = &cobra.Command{
	Use:   "bash <N>",
	Short: "Open bash in env N's sandbox",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		r, err := loadResolved(args[0])
		if err != nil {
			return err
		}
		return env.Bash(cmd.Context(), r, args[0])
	},
}

var envRebuildCmd = &cobra.Command{
	Use:   "rebuild <N>",
	Short: "Recreate env N's sandbox from the current template (keep worktree)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		r, err := loadResolved(args[0])
		if err != nil {
			return err
		}
		return env.Rebuild(cmd.Context(), r, args[0])
	},
}

var envDestroyCmd = &cobra.Command{
	Use:   "destroy <N>",
	Short: "Remove worktree AND sandbox for env N",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		r, err := loadResolved(args[0])
		if err != nil {
			return err
		}
		return env.Destroy(cmd.Context(), r, args[0])
	},
}

var envListCmd = &cobra.Command{
	Use:   "list",
	Short: "Show all managed and other worktrees",
	RunE: func(cmd *cobra.Command, _ []string) error {
		r, err := loadResolved("")
		if err != nil {
			return err
		}
		return env.List(cmd.Context(), r)
	},
}

func init() {
	envCreateCmd.Flags().StringVarP(&envCreateBaseFlag, "base", "b", "", "base branch when creating a new branch")

	envCmd.AddCommand(envCreateCmd)
	envCmd.AddCommand(envStartCmd)
	envCmd.AddCommand(envStopCmd)
	envCmd.AddCommand(envBashCmd)
	envCmd.AddCommand(envRebuildCmd)
	envCmd.AddCommand(envDestroyCmd)
	envCmd.AddCommand(envListCmd)
	rootCmd.AddCommand(envCmd)
}
