// Package plugin manages the lifecycle of vault provider plugin processes.
// Plugins are discovered on disk, launched as child processes, and communicated
// with over gRPC using the hashicorp/go-plugin framework.
package plugin

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	goplugin "github.com/hashicorp/go-plugin"

	vaultv1 "github.com/lorem-dev/locksmith/gen/proto/vault/v1"
	"github.com/lorem-dev/locksmith/internal/log"
	"github.com/lorem-dev/locksmith/sdk/vault"
	sdkversion "github.com/lorem-dev/locksmith/sdk/version"
)

const pluginPrefix = "locksmith-plugin-"

// pluginClient abstracts *goplugin.Client for testability.
type pluginClient interface {
	Client() (goplugin.ClientProtocol, error)
	Kill()
}

// clientFactory creates a pluginClient from a binary path.
// Swappable in tests.
type clientFactory func(binaryPath string) pluginClient

func defaultClientFactory(binaryPath string) pluginClient {
	return goplugin.NewClient(vault.NewClientConfig(binaryPath))
}

// Manager owns the set of running vault plugin processes.
type Manager struct {
	mu            sync.RWMutex
	plugins       map[string]*runningPlugin
	clientFactory clientFactory
}

type runningPlugin struct {
	client   pluginClient
	provider vault.Provider
	info     *vaultv1.InfoResponse
	warnings []CompatWarning
}

// NewManager creates a new, empty plugin manager.
func NewManager() *Manager {
	return &Manager{
		plugins:       make(map[string]*runningPlugin),
		clientFactory: defaultClientFactory,
	}
}

// Discover searches the given directories for locksmith-plugin-* executables.
// Returns a map of vault type name → binary path. First match wins.
func Discover(searchDirs []string) map[string]string {
	found := make(map[string]string)
	for _, dir := range searchDirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasPrefix(entry.Name(), pluginPrefix) {
				continue
			}
			vaultType := strings.TrimPrefix(entry.Name(), pluginPrefix)
			fullPath := filepath.Join(dir, entry.Name())

			info, err := entry.Info()
			if err != nil || info.Mode()&0o111 == 0 {
				continue // skip non-executable
			}
			if _, exists := found[vaultType]; !exists {
				found[vaultType] = fullPath
				log.Debug().Str("vault", vaultType).Str("path", fullPath).Msg("discovered plugin")
			}
		}
	}
	return found
}

// DefaultSearchDirs returns the standard plugin lookup paths.
func DefaultSearchDirs() []string {
	var dirs []string
	if execPath, err := os.Executable(); err == nil {
		dirs = append(dirs, filepath.Dir(execPath))
	}
	if home, err := os.UserHomeDir(); err == nil {
		dirs = append(dirs, filepath.Join(home, ".config", "locksmith", "plugins"))
	}
	if pathEnv := os.Getenv("PATH"); pathEnv != "" {
		dirs = append(dirs, filepath.SplitList(pathEnv)...)
	}
	return dirs
}

// Launch starts a vault plugin process for the given vault type.
func (m *Manager) Launch(vaultType, binaryPath string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.plugins[vaultType]; exists {
		return fmt.Errorf("plugin %q already running", vaultType)
	}

	client := m.clientFactory(binaryPath)

	rpcClient, err := client.Client()
	if err != nil {
		client.Kill()
		return fmt.Errorf("connecting to plugin %q: %w", vaultType, err)
	}

	raw, err := rpcClient.Dispense("vault")
	if err != nil {
		client.Kill()
		return fmt.Errorf("dispensing plugin %q: %w", vaultType, err)
	}

	provider, ok := raw.(vault.Provider)
	if !ok {
		client.Kill()
		return fmt.Errorf("plugin %q does not implement Provider", vaultType)
	}

	rp := &runningPlugin{client: client, provider: provider}

	infoCtx, infoCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer infoCancel()
	info, infoErr := provider.Info(infoCtx, &vaultv1.InfoRequest{})
	if infoErr != nil {
		rp.warnings = append(rp.warnings, CompatWarning{
			Kind:    WarnInfoUnavailable,
			Message: infoErr.Error(),
		})
	} else {
		rp.info = info
		validator := &CompatValidator{
			Platform:         runtime.GOOS,
			LocksmithVersion: sdkversion.Current,
		}
		rp.warnings = append(rp.warnings, validator.Validate(info)...)
	}

	for _, w := range rp.warnings {
		ev := log.Warn()
		if w.Kind == WarnMinVersionMissing {
			ev = log.Debug()
		}
		ev.Str("vault", vaultType).Str("kind", string(w.Kind)).Msg(w.Message)
	}

	m.plugins[vaultType] = rp
	log.Info().Str("vault", vaultType).Str("binary", binaryPath).Msg("plugin launched")
	return nil
}

// Warnings returns stored compatibility warnings for the given vault type.
// Returns nil if the vault type is not loaded or has no warnings.
func (m *Manager) Warnings(vaultType string) []CompatWarning {
	m.mu.RLock()
	defer m.mu.RUnlock()
	rp, ok := m.plugins[vaultType]
	if !ok {
		return nil
	}
	return rp.warnings
}

// CachedInfo returns the InfoResponse cached at Launch time.
// Returns nil if the vault type is not loaded or Info() was unavailable.
func (m *Manager) CachedInfo(vaultType string) *vaultv1.InfoResponse {
	m.mu.RLock()
	defer m.mu.RUnlock()
	rp, ok := m.plugins[vaultType]
	if !ok {
		return nil
	}
	return rp.info
}

// Get returns the Provider for a vault type, or an error if not loaded.
func (m *Manager) Get(vaultType string) (vault.Provider, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	rp, ok := m.plugins[vaultType]
	if !ok {
		return nil, fmt.Errorf("no plugin loaded for vault type %q", vaultType)
	}
	return rp.provider, nil
}

// Types returns the list of loaded vault type names.
func (m *Manager) Types() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	types := make([]string, 0, len(m.plugins))
	for t := range m.plugins {
		types = append(types, t)
	}
	return types
}

// Kill stops all running plugin processes.
func (m *Manager) Kill() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for name, rp := range m.plugins {
		rp.client.Kill()
		delete(m.plugins, name)
		log.Debug().Str("vault", name).Msg("plugin killed")
	}
}

// KillOne stops the plugin process for a single vault type.
// It is a no-op if the vault type is not currently loaded.
func (m *Manager) KillOne(vaultType string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	rp, ok := m.plugins[vaultType]
	if !ok {
		return
	}
	rp.client.Kill()
	delete(m.plugins, vaultType)
	log.Debug().Str("vault", vaultType).Msg("plugin killed")
}
