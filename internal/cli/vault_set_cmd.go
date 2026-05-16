package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/term"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	locksmithv1 "github.com/lorem-dev/locksmith/gen/proto/locksmith/v1"
)

// newVaultSetCmd returns the `locksmith vault set <alias>` command.
func newVaultSetCmd() *cobra.Command {
	var (
		force    bool
		fromFile string
	)
	cmd := &cobra.Command{
		Use:   "set <alias>",
		Short: "Write a secret to a vault by key alias",
		Long: `Write a secret into the vault referenced by <alias> in config.yaml.

The secret value is read from --from-file, from piped stdin, or from an
interactive TTY prompt (with confirmation). The vault plugin must
support write; today that means the keychain or gopass plugin. Other
vault types return an "does not support write" error.

By default, set refuses to overwrite an existing item. Pass --force to
overwrite.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			alias := args[0]

			client, conn, err := dialDaemon()
			if err != nil {
				return err
			}
			defer conn.Close() //nolint:errcheck // gRPC connection close; error not actionable in defer

			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()

			sessionID, err := ensureSession(ctx, client)
			if err != nil {
				return err
			}

			if !force {
				resp, kerr := client.KeyExists(ctx, &locksmithv1.KeyExistsRequest{
					SessionId: sessionID,
					KeyAlias:  alias,
				})
				if kerr != nil {
					if status.Code(kerr) != codes.Unimplemented {
						return kerr
					}
					// Plugin does not support an efficient existence
					// probe; skip the strict check and let the write
					// proceed (the plugin may still refuse).
				} else if resp.Exists {
					return fmt.Errorf("%s already has a secret; pass --force to overwrite", alias)
				}
			}

			secret, err := readSecret(cmd, fromFile)
			if err != nil {
				return err
			}
			if len(secret) == 0 {
				return errors.New("secret is empty")
			}

			if _, err := client.SetSecret(ctx, &locksmithv1.SetSecretRequest{
				SessionId: sessionID,
				KeyAlias:  alias,
				Secret:    secret,
				Force:     force,
			}); err != nil {
				if status.Code(err) == codes.Unimplemented {
					return fmt.Errorf("%s does not support secret write (vault type is read-only)", alias)
				}
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "stored %s\n", alias) //nolint:errcheck // writing to stdout
			return nil
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "overwrite an existing item without prompting")
	cmd.Flags().StringVar(&fromFile, "from-file", "", "read the secret value from this file (trailing newline trimmed)")
	return cmd
}

// readSecret returns the secret bytes from --from-file, piped stdin, or
// an interactive TTY prompt with confirmation.
func readSecret(cmd *cobra.Command, fromFile string) ([]byte, error) {
	if fromFile != "" {
		raw, err := os.ReadFile(fromFile) //nolint:gosec // G304: user-provided path is intentional for --from-file
		if err != nil {
			return nil, fmt.Errorf("reading --from-file %s: %w", fromFile, err)
		}
		return bytes.TrimRight(raw, "\n"), nil
	}

	stdin := cmd.InOrStdin()
	if f, ok := stdin.(*os.File); ok {
		fd := int(f.Fd()) //nolint:gosec // G115: fd value fits in int on supported platforms
		if term.IsTerminal(fd) {
			return promptSecret(cmd, fd)
		}
	}

	raw, err := io.ReadAll(stdin)
	if err != nil {
		return nil, fmt.Errorf("reading stdin: %w", err)
	}
	return bytes.TrimRight(raw, "\n"), nil
}

// promptSecret reads a secret twice from the given TTY file descriptor
// and returns it if both inputs match.
func promptSecret(cmd *cobra.Command, fd int) ([]byte, error) {
	fmt.Fprint(cmd.OutOrStdout(), "Secret: ") //nolint:errcheck // writing to stdout
	first, err := term.ReadPassword(fd)
	fmt.Fprintln(cmd.OutOrStdout()) //nolint:errcheck // writing to stdout
	if err != nil {
		return nil, err
	}
	fmt.Fprint(cmd.OutOrStdout(), "Confirm: ") //nolint:errcheck // writing to stdout
	second, err := term.ReadPassword(fd)
	fmt.Fprintln(cmd.OutOrStdout()) //nolint:errcheck // writing to stdout
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(first, second) {
		return nil, errors.New("secrets do not match")
	}
	return first, nil
}

// ensureSession returns LOCKSMITH_SESSION from env if set, otherwise
// starts a new session via the daemon and returns its ID.
func ensureSession(ctx context.Context, client locksmithv1.LocksmithServiceClient) (string, error) {
	if id := os.Getenv("LOCKSMITH_SESSION"); id != "" {
		return id, nil
	}
	resp, err := client.SessionStart(ctx, &locksmithv1.SessionStartRequest{})
	if err != nil {
		return "", err
	}
	return resp.SessionId, nil
}
