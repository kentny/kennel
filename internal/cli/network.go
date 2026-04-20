package cli

import (
	"github.com/spf13/cobra"

	"github.com/kentny/kennel/internal/sandbox"
)

var networkCmd = &cobra.Command{
	Use:   "network",
	Short: "Network policy operations",
}

var networkApplyEnv string

var networkApplyCmd = &cobra.Command{
	Use:   "apply",
	Short: "Reapply the configured network policy to the sandbox",
	Long: `Reapply the network policy (deny-by-default allow-list, or full
outbound when default_policy is 'allow') to a running sandbox.

Use after editing .kennel.yaml's network section or agents.<name>.allow_hosts
to push the change without destroying and recreating the sandbox.`,
	RunE: func(cmd *cobra.Command, _ []string) error {
		r, err := loadResolved(networkApplyEnv)
		if err != nil {
			return err
		}
		target := r.SandboxPrefix
		if networkApplyEnv != "" {
			target = r.SandboxPrefix + "-" + networkApplyEnv
		}
		return sandbox.ApplyNetwork(cmd.Context(), r, target)
	},
}

func init() {
	networkApplyCmd.Flags().StringVar(&networkApplyEnv, "env", "", "target env N sandbox (omit for the single-sandbox name)")

	networkCmd.AddCommand(networkApplyCmd)
	rootCmd.AddCommand(networkCmd)
}
