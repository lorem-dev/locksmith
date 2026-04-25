// Package cli implements all locksmith subcommands via cobra.
package cli

import (
	"fmt"
	"os"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	locksmithv1 "github.com/lorem-dev/locksmith/gen/proto/locksmith/v1"
	"github.com/lorem-dev/locksmith/internal/config"
)

// dialDaemon connects to the locksmith daemon Unix socket and returns a client.
// Returns an error with a helpful hint if the daemon is not running.
// The LOCKSMITH_SOCKET environment variable overrides the default socket path.
func dialDaemon() (locksmithv1.LocksmithServiceClient, *grpc.ClientConn, error) {
	socketPath := config.ExpandPath("~/.config/locksmith/locksmith.sock")
	if envSocket := os.Getenv("LOCKSMITH_SOCKET"); envSocket != "" {
		socketPath = envSocket
	}
	conn, err := grpc.NewClient(
		"unix://"+socketPath,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, nil, fmt.Errorf(
			"cannot connect to locksmith daemon at %s: %w\nHint: run 'locksmith serve' first",
			socketPath, err,
		)
	}
	return locksmithv1.NewLocksmithServiceClient(conn), conn, nil
}
