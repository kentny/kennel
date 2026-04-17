package cli

import (
	"github.com/kentny/kennel/internal/config"
)

// loadResolved is the one place every docker-facing subcommand goes to turn
// the --config / --agent global flags into a Resolved. Keeping it here means
// subcommand files stay focused on their own logic.
//
// envNum is passed through for `kennel env *` commands; empty for
// single-sandbox commands so `{env}` is left unresolved (which, for those
// commands, would be a configuration error anyway — those commands never
// reference the {env} token).
func loadResolved(envNum string) (*config.Resolved, error) {
	return config.Load(config.LoadOptions{
		ConfigPath:    flagConfig,
		AgentOverride: flagAgent,
		EnvNumber:     envNum,
	})
}
