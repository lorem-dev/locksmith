package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/lorem-dev/locksmith/internal/config"
	"github.com/lorem-dev/locksmith/internal/daemon"
	"github.com/lorem-dev/locksmith/internal/log"
)

// newServeCmd returns the `locksmith serve` command.
func newServeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "serve",
		Short: "Start the locksmith daemon",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfgPath := cfgFile
			if cfgPath == "" {
				cfgPath = config.DefaultConfigPath()
			}
			cfg, err := config.Load(cfgPath)
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}
			log.Init(log.Config{Level: cfg.Logging.Level, Format: cfg.Logging.Format})

			d := daemon.New(cfg)
			go d.WaitForShutdown()

			if err := d.Start(); err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				os.Exit(1)
			}
			return nil
		},
	}
}
