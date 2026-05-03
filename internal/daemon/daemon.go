package daemon

import (
	"fmt"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/fsnotify/fsnotify"
	"google.golang.org/grpc"

	locksmithv1 "github.com/lorem-dev/locksmith/gen/proto/locksmith/v1"
	vaultv1 "github.com/lorem-dev/locksmith/gen/proto/vault/v1"
	"github.com/lorem-dev/locksmith/internal/config"
	"github.com/lorem-dev/locksmith/internal/log"
	pluginpkg "github.com/lorem-dev/locksmith/internal/plugin"
	"github.com/lorem-dev/locksmith/internal/session"
	"github.com/lorem-dev/locksmith/sdk/vault"
)

// daemonPlugins is the full plugin management interface used by Daemon.
// *plugin.Manager satisfies this interface.
type daemonPlugins interface {
	Launch(vaultType, binaryPath string) error
	KillOne(vaultType string)
	Kill()
	Get(vaultType string) (vault.Provider, error)
	Types() []string
	Warnings(vaultType string) []pluginpkg.CompatWarning
	CachedInfo(vaultType string) *vaultv1.InfoResponse
}

// Daemon manages the daemon lifecycle: plugin loading, gRPC server, session cleanup.
type Daemon struct {
	cfgPath string
	cfg     atomic.Pointer[config.Config]
	store   *session.Store
	plugins daemonPlugins

	reloadMu sync.Mutex

	mu            sync.Mutex
	grpcServer    *grpc.Server
	listener      net.Listener
	stopCleanup   chan struct{}
	stopOnce      sync.Once
	watchDebounce time.Duration // overridable in tests; default 1s
}

// New creates a Daemon from the given config and the path it was loaded from.
// cfgPath is stored so Reload() can re-read the file on demand.
func New(cfg *config.Config, cfgPath string) *Daemon {
	d := &Daemon{
		cfgPath:       cfgPath,
		store:         session.NewStore(),
		plugins:       pluginpkg.NewManager(),
		stopCleanup:   make(chan struct{}),
		watchDebounce: 1 * time.Second,
	}
	d.cfg.Store(cfg)
	return d
}

// loadedConfig returns the current config snapshot via atomic load.
func (d *Daemon) loadedConfig() *config.Config {
	return d.cfg.Load()
}

// Reload re-reads the config file, delta-syncs plugins, and atomically
// replaces the active config. Returns an error if the new config is
// invalid; the previous config remains active in that case.
func (d *Daemon) Reload() error {
	if d.cfgPath == "" {
		return fmt.Errorf("cannot reload: no config path set")
	}
	d.reloadMu.Lock()
	defer d.reloadMu.Unlock()

	newCfg, err := config.Load(d.cfgPath)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	if err := d.syncPlugins(d.cfg.Load(), newCfg); err != nil {
		return fmt.Errorf("syncing plugins: %w", err)
	}

	d.cfg.Store(newCfg)
	log.Info().Str("path", d.cfgPath).Msg("config reloaded")
	return nil
}

// syncPlugins computes the delta between old and new vault type sets,
// launches new plugins, and kills removed ones.
func (d *Daemon) syncPlugins(old, newCfg *config.Config) error {
	oldTypes := vaultTypeSet(old)
	newTypes := vaultTypeSet(newCfg)

	discovered := pluginpkg.Discover(pluginpkg.DefaultSearchDirs())

	for vaultType := range newTypes {
		if _, exists := oldTypes[vaultType]; !exists {
			binaryPath, ok := discovered[vaultType]
			if !ok {
				log.Warn().Str("type", vaultType).Msg("no plugin binary found for new vault type")
				continue
			}
			if err := d.plugins.Launch(vaultType, binaryPath); err != nil {
				return fmt.Errorf("launching plugin %q: %w", vaultType, err)
			}
		}
	}

	for vaultType := range oldTypes {
		if _, exists := newTypes[vaultType]; !exists {
			d.plugins.KillOne(vaultType)
		}
	}

	return nil
}

// vaultTypeSet returns the unique set of vault type names from a config.
func vaultTypeSet(cfg *config.Config) map[string]struct{} {
	types := make(map[string]struct{}, len(cfg.Vaults))
	for _, v := range cfg.Vaults {
		types[v.Type] = struct{}{}
	}
	return types
}

