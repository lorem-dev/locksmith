package cli_test

import (
	"testing"

	"github.com/lorem-dev/locksmith/internal/cli"
)

// TestGet_DefaultSocket covers the else branch in dialDaemon for the get command
// when LOCKSMITH_SOCKET is not set, so the default socket path is used.
func TestGet_DefaultSocket(t *testing.T) {
	t.Setenv("LOCKSMITH_SOCKET", "")
	t.Setenv("LOCKSMITH_SESSION", "test-session")
	root := cli.NewRootCmd()
	root.SetArgs([]string{"get", "--key", "mykey"})
	// Expected to fail (no daemon), but covers the default socket path branch.
	_ = root.Execute()
}
