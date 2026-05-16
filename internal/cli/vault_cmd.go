package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	locksmithv1 "github.com/lorem-dev/locksmith/gen/proto/locksmith/v1"
)

// newVaultCmd returns the `locksmith vault` command group.
func newVaultCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "vault", Short: "Manage vault providers"}
	cmd.AddCommand(
		&cobra.Command{
			Use:   "list",
			Short: "List available vault providers",
			RunE: func(cmd *cobra.Command, args []string) error {
				client, conn, err := dialDaemon()
				if err != nil {
					return err
				}
				defer conn.Close() //nolint:errcheck // gRPC connection close; error not actionable in defer
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				resp, err := client.VaultList(ctx, &locksmithv1.VaultListRequest{})
				if err != nil {
					return fmt.Errorf("listing vaults: %w", err)
				}
				if len(resp.Vaults) == 0 {
					fmt.Println("no vault providers loaded")
					return nil
				}
				out, err := json.MarshalIndent(resp.Vaults, "", "  ")
				if err != nil {
					return fmt.Errorf("marshaling vaults: %w", err)
				}
				fmt.Println(string(out))
				return nil
			},
		},
		&cobra.Command{
			Use:   "health",
			Short: "Check health of vault providers",
			RunE: func(cmd *cobra.Command, args []string) error {
				client, conn, err := dialDaemon()
				if err != nil {
					return err
				}
				defer conn.Close() //nolint:errcheck // gRPC connection close; error not actionable in defer
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()
				resp, err := client.VaultHealth(ctx, &locksmithv1.VaultHealthRequest{})
				if err != nil {
					return fmt.Errorf("checking vault health: %w", err)
				}
				for _, v := range resp.Vaults {
					status := "UNAVAILABLE"
					if v.Available {
						status = "OK"
					}
					fmt.Printf("%-20s %s  %s\n", v.Name, status, v.Message)
					for _, w := range v.CompatWarnings {
						fmt.Printf("  ! %s\n", w)
					}
				}
				return nil
			},
		},
	)
	cmd.AddCommand(newVaultSetCmd())
	return cmd
}
