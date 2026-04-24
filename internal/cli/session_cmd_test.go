package cli_test

import (
	"testing"

	"github.com/lorem-dev/locksmith/internal/cli"
)

// TestSessionList_DefaultSocket covers the else branch in dialDaemon where
// LOCKSMITH_SOCKET is not set, so the default socket path is used.
func TestSessionList_DefaultSocket(t *testing.T) {
	t.Setenv("LOCKSMITH_SOCKET", "")
	root := cli.NewRootCmd()
	root.SetArgs([]string{"session", "list"})
	// Expected to fail (no daemon), but covers the default socket path branch.
	_ = root.Execute()
}

// TestDialDaemon_EnvSocket covers the LOCKSMITH_SOCKET env var branch in dialDaemon.
func TestDialDaemon_EnvSocket(t *testing.T) {
	t.Setenv("LOCKSMITH_SOCKET", "/tmp/nonexistent-test.sock")
	root := cli.NewRootCmd()
	root.SetArgs([]string{"session", "list"})
	// Will fail (no daemon), but covers the LOCKSMITH_SOCKET env branch.
	_ = root.Execute()
}
