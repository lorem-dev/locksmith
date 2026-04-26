package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	locksmithv1 "github.com/lorem-dev/locksmith/gen/proto/locksmith/v1"
)

// newReloadCmd returns the `locksmith reload` command.
func newReloadCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "reload",
		Short: "Reload the daemon configuration without restarting",
		Long: `Reload re-reads the config file and applies changes to vaults, keys, and
daemon parameters without stopping the daemon. Active sessions are preserved.

Equivalent to sending SIGHUP to the daemon process.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			client, conn, err := dialDaemon()
			if err != nil {
				return err
			}
			defer conn.Close() //nolint:errcheck

			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()

			resp, err := client.ReloadConfig(ctx, &locksmithv1.ReloadConfigRequest{})
			if err != nil {
				return fmt.Errorf("reload failed: %w", err)
			}
			fmt.Fprintln(cmd.OutOrStdout(), resp.Message) //nolint:errcheck // writing to stdout
			return nil
		},
	}
}
