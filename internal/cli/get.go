package cli

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	locksmithv1 "github.com/lorem-dev/locksmith/gen/proto/locksmith/v1"
)

// newGetCmd returns the `locksmith get` command.
func newGetCmd() *cobra.Command {
	var keyAlias, vaultName, path string

	cmd := &cobra.Command{
		Use:   "get",
		Short: "Retrieve a secret from a vault provider",
		RunE: func(cmd *cobra.Command, args []string) error {
			sessionID := os.Getenv("LOCKSMITH_SESSION")
			if sessionID == "" {
				return fmt.Errorf("LOCKSMITH_SESSION not set — run 'locksmith session start' first")
			}

			client, conn, err := dialDaemon("")
			if err != nil {
				return err
			}
			defer conn.Close()

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			resp, err := client.GetSecret(ctx, &locksmithv1.GetSecretRequest{
				SessionId: sessionID,
				KeyAlias:  keyAlias,
				VaultName: vaultName,
				Path:      path,
			})
			if err != nil {
				return fmt.Errorf("getting secret: %w", err)
			}

			fmt.Print(string(resp.Secret))
			return nil
		},
	}

	cmd.Flags().StringVar(&keyAlias, "key", "", "key alias from config")
	cmd.Flags().StringVar(&vaultName, "vault", "", "vault name (fallback, requires --path)")
	cmd.Flags().StringVar(&path, "path", "", "secret path in vault (fallback, requires --vault)")
	return cmd
}
