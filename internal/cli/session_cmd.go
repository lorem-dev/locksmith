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
	cmd.AddCommand(newSessionStartCmd(), newSessionEndCmd(), newSessionListCmd(), newSessionEnsureCmd())
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
	var sessionIDPrefix string
	cmd := &cobra.Command{
		Use:   "end",
		Short: "End an agent session",
		RunE: func(cmd *cobra.Command, args []string) error {
			if sessionIDPrefix == "" {
				sessionIDPrefix = os.Getenv("LOCKSMITH_SESSION")
			}
			if sessionIDPrefix == "" {
				return fmt.Errorf("session ID prefix required: use --session or set LOCKSMITH_SESSION")
			}
			client, conn, err := dialDaemon("")
			if err != nil {
				return err
			}
			defer conn.Close()
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			if _, err := client.SessionEnd(ctx, &locksmithv1.SessionEndRequest{SessionIdPrefix: sessionIDPrefix}); err != nil {
				return fmt.Errorf("ending session: %w", err)
			}
			fmt.Println("session ended")
			return nil
		},
	}
	cmd.Flags().StringVar(&sessionIDPrefix, "session", "", "session ID prefix (default: $LOCKSMITH_SESSION)")
	return cmd
}

// newSessionEnsureCmd returns the `locksmith session ensure` command.
// It reuses an existing valid session from LOCKSMITH_SESSION or starts a new one.
// Exits non-zero if the daemon is not running.
// With --quiet, only the session ID is printed to stdout (for use in hook scripts).
func newSessionEnsureCmd() *cobra.Command {
	var quiet bool
	cmd := &cobra.Command{
		Use:   "ensure",
		Short: "Ensure a valid session exists, reusing or creating one",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, conn, err := dialDaemon("")
			if err != nil {
				return err
			}
			defer conn.Close()

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			// Try to reuse the session from environment.
			if existing := os.Getenv("LOCKSMITH_SESSION"); existing != "" {
				resp, err := client.SessionList(ctx, &locksmithv1.SessionListRequest{})
				if err == nil {
					for _, s := range resp.Sessions {
						if s.SessionId == existing {
							fmt.Fprintln(cmd.OutOrStdout(), existing)
							return nil
						}
					}
				}
				// Session not found or expired - fall through to create a new one.
			}

			resp, err := client.SessionStart(ctx, &locksmithv1.SessionStartRequest{})
			if err != nil {
				return fmt.Errorf("starting session: %w", err)
			}

			if quiet {
				fmt.Fprintln(cmd.OutOrStdout(), resp.SessionId)
			} else {
				fmt.Fprintf(cmd.ErrOrStderr(),
					"locksmith: session started (expires %s)\n  export LOCKSMITH_SESSION=%s\n",
					resp.ExpiresAt, resp.SessionId)
				fmt.Fprintln(cmd.OutOrStdout(), resp.SessionId)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&quiet, "quiet", false, "print only the session ID (for use in scripts)")
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
