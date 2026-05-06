package daemon

import (
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"testing"
	"time"

	vaultv1 "github.com/lorem-dev/locksmith/gen/proto/vault/v1"
	"github.com/lorem-dev/locksmith/internal/config"
	"github.com/lorem-dev/locksmith/internal/log"
	pluginpkg "github.com/lorem-dev/locksmith/internal/plugin"
	vault "github.com/lorem-dev/locksmith/sdk/vault"
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

	d := New(cfg, "")

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

	d := New(cfg, "")
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

	d := New(cfg, "")

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
	d := New(cfg, "")
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
	d := New(cfg, "")
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
	d := New(cfg, "")
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
	d := New(cfg, "")
	err := d.Start()
	if err == nil {
		d.Stop()
		t.Fatal("Start() expected error for invalid socket directory")
	}
}

func TestDaemon_Start_ListenError(t *testing.T) {
	// Use a non-empty directory at the socket path so that Start()'s stale-socket
	// removal fails (os.Remove cannot remove a non-empty directory) on both
	// macOS and Linux. An empty directory is removable, and net.Listen("unix",
	// path) on a freshly emptied path then succeeds on Linux (where the socket
	// path length limit is 108 bytes), causing Start() to block in grpc.Serve.
	// A non-empty directory triggers a deterministic error on both platforms
	// before Start() reaches the listen step.
	dir := t.TempDir()
	socketPath := filepath.Join(dir, "socket-is-a-dir")
	if err := os.Mkdir(socketPath, 0o700); err != nil {
		t.Fatal(err)
	}
	// Place a file inside so os.Remove(socketPath) returns "directory not empty".
	if err := os.WriteFile(filepath.Join(socketPath, "blocker"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{
		Defaults: config.Defaults{SessionTTL: "1h", SocketPath: socketPath},
		Vaults:   map[string]config.Vault{},
		Keys:     map[string]config.Key{},
	}
	d := New(cfg, "")
	err := d.Start()
	if err == nil {
		d.Stop()
		t.Fatal("Start() expected error when socket path is a non-empty directory")
	}
}

// TestDaemon_cleanupExpired_WithExpiredSession exercises the removed > 0 branch.
func TestDaemon_cleanupExpired_WithExpiredSession(t *testing.T) {
	cfg := &config.Config{
		Defaults: config.Defaults{SessionTTL: "1h"},
		Vaults:   map[string]config.Vault{},
		Keys:     map[string]config.Key{},
	}
	d := New(cfg, "")
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
	d := New(cfg, "")
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

// fakePluginManager is a daemonPlugins stub for testing syncPlugins.
type fakePluginManager struct {
	mu       sync.Mutex
	launched []string
	killed   []string
	running  map[string]struct{}
}

func (f *fakePluginManager) Launch(vaultType, _ string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.launched = append(f.launched, vaultType)
	f.running[vaultType] = struct{}{}
	return nil
}

func (f *fakePluginManager) KillOne(vaultType string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.killed = append(f.killed, vaultType)
	delete(f.running, vaultType)
}

func (f *fakePluginManager) Kill() {
	f.mu.Lock()
	defer f.mu.Unlock()
	for k := range f.running {
		delete(f.running, k)
	}
}

func (f *fakePluginManager) Get(_ string) (vault.Provider, error) { return nil, nil }

func (f *fakePluginManager) Warnings(_ string) []pluginpkg.CompatWarning { return nil }

func (f *fakePluginManager) CachedInfo(_ string) *vaultv1.InfoResponse { return nil }

func (f *fakePluginManager) Types() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]string, 0, len(f.running))
	for t := range f.running {
		out = append(out, t)
	}
	return out
}

func newFakePluginManager(running ...string) *fakePluginManager {
	f := &fakePluginManager{running: make(map[string]struct{})}
	for _, t := range running {
		f.running[t] = struct{}{}
	}
	return f
}

