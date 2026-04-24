package cli_test

import (
	"net"
	"os"
	"path/filepath"
	"testing"

	"github.com/lorem-dev/locksmith/internal/cli"
)

func TestAutostart_DaemonAlreadyRunning(t *testing.T) {
	// Use a short path under /tmp to stay within the macOS 104-char Unix socket limit.
	dir, err := os.MkdirTemp("", "lks")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	sock := filepath.Join(dir, "ls.sock")

	// Start a real Unix socket listener to simulate a running daemon.
	l, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()

	t.Setenv("LOCKSMITH_SOCKET", sock)

	cmd := cli.NewRootCmd()
	cmd.SetArgs([]string{"_autostart"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAutostart_DaemonNotRunning(t *testing.T) {
	dir := t.TempDir()
	sock := filepath.Join(dir, "nonexistent.sock")
	t.Setenv("LOCKSMITH_SOCKET", sock)
	// Redirect HOME so the spawned "serve" process cannot find a real config and
	// exits quickly. Without this, serve finds ~/.config/locksmith/config.yaml and
	// starts a long-lived daemon that accumulates across test runs.
	t.Setenv("HOME", dir)

	cmd := cli.NewRootCmd()
	cmd.SetArgs([]string{"_autostart"})
	// Must return nil even when daemon is not running (shell must not break).
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAutostart_StaleSocket(t *testing.T) {
	dir := t.TempDir()
	sock := filepath.Join(dir, "locksmith.sock")
	// Create the file but do not bind to it (stale socket).
	if err := os.WriteFile(sock, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("LOCKSMITH_SOCKET", sock)
	// Redirect HOME so the spawned "serve" process exits fast (no real config found).
	t.Setenv("HOME", dir)

	cmd := cli.NewRootCmd()
	cmd.SetArgs([]string{"_autostart"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAutostart_Hidden(t *testing.T) {
	root := cli.NewRootCmd()
	found := false
	for _, sub := range root.Commands() {
		if sub.Use == "_autostart" {
			found = true
			if !sub.Hidden {
				t.Error("_autostart command should be Hidden=true")
			}
		}
	}
	if !found {
		t.Error("_autostart command is not registered")
	}
}
