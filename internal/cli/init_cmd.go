package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/lorem-dev/locksmith/internal/initflow"
)

// newInitCmd returns the `locksmith init` command - an interactive setup wizard
// that configures vaults, detects AI agents, and installs instructions/permissions.
func newInitCmd() *cobra.Command {
	var noTUI, auto, skipAgents bool
	var agentOnly string

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Interactive setup for locksmith",
		RunE: func(cmd *cobra.Command, args []string) error {
			result, err := initflow.RunInit(initflow.InitOptions{
				NoTUI:      noTUI,
				Auto:       auto,
				AgentOnly:  agentOnly,
				SkipAgents: skipAgents,
			})
			if err != nil {
				return err
			}
			msg := "\nSetup complete! Config: %s\n"
			if !result.ShellHookInstall && !result.ShellHookAlreadyPresent {
				msg += "Run 'locksmith serve' to start the daemon.\n"
			}
			fmt.Printf(msg, result.ConfigPath)
			return nil
		},
	}

	cmd.Flags().
		BoolVar(&noTUI, "no-tui", false, "use plain-text prompts (also auto-enabled when TERM=dumb or non-TTY stdin)")
	cmd.Flags().BoolVar(&auto, "auto", false, "auto-detect everything, apply defaults without prompts")
	cmd.Flags().StringVar(&agentOnly, "agent", "", "install for one specific agent only")
	cmd.Flags().BoolVar(&skipAgents, "skip-agents", false, "skip agent setup")
	return cmd
}