//nolint:unparam // content kept for readability at call sites
func writeTempConfig(t *testing.T, dir, content string) string {
	t.Helper()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

const baseConfigYAML = `
defaults:
  session_ttl: 1h
vaults: {}
keys: {}
`

func TestDaemon_Reload_Success(t *testing.T) {
	dir := t.TempDir()
	cfgPath := writeTempConfig(t, dir, baseConfigYAML)

	cfg, err := config.LoadFromBytes([]byte(baseConfigYAML))
	if err != nil {
		t.Fatal(err)
	}
	d := New(cfg, cfgPath)

	updated := `
defaults:
  session_ttl: 2h
vaults: {}
keys: {}
`
	if err := os.WriteFile(cfgPath, []byte(updated), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := d.Reload(); err != nil {
		t.Fatalf("Reload() error: %v", err)
	}
	if got := d.loadedConfig().Defaults.SessionTTL; got != "2h" {
		t.Errorf("SessionTTL = %s, want 2h", got)
	}
}

func TestDaemon_Reload_InvalidConfig(t *testing.T) {
	dir := t.TempDir()
	cfgPath := writeTempConfig(t, dir, baseConfigYAML)

	cfg, err := config.LoadFromBytes([]byte(baseConfigYAML))
	if err != nil {
		t.Fatal(err)
	}
	d := New(cfg, cfgPath)

	if err := os.WriteFile(cfgPath, []byte("invalid: yaml: [\nbad"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := d.Reload(); err == nil {
		t.Fatal("Reload() expected error for invalid config")
	}
	if got := d.loadedConfig().Defaults.SessionTTL; got != "1h" {
		t.Errorf("config changed despite error; SessionTTL = %s, want 1h", got)
	}
}

func TestDaemon_Reload_Concurrent(t *testing.T) {
	dir := t.TempDir()
	cfgPath := writeTempConfig(t, dir, baseConfigYAML)
	cfg, _ := config.LoadFromBytes([]byte(baseConfigYAML))
	d := New(cfg, cfgPath)

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = d.Reload()
		}()
	}
	wg.Wait()
}

func TestDaemon_syncPlugins_Kill(t *testing.T) {
	oldCfg := &config.Config{
		Defaults: config.Defaults{SessionTTL: "1h"},
		Vaults: map[string]config.Vault{
			"kc": {Type: "keychain"},
			"gp": {Type: "gopass"},
		},
		Keys: map[string]config.Key{},
	}
	newCfg := &config.Config{
		Defaults: config.Defaults{SessionTTL: "1h"},
		Vaults: map[string]config.Vault{
			"kc": {Type: "keychain"},
		},
		Keys: map[string]config.Key{},
	}

	fake := newFakePluginManager("keychain", "gopass")
	d := New(oldCfg, "")
	d.plugins = fake

	if err := d.syncPlugins(oldCfg, newCfg); err != nil {
		t.Fatalf("syncPlugins() error: %v", err)
	}

	fake.mu.Lock()
	defer fake.mu.Unlock()

	killed := make(map[string]bool)
	for _, k := range fake.killed {
		killed[k] = true
	}
	if !killed["gopass"] {
		t.Error("expected gopass to be killed")
	}
	if killed["keychain"] {
		t.Error("keychain should not have been killed")
	}
}

func TestDaemon_syncPlugins_Launch(t *testing.T) {
	pluginDir := t.TempDir()
	fakeBin := filepath.Join(pluginDir, "locksmith-plugin-1password")
	if err := os.WriteFile(fakeBin, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", pluginDir+":"+os.Getenv("PATH"))

	oldCfg := &config.Config{
		Defaults: config.Defaults{SessionTTL: "1h"},
		Vaults:   map[string]config.Vault{"kc": {Type: "keychain"}},
		Keys:     map[string]config.Key{},
	}
	newCfg := &config.Config{
		Defaults: config.Defaults{SessionTTL: "1h"},
		Vaults: map[string]config.Vault{
			"kc": {Type: "keychain"},
			"op": {Type: "1password"},
		},
		Keys: map[string]config.Key{},
	}

	fake := newFakePluginManager("keychain")
	d := New(oldCfg, "")
	d.plugins = fake

	if err := d.syncPlugins(oldCfg, newCfg); err != nil {
		t.Fatalf("syncPlugins() error: %v", err)
	}

	fake.mu.Lock()
	defer fake.mu.Unlock()

	launched := make(map[string]bool)
	for _, l := range fake.launched {
		launched[l] = true
	}
	if !launched["1password"] {
		t.Error("expected 1password to be launched")
	}
	if launched["keychain"] {
		t.Error("keychain should not have been launched again")
	}
}

func TestDaemon_WaitForShutdown_SIGHUP_Reloads(t *testing.T) {
	dir := t.TempDir()
	cfgPath := writeTempConfig(t, dir, baseConfigYAML)

	cfg, _ := config.LoadFromBytes([]byte(baseConfigYAML))
	d := New(cfg, cfgPath)

	// Write updated config before SIGHUP.
	updated := `
defaults:
  session_ttl: 2h
vaults: {}
keys: {}
`
	if err := os.WriteFile(cfgPath, []byte(updated), 0o600); err != nil {
		t.Fatal(err)
	}

	done := make(chan struct{})
	go func() {
		d.WaitForShutdown()
		close(done)
	}()
	// Give signal handler time to register.
	time.Sleep(20 * time.Millisecond)

	// SIGHUP should reload, not stop.
	if err := syscall.Kill(os.Getpid(), syscall.SIGHUP); err != nil {
		t.Fatal(err)
	}
	time.Sleep(100 * time.Millisecond)

	if got := d.loadedConfig().Defaults.SessionTTL; got != "2h" {
		t.Errorf("after SIGHUP SessionTTL = %s, want 2h", got)
	}

	// Daemon must still be running - send SIGINT to stop it.
	if err := syscall.Kill(os.Getpid(), syscall.SIGINT); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("WaitForShutdown did not return after SIGINT")
	}
}

func TestDaemon_watchConfig_Debounce(t *testing.T) {
	dir := t.TempDir()
	cfgPath := writeTempConfig(t, dir, baseConfigYAML)

	cfg, _ := config.LoadFromBytes([]byte(baseConfigYAML))
	d := New(cfg, cfgPath)
	d.watchDebounce = 50 * time.Millisecond // speed up for test

	go d.watchConfig()

	// Write a new config three times quickly - should debounce to one reload.
	updated := `
defaults:
  session_ttl: 3h
vaults: {}
keys: {}
`
	for i := 0; i < 3; i++ {
		if err := os.WriteFile(cfgPath, []byte(updated), 0o600); err != nil {
			t.Fatal(err)
		}
		time.Sleep(10 * time.Millisecond)
	}

	// Wait for debounce to fire (50ms quiet + margin).
	time.Sleep(200 * time.Millisecond)

	if got := d.loadedConfig().Defaults.SessionTTL; got != "3h" {
		t.Errorf("after file change SessionTTL = %s, want 3h", got)
	}

	// Stop the watcher goroutine cleanly.
	d.stopOnce.Do(func() { close(d.stopCleanup) })
}

func TestDaemon_Reload_EmptyCfgPath(t *testing.T) {
	cfg, _ := config.LoadFromBytes([]byte(baseConfigYAML))
	d := New(cfg, "") // no config path
	err := d.Reload()
	if err == nil {
		t.Fatal("Reload() expected error when cfgPath is empty")
	}
}

func TestDaemon_Reload_SyncPluginsError(t *testing.T) {
	dir := t.TempDir()
	cfgPath := writeTempConfig(t, dir, baseConfigYAML)
	cfg, _ := config.LoadFromBytes([]byte(baseConfigYAML))
	d := New(cfg, cfgPath)

	// Replace plugins with one that fails on Launch for new types.
	d.plugins = &errorOnLaunchPluginManager{}

	// Write config that adds a new vault type so syncPlugins tries to launch.
	pluginDir := t.TempDir()
	fakeBin := filepath.Join(pluginDir, "locksmith-plugin-errorvault")
	if err := os.WriteFile(fakeBin, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", pluginDir+":"+os.Getenv("PATH"))

	newCfgYAML := `
defaults:
  session_ttl: 1h
vaults:
  ev:
    type: errorvault
keys: {}
`
	if err := os.WriteFile(cfgPath, []byte(newCfgYAML), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := d.Reload(); err == nil {
		t.Fatal("Reload() expected error when syncPlugins fails")
	}
}

// errorOnLaunchPluginManager is a daemonPlugins stub that errors on Launch.
type errorOnLaunchPluginManager struct {
	fakePluginManager
}

func (e *errorOnLaunchPluginManager) Launch(_, _ string) error {
	return fmt.Errorf("launch error (test stub)")
}

func TestDaemon_watchConfig_EmptyCfgPath(t *testing.T) {
	cfg, _ := config.LoadFromBytes([]byte(baseConfigYAML))
	d := New(cfg, "")
	// watchConfig should return immediately when cfgPath is empty.
	done := make(chan struct{})
	go func() {
		d.watchConfig()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("watchConfig did not return immediately for empty cfgPath")
	}
}

func TestDaemon_watchConfig_InvalidDir(t *testing.T) {
	cfg, _ := config.LoadFromBytes([]byte(baseConfigYAML))
	// Point to a file inside a non-existent directory - watcher.Add will fail.
	d := New(cfg, "/nonexistent/dir/config.yaml")
	done := make(chan struct{})
	go func() {
		d.watchConfig()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("watchConfig did not return when watcher.Add fails")
	}
}

func TestDaemon_cleanupLoop_Tick(t *testing.T) {
	cfg := &config.Config{
		Defaults: config.Defaults{SessionTTL: "1h"},
		Vaults:   map[string]config.Vault{},
		Keys:     map[string]config.Key{},
	}
	d := New(cfg, "")
	// Run cleanupLoop in background and stop it quickly.
	go d.cleanupLoop()
	time.Sleep(10 * time.Millisecond)
	d.stopOnce.Do(func() { close(d.stopCleanup) })
}
