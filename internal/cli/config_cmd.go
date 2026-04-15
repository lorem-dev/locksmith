package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/lorem-dev/locksmith/internal/config"
)

// newConfigCmd returns the `locksmith config` command group.
func newConfigCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "config", Short: "Configuration management"}
	cmd.AddCommand(&cobra.Command{
		Use:   "check",
		Short: "Validate config file",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfgPath := cfgFile
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
	})
	return cmd
}