// Start initialises the Unix socket, loads vault plugins, and begins serving gRPC.
func (d *Daemon) Start() error {
	cfg := d.cfg.Load()
	socketPath := config.ExpandPath(cfg.Defaults.SocketPath)

	if err := os.MkdirAll(filepath.Dir(socketPath), 0o700); err != nil {
		return fmt.Errorf("creating socket directory: %w", err)
	}
	if err := os.Remove(socketPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("removing stale socket: %w", err)
	}

	if err := d.loadPlugins(); err != nil {
		return fmt.Errorf("loading plugins: %w", err)
	}

	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		return fmt.Errorf("listening on %s: %w", socketPath, err)
	}

	if err := os.Chmod(socketPath, 0o600); err != nil {
		listener.Close() //nolint:errcheck
		return fmt.Errorf("setting socket permissions: %w", err)
	}

	srv := grpc.NewServer()
	locksmithv1.RegisterLocksmithServiceServer(srv, NewServerWithRegistry(d.loadedConfig, d.store, d.plugins, d))

	d.mu.Lock()
	d.listener = listener
	d.grpcServer = srv
	d.mu.Unlock()

	go d.cleanupLoop()
	go d.watchConfig()

	log.Info().Str("socket", socketPath).Msg("locksmith daemon listening")
	if err := srv.Serve(listener); err != nil {
		return fmt.Errorf("gRPC serve: %w", err)
	}
	return nil
}

// Stop gracefully shuts down the gRPC server and kills plugin processes.
func (d *Daemon) Stop() {
	d.mu.Lock()
	srv := d.grpcServer
	ln := d.listener
	d.mu.Unlock()

	d.stopOnce.Do(func() { close(d.stopCleanup) })

	if srv != nil {
		srv.GracefulStop()
	}
	if ln != nil {
		ln.Close() //nolint:errcheck
	}
	d.plugins.Kill()
	log.Info().Msg("locksmith daemon stopped")
}

// WaitForShutdown blocks until a shutdown or reload signal arrives.
// SIGHUP triggers a config reload; SIGINT/SIGTERM trigger graceful shutdown.
func (d *Daemon) WaitForShutdown() {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)
	for {
		sig := <-sigCh
		switch sig {
		case syscall.SIGHUP:
			if err := d.Reload(); err != nil {
				log.Error().Err(err).Msg("config reload failed via SIGHUP, keeping old config")
			} else {
				log.Info().Msg("config reloaded via SIGHUP")
			}
		default:
			log.Info().Msg("received shutdown signal")
			d.Stop()
			return
		}
	}
}

// watchConfig watches the config file directory and triggers Reload() after
// a debounce period of quiet. Stops when stopCleanup is closed.
func (d *Daemon) watchConfig() {
	if d.cfgPath == "" {
		return
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Warn().Err(err).Msg("failed to create config file watcher")
		return
	}
	defer watcher.Close() //nolint:errcheck // watcher cleanup on goroutine exit; error not actionable

	cfgDir := filepath.Dir(d.cfgPath)
	if err := watcher.Add(cfgDir); err != nil {
		log.Warn().Err(err).Str("dir", cfgDir).Msg("failed to watch config directory")
		return
	}

	debounce := time.NewTimer(0)
	if !debounce.Stop() {
		<-debounce.C
	}

	log.Debug().Str("path", d.cfgPath).Msg("watching config file for changes")

	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}
			if event.Name != d.cfgPath {
				continue
			}
			if !event.Has(fsnotify.Write) && !event.Has(fsnotify.Create) && !event.Has(fsnotify.Rename) {
				continue
			}
			debounce.Reset(d.watchDebounce)

		case <-debounce.C:
			if err := d.Reload(); err != nil {
				log.Error().Err(err).Msg("config reload failed (file watcher), keeping old config")
			}

		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			log.Warn().Err(err).Msg("config watcher error")

		case <-d.stopCleanup:
			return
		}
	}
}

func (d *Daemon) loadPlugins() error {
	cfg := d.cfg.Load()
	discovered := pluginpkg.Discover(pluginpkg.DefaultSearchDirs())
	for name, v := range cfg.Vaults {
		binaryPath, ok := discovered[v.Type]
		if !ok {
			log.Warn().Str("vault", name).Str("type", v.Type).Msg("no plugin binary found for vault type")
			continue
		}
		if err := d.plugins.Launch(v.Type, binaryPath); err != nil {
			return fmt.Errorf("launching plugin %q: %w", v.Type, err)
		}
	}
	return nil
}

func (d *Daemon) cleanupLoop() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			d.cleanupExpired()
		case <-d.stopCleanup:
			return
		}
	}
}

func (d *Daemon) cleanupExpired() {
	if removed := d.store.Cleanup(); removed > 0 {
		log.Info().Int("removed", removed).Msg("expired sessions cleaned up")
	}
}
