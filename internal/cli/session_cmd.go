package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	locksmithv1 "github.com/lorem-dev/locksmith/gen/proto/locksmith/v1"
)

// newSessionCmd returns the `locksmith session` command group.
func newSessionCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "session", Short: "Manage agent sessions"}
	cmd.AddCommand(newSessionStartCmd(), newSessionEndCmd(), newSessionListCmd())
	return cmd
}

// newSessionStartCmd returns the `locksmith session start` command.
func newSessionStartCmd() *cobra.Command {
	var ttl string
	var keys []string
	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start a new agent session",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, conn, err := dialDaemon("")
			if err != nil {
				return err
			}
			defer conn.Close()
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			resp, err := client.SessionStart(ctx, &locksmithv1.SessionStartRequest{
				Ttl: ttl, AllowedKeys: keys,
			})
			if err != nil {
				return fmt.Errorf("starting session: %w", err)
			}
			out, _ := json.MarshalIndent(map[string]string{
				"session_id": resp.SessionId,
				"expires_at": resp.ExpiresAt,
			}, "", "  ")
			fmt.Println(string(out))
			return nil
		},
	}
	cmd.Flags().StringVar(&ttl, "ttl", "", "session TTL (default: from config)")
	cmd.Flags().StringSliceVar(&keys, "keys", nil, "restrict to specific key aliases")
	return cmd
}

// newSessionEndCmd returns the `locksmith session end` command.
func newSessionEndCmd() *cobra.Command {
	var sessionID string
	cmd := &cobra.Command{
		Use:   "end",
		Short: "End an agent session",
		RunE: func(cmd *cobra.Command, args []string) error {
			if sessionID == "" {
				sessionID = os.Getenv("LOCKSMITH_SESSION")
			}
			if sessionID == "" {
				return fmt.Errorf("session ID required: use --session or set LOCKSMITH_SESSION")
			}
			client, conn, err := dialDaemon("")
			if err != nil {
				return err
			}
			defer conn.Close()
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			if _, err := client.SessionEnd(ctx, &locksmithv1.SessionEndRequest{SessionId: sessionID}); err != nil {
				return fmt.Errorf("ending session: %w", err)
			}
			fmt.Println("session ended")
			return nil
		},
	}
	cmd.Flags().StringVar(&sessionID, "session", "", "session ID (default: $LOCKSMITH_SESSION)")
	return cmd
}

// newSessionListCmd returns the `locksmith session list` command.
func newSessionListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List active sessions",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, conn, err := dialDaemon("")
			if err != nil {
				return err
			}
			defer conn.Close()
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			resp, err := client.SessionList(ctx, &locksmithv1.SessionListRequest{})
			if err != nil {
				return fmt.Errorf("listing sessions: %w", err)
			}
			if len(resp.Sessions) == 0 {
				fmt.Println("no active sessions")
				return nil
			}
			out, _ := json.MarshalIndent(resp.Sessions, "", "  ")
			fmt.Println(string(out))
			return nil
		},
	}
}
