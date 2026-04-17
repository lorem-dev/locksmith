package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/lorem-dev/locksmith/internal/config"
	"github.com/lorem-dev/locksmith/internal/initflow"
)

// newConfigCmd returns the `locksmith config` command group.
func newConfigCmd(cfgFile *string) *cobra.Command {
	cmd := &cobra.Command{Use: "config", Short: "Configuration management"}
	cmd.AddCommand(newConfigCheckCmd(cfgFile))
	cmd.AddCommand(newConfigPinentryCmd())
	return cmd
}

func newConfigCheckCmd(cfgFile *string) *cobra.Command {
	return &cobra.Command{
		Use:   "check",
		Short: "Validate config file",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfgPath := *cfgFile
			if cfgPath == "" {
				cfgPath = config.DefaultConfigPath()
			}
			cfg, err := config.Load(cfgPath)
			if err != nil {
				return fmt.Errorf("config error: %w", err)
			}
			fmt.Printf("config OK: %s\n  vaults: %d\n  keys:   %d\n  ttl:    %s\n",
				cfgPath, len(cfg.Vaults), len(cfg.Keys), cfg.Defaults.SessionTTL)
			return nil
		},
	}
}

func newConfigPinentryCmd() *cobra.Command {
	var auto, noTUI bool
	cmd := &cobra.Command{
		Use:   "pinentry",
		Short: "Configure locksmith-pinentry as the gpg-agent pinentry program",
		Long: `Configure locksmith-pinentry as the pinentry program for gpg-agent.

Use this after running 'locksmith init --auto', or any time you want to
(re-)configure GPG passphrase prompts independently of init.

Requires locksmith-pinentry to be installed (run 'make init' once after cloning).`,
		RunE: func(cmd *cobra.Command, args []string) error {
			result, err := initflow.RunConfigPinentry(initflow.ConfigPinentryOptions{
				Auto:  auto,
				NoTUI: noTUI,
			})
			if err != nil {
				return err
			}
			if !result.Configured {
				return nil
			}
			fmt.Printf("  locksmith-pinentry found at %s\n", result.PinentryPath)
			if result.Replaced != "" {
				fmt.Printf("  Previous pinentry-program (%s) commented out.\n", result.Replaced)
			}
			fmt.Printf("  Configured: pinentry-program set to %s\n", result.PinentryPath)
			return nil
		},
	}
	cmd.Flags().BoolVar(&auto, "auto", false, "configure without prompting")
	cmd.Flags().BoolVar(&noTUI, "no-tui", false, "use plain-text prompts (also auto-enabled when TERM=dumb or non-TTY stdin)")
	return cmd
}
