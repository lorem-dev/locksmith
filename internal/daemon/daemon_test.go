package daemon

import (
	"io"
	"net"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/lorem-dev/locksmith/internal/config"
	"github.com/lorem-dev/locksmith/internal/log"
)

func init() {
	log.Init(io.Discard, "error", "text")
}

func TestParseTTL_Explicit(t *testing.T) {
	d, err := parseTTL("2h", "3h")
	if err != nil {
		t.Fatalf("parseTTL() error: %v", err)
	}
	if d != 2*time.Hour {
		t.Errorf("duration = %v, want 2h", d)
	}
}

func TestParseTTL_Default(t *testing.T) {
	d, err := parseTTL("", "3h")
	if err != nil {
		t.Fatalf("parseTTL() error: %v", err)
	}
	if d != 3*time.Hour {
		t.Errorf("duration = %v, want 3h", d)
	}
}

func TestParseTTL_Invalid(t *testing.T) {
	_, err := parseTTL("notaduration", "3h")
	if err == nil {
		t.Fatal("parseTTL() expected error for invalid input")
	}
}

func TestDaemon_StartStop(t *testing.T) {
	socketPath := filepath.Join(t.TempDir(), "test.sock")
	cfg := &config.Config{
		Defaults: config.Defaults{
			SessionTTL: "1h",
			SocketPath: socketPath,
		},
		Vaults: map[string]config.Vault{},
		Keys:   map[string]config.Key{},
	}

	d := New(cfg)

	errCh := make(chan error, 1)
	go func() {
		errCh <- d.Start()
	}()

	// Wait for the socket to appear.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(socketPath); err == nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	// Verify we can connect.
	conn, err := net.DialTimeout("unix", socketPath, time.Second)
	if err != nil {
		t.Fatalf("could not connect to socket: %v", err)
	}
	conn.Close()

	d.Stop()

	select {
	case err := <-errCh:
		// grpc.Serve returns nil after GracefulStop.
		if err != nil {
			t.Errorf("Start() returned error: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Error("daemon did not stop in time")
	}
}

func TestDaemon_Start_StaleSocket(t *testing.T) {
	dir := t.TempDir()
	socketPath := filepath.Join(dir, "test.sock")

	// Create a stale socket file.
	f, err := os.Create(socketPath)
	if err != nil {
		t.Fatal(err)
	}
	f.Close()

	cfg := &config.Config{
		Defaults: config.Defaults{SessionTTL: "1h", SocketPath: socketPath},
		Vaults:   map[string]config.Vault{},
		Keys:     map[string]config.Key{},
	}

	d := New(cfg)
	errCh := make(chan error, 1)
	go func() {
		errCh <- d.Start()
	}()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(socketPath); err == nil {
			if conn, err := net.DialTimeout("unix", socketPath, 100*time.Millisecond); err == nil {
				conn.Close()
				break
			}
		}
		time.Sleep(10 * time.Millisecond)
	}

	d.Stop()

	select {
	case err := <-errCh:
		if err != nil {
			t.Errorf("Start() returned error: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Error("daemon did not stop in time")
	}
}

func TestDaemon_WaitForShutdown(t *testing.T) {
	socketPath := filepath.Join(t.TempDir(), "test2.sock")
	cfg := &config.Config{
		Defaults: config.Defaults{SessionTTL: "1h", SocketPath: socketPath},
		Vaults:   map[string]config.Vault{},
		Keys:     map[string]config.Key{},
	}

	d := New(cfg)

	startErrCh := make(chan error, 1)
	go func() {
		startErrCh <- d.Start()
	}()

	// Wait for socket to be ready.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(socketPath); err == nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	// WaitForShutdown blocks on a signal channel. We trigger shutdown directly
	// by calling Stop() and verify the daemon stops cleanly. The WaitForShutdown
	// path itself (signal.Notify + channel receive) is exercised separately at
	// the OS level; testing it would require sending SIGTERM to the whole test
	// binary, which aborts the process.
	d.Stop()

	select {
	case err := <-startErrCh:
		if err != nil {
			t.Errorf("Start() returned error after Stop: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Error("daemon did not stop in time")
	}
}

func TestDaemon_loadPlugins_NoVaults(t *testing.T) {
	cfg := &config.Config{
		Defaults: config.Defaults{SessionTTL: "1h"},
		Vaults:   map[string]config.Vault{},
		Keys:     map[string]config.Key{},
	}
	d := New(cfg)
	if err := d.loadPlugins(); err != nil {
		t.Fatalf("loadPlugins() with no vaults: %v", err)
	}
}

func TestDaemon_loadPlugins_MissingBinary(t *testing.T) {
	cfg := &config.Config{
		Defaults: config.Defaults{SessionTTL: "1h"},
		Vaults:   map[string]config.Vault{"myvault": {Type: "nonexistent-vault-type-xyz"}},
		Keys:     map[string]config.Key{},
	}
	d := New(cfg)
	// Should not error - just warns when binary not found.
	if err := d.loadPlugins(); err != nil {
		t.Fatalf("loadPlugins() with missing binary should warn not error: %v", err)
	}
}

func TestDaemon_cleanupLoop(t *testing.T) {
	cfg := &config.Config{
		Defaults: config.Defaults{SessionTTL: "1h"},
		Vaults:   map[string]config.Vault{},
		Keys:     map[string]config.Key{},
	}
	d := New(cfg)
	// cleanupExpired on empty store removes nothing.
	d.cleanupExpired()
}

func TestDaemon_Start_InvalidSocketDir(t *testing.T) {
	// Use a path that can't be created (file exists where dir should be).
	dir := t.TempDir()
	// Create a regular file to block directory creation.
	blocker := filepath.Join(dir, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	// Socket path inside the blocker file (can't mkdir blocker/subdir).
	socketPath := filepath.Join(blocker, "subdir", "test.sock")
	cfg := &config.Config{
		Defaults: config.Defaults{SessionTTL: "1h", SocketPath: socketPath},
		Vaults:   map[string]config.Vault{},
		Keys:     map[string]config.Key{},
	}
	d := New(cfg)
	err := d.Start()
	if err == nil {
		d.Stop()
		t.Fatal("Start() expected error for invalid socket directory")
	}
}

func TestDaemon_Start_ListenError(t *testing.T) {
	// Use an existing regular file as the socket path (after mkdir succeeds for the dir)
	// so that Listen fails with "address already in use" or similar.
	dir := t.TempDir()
	// Create a directory where the socket should be so net.Listen fails.
	socketPath := filepath.Join(dir, "socket-is-a-dir")
	if err := os.Mkdir(socketPath, 0o700); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{
		Defaults: config.Defaults{SessionTTL: "1h", SocketPath: socketPath},
		Vaults:   map[string]config.Vault{},
		Keys:     map[string]config.Key{},
	}
	d := New(cfg)
	err := d.Start()
	if err == nil {
		d.Stop()
		t.Fatal("Start() expected error when socket path is a directory")
	}
}

// TestDaemon_cleanupExpired_WithExpiredSession exercises the removed > 0 branch.
func TestDaemon_cleanupExpired_WithExpiredSession(t *testing.T) {
	cfg := &config.Config{
		Defaults: config.Defaults{SessionTTL: "1h"},
		Vaults:   map[string]config.Vault{},
		Keys:     map[string]config.Key{},
	}
	d := New(cfg)
	// Create a session with a very short TTL so it expires immediately.
	d.store.Create(1*time.Nanosecond, nil)
	time.Sleep(time.Millisecond)
	// cleanupExpired should remove the expired session and log.
	d.cleanupExpired()
}

func TestDaemon_WaitForShutdown_Signal(t *testing.T) {
	cfg := &config.Config{
		Defaults: config.Defaults{SessionTTL: "1h", SocketPath: filepath.Join(t.TempDir(), "signal.sock")},
		Vaults:   map[string]config.Vault{},
		Keys:     map[string]config.Key{},
	}
	d := New(cfg)
	done := make(chan struct{})
	go func() {
		d.WaitForShutdown()
		close(done)
	}()
	// Give signal handler time to register.
	time.Sleep(20 * time.Millisecond)
	syscall.Kill(os.Getpid(), syscall.SIGINT)
	select {
	case <-done:
		// WaitForShutdown returned - success.
	case <-time.After(2 * time.Second):
		t.Fatal("WaitForShutdown() did not return after SIGINT")
	}
}
