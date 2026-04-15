package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/lorem-dev/locksmith/internal/config"
	"github.com/lorem-dev/locksmith/internal/daemon"
	"github.com/lorem-dev/locksmith/internal/log"
)

// newServeCmd returns the `locksmith serve` command.
func newServeCmd(cfgFile *string) *cobra.Command {
	return &cobra.Command{
		Use:   "serve",
		Short: "Start the locksmith daemon",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfgPath := *cfgFile
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

			return d.Start()
		},
	}
}
