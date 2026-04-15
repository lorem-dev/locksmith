package daemon

import (
	"fmt"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"google.golang.org/grpc"

	locksmithv1 "github.com/lorem-dev/locksmith/gen/proto/locksmith/v1"
	"github.com/lorem-dev/locksmith/internal/config"
	"github.com/lorem-dev/locksmith/internal/log"
	pluginpkg "github.com/lorem-dev/locksmith/internal/plugin"
	"github.com/lorem-dev/locksmith/internal/session"
)

// Daemon manages the daemon lifecycle: plugin loading, gRPC server, session cleanup.
type Daemon struct {
	cfg     *config.Config
	store   *session.Store
	plugins *pluginpkg.Manager

	mu          sync.Mutex
	grpcServer  *grpc.Server
	listener    net.Listener
	stopCleanup chan struct{}
	stopOnce    sync.Once
}

// New creates a Daemon from the given config.
func New(cfg *config.Config) *Daemon {
	return &Daemon{
		cfg:         cfg,
		store:       session.NewStore(),
		plugins:     pluginpkg.NewManager(),
		stopCleanup: make(chan struct{}),
	}
}

// Start initialises the Unix socket, loads vault plugins, and begins serving gRPC.
// Blocks until the gRPC server stops. Call Stop() or WaitForShutdown() to stop it.
func (d *Daemon) Start() error {
	socketPath := config.ExpandPath(d.cfg.Defaults.SocketPath)

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
		listener.Close()
		return fmt.Errorf("setting socket permissions: %w", err)
	}

	srv := grpc.NewServer()
	locksmithv1.RegisterLocksmithServiceServer(srv, NewServer(d.cfg, d.store, d.plugins))

	d.mu.Lock()
	d.listener = listener
	d.grpcServer = srv
	d.mu.Unlock()

	go d.cleanupLoop()

	log.Info().Str("socket", socketPath).Msg("locksmith daemon listening")
	return srv.Serve(listener)
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
		ln.Close()
	}
	d.plugins.Kill()
	log.Info().Msg("locksmith daemon stopped")
}

// WaitForShutdown blocks until SIGINT or SIGTERM, then calls Stop().
func (d *Daemon) WaitForShutdown() {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
	log.Info().Msg("received shutdown signal")
	d.Stop()
}

func (d *Daemon) loadPlugins() error {
	discovered := pluginpkg.Discover(pluginpkg.DefaultSearchDirs())
	for name, vault := range d.cfg.Vaults {
		binaryPath, ok := discovered[vault.Type]
		if !ok {
			log.Warn().Str("vault", name).Str("type", vault.Type).Msg("no plugin binary found for vault type")
			continue
		}
		if err := d.plugins.Launch(vault.Type, binaryPath); err != nil {
			return fmt.Errorf("launching plugin %q: %w", vault.Type, err)
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
