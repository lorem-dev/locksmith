package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/lorem-dev/locksmith/internal/config"
	"github.com/lorem-dev/locksmith/internal/daemon"
	"github.com/lorem-dev/locksmith/internal/log"
	sdklog "github.com/lorem-dev/locksmith/sdk/log"
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

			w, err := sdklog.NewLogWriter(sdklog.LogConfig{
				Level:  cfg.Logging.Level,
				Format: cfg.Logging.Format,
				File:   cfg.Logging.File,
			})
			if err != nil {
				return fmt.Errorf("log setup: %w", err)
			}

			if sdklog.IsDebug() {
				fmt.Fprintln(os.Stderr,
					"WARNING: debug logging is enabled - session IDs will be written to logs "+
						"in plaintext. Do not use debug level in production. "+
						"See docs/security/debug-logging.md")
			}

			log.Init(w, cfg.Logging.Level, cfg.Logging.Format)

			d := daemon.New(cfg)
			go d.WaitForShutdown()

			return d.Start()
		},
	}
}
