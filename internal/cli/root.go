package cli

import "github.com/spf13/cobra"

// NewRootCmd builds the cobra root command with all subcommands registered.
func NewRootCmd() *cobra.Command {
	var cfgFile string

	root := &cobra.Command{
		Use:          "locksmith",
		Short:        "Secure secret middleware for AI agents",
		Long:         "Locksmith gives AI agents secure access to secrets from vault providers (macOS Keychain, gopass, etc.) with per-session caching and Touch ID support.",
		SilenceErrors: true,
		SilenceUsage:  true,
	}
	root.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default ~/.config/locksmith/config.yaml)")
	root.AddCommand(
		newServeCmd(&cfgFile),
		newGetCmd(),
		newSessionCmd(),
		newVaultCmd(),
		newConfigCmd(&cfgFile),
		newInitCmd(),
	)
	return root
}
