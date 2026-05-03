// Package daemon implements the LocksmithService gRPC server and the Unix socket
// daemon lifecycle (Start, Stop, signal handling, session cleanup).
package daemon

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	locksmithv1 "github.com/lorem-dev/locksmith/gen/proto/locksmith/v1"
	vaultv1 "github.com/lorem-dev/locksmith/gen/proto/vault/v1"
	"github.com/lorem-dev/locksmith/internal/config"
	"github.com/lorem-dev/locksmith/internal/log"
	pluginpkg "github.com/lorem-dev/locksmith/internal/plugin"
	"github.com/lorem-dev/locksmith/internal/session"
	sdksession "github.com/lorem-dev/locksmith/sdk/session"
	"github.com/lorem-dev/locksmith/sdk/vault"
)

// pluginRegistry is the subset of plugin.Manager used by Server.
// It is satisfied by *plugin.Manager and can be replaced by a test double.
type pluginRegistry interface {
	Get(vaultType string) (vault.Provider, error)
	Types() []string
	Warnings(vaultType string) []pluginpkg.CompatWarning
	CachedInfo(vaultType string) *vaultv1.InfoResponse
}

// reloader is implemented by Daemon and injected into Server so that
// the ReloadConfig RPC can trigger a config reload without an import cycle.
type reloader interface {
	Reload() error
}

// Server is the gRPC implementation of LocksmithService.
type Server struct {
	locksmithv1.UnimplementedLocksmithServiceServer

	cfgFn    func() *config.Config
	store    *session.Store
	plugins  pluginRegistry
	reloader reloader
}

// NewServer creates a LocksmithService server.
// cfgFn is called on every request to read the current config snapshot.
// r may be nil (tests that do not exercise ReloadConfig can pass nil).
func NewServer(cfgFn func() *config.Config, store *session.Store, plugins *pluginpkg.Manager, r reloader) *Server {
	var reg pluginRegistry
	if plugins != nil {
		reg = plugins
	}
	return &Server{cfgFn: cfgFn, store: store, plugins: reg, reloader: r}
}

// NewServerWithRegistry creates a Server with an arbitrary pluginRegistry.
// Use this in tests to inject fakes; production code uses NewServer.
func NewServerWithRegistry(cfgFn func() *config.Config, store *session.Store, reg pluginRegistry, r reloader) *Server {
	return &Server{cfgFn: cfgFn, store: store, plugins: reg, reloader: r}
}

// SessionStart creates a new agent session with the requested TTL and key restrictions.
func (s *Server) SessionStart(
	_ context.Context,
	req *locksmithv1.SessionStartRequest,
) (*locksmithv1.SessionStartResponse, error) {
	cfg := s.cfgFn()
	ttl, err := parseTTL(req.Ttl, cfg.Defaults.SessionTTL)
	if err != nil {
		return nil, fmt.Errorf("invalid TTL: %w", err)
	}
	sess, err := s.store.Create(ttl, req.AllowedKeys)
	if err != nil {
		return nil, fmt.Errorf("creating session: %w", err)
	}
	log.Info().Str("session_id", sdksession.MaskSessionId(sess.ID)).Dur("ttl", ttl).Msg("session started")
	return &locksmithv1.SessionStartResponse{
		SessionId: sess.ID,
		ExpiresAt: sess.ExpiresAt.Format(time.RFC3339),
	}, nil
}

// SessionEnd invalidates a session and wipes its cached secrets.
func (s *Server) SessionEnd(
	_ context.Context,
	req *locksmithv1.SessionEndRequest,
) (*locksmithv1.SessionEndResponse, error) {
	sessionID, err := s.store.Delete(req.SessionIdPrefix)
	if err != nil {
		return nil, fmt.Errorf("deleting session: %w", err)
	}
	log.Info().Str("session_id", sdksession.MaskSessionId(*sessionID)).Msg("session ended")
	return &locksmithv1.SessionEndResponse{SessionId: *sessionID}, nil
}

// SessionList returns metadata for all active sessions.
func (s *Server) SessionList(
	_ context.Context,
	_ *locksmithv1.SessionListRequest,
) (*locksmithv1.SessionListResponse, error) {
	sessions := s.store.List()
	infos := make([]*locksmithv1.SessionInfo, len(sessions))
	for i, sess := range sessions {
		infos[i] = &locksmithv1.SessionInfo{
			SessionId:   sdksession.HideSessionId(sess.ID),
			CreatedAt:   sess.CreatedAt.Format(time.RFC3339),
			ExpiresAt:   sess.ExpiresAt.Format(time.RFC3339),
			AllowedKeys: sess.AllowedKeys,
		}
	}
	return &locksmithv1.SessionListResponse{Sessions: infos}, nil
}

// GetSecret retrieves a secret from the appropriate vault plugin, serving from
// the session cache when possible to avoid repeated vault authorization prompts.
func (s *Server) GetSecret(
	ctx context.Context,
	req *locksmithv1.GetSecretRequest,
) (*locksmithv1.GetSecretResponse, error) {
	if _, err := s.store.Get(req.SessionId); err != nil {
		return nil, fmt.Errorf("invalid session: %w", err)
	}

	vaultType, path, opts, err := s.resolveKey(req)
	if err != nil {
		return nil, err
	}

	cacheKey := vaultType + ":" + path
	if cached, ok := s.store.GetCachedSecret(req.SessionId, cacheKey); ok {
		log.Debug().
			Str("session_id", sdksession.MaskSessionId(req.SessionId)).
			Str("key", cacheKey).
			Msg("serving secret from cache")
		return &locksmithv1.GetSecretResponse{Secret: cached, ContentType: "text/plain"}, nil
	}

	if s.plugins == nil {
		return nil, fmt.Errorf("no plugin manager available")
	}
	provider, err := s.plugins.Get(vaultType)
	if err != nil {
		return nil, fmt.Errorf("vault plugin: %w", err)
	}

	resp, err := provider.GetSecret(ctx, &vaultv1.GetSecretRequest{Path: path, Opts: opts})
	if err != nil {
		if st, ok := status.FromError(err); ok && st.Code() != codes.OK {
			return nil, status.Errorf(st.Code(), "fetching secret: %s", st.Message())
		}
		return nil, status.Errorf(codes.Internal, "fetching secret: %s", err.Error())
	}

	s.store.CacheSecret(req.SessionId, cacheKey, resp.Secret)
	log.Info().
		Str("session_id", sdksession.MaskSessionId(req.SessionId)).
		Str("vault", vaultType).
		Str("path", path).
		Msg("secret retrieved and cached")

	return &locksmithv1.GetSecretResponse{Secret: resp.Secret, ContentType: resp.ContentType}, nil
}

// VaultList returns info for all loaded vault plugins.
func (s *Server) VaultList(_ context.Context, _ *locksmithv1.VaultListRequest) (*locksmithv1.VaultListResponse, error) {
	if s.plugins == nil {
		return &locksmithv1.VaultListResponse{}, nil
	}
	var vaults []*locksmithv1.VaultInfo
	for _, vaultType := range s.plugins.Types() {
		info := &locksmithv1.VaultInfo{Name: vaultType, Type: vaultType}
		if pi := s.plugins.CachedInfo(vaultType); pi != nil {
			info.Version = pi.Version
			info.Platforms = pi.Platforms
		} else if provider, err := s.plugins.Get(vaultType); err == nil {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			if pi, err := provider.Info(ctx, &vaultv1.InfoRequest{}); err == nil {
				info.Version = pi.Version
				info.Platforms = pi.Platforms
			}
			cancel()
		}
		vaults = append(vaults, info)
	}
	return &locksmithv1.VaultListResponse{Vaults: vaults}, nil
}

// VaultHealth returns availability status for all loaded vault plugins.
func (s *Server) VaultHealth(
	_ context.Context,
	_ *locksmithv1.VaultHealthRequest,
) (*locksmithv1.VaultHealthResponse, error) {
	if s.plugins == nil {
		return &locksmithv1.VaultHealthResponse{}, nil
	}
	var results []*locksmithv1.VaultHealthInfo
	for _, vaultType := range s.plugins.Types() {
		result := &locksmithv1.VaultHealthInfo{Name: vaultType}
		if provider, err := s.plugins.Get(vaultType); err != nil {
			result.Message = err.Error()
		} else {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			if h, err := provider.HealthCheck(ctx, &vaultv1.HealthCheckRequest{}); err != nil {
				result.Message = err.Error()
			} else {
				result.Available = h.Available
				result.Message = h.Message
			}
			cancel()
		}
		for _, w := range s.plugins.Warnings(vaultType) {
			result.CompatWarnings = append(result.CompatWarnings,
				fmt.Sprintf("%s: %s", w.Kind, w.Message))
		}
		results = append(results, result)
	}
	return &locksmithv1.VaultHealthResponse{Vaults: results}, nil
}

// ReloadConfig triggers a config reload on the daemon and returns the result.
func (s *Server) ReloadConfig(
	_ context.Context,
	_ *locksmithv1.ReloadConfigRequest,
) (*locksmithv1.ReloadConfigResponse, error) {
	if s.reloader == nil {
		return nil, status.Error(codes.Unimplemented, "reload not available")
	}
	if err := s.reloader.Reload(); err != nil {
		return nil, status.Errorf(codes.Internal, "reload failed: %s", err)
	}
	return &locksmithv1.ReloadConfigResponse{Message: "config reloaded"}, nil
}

// resolveKey returns vault type, secret path, and extra opts for a GetSecret request.
// Supports both key alias lookup and direct vault+path fallback.
func (s *Server) resolveKey(
	req *locksmithv1.GetSecretRequest,
) (vaultType, path string, opts map[string]string, err error) {
	cfg := s.cfgFn()
	opts = make(map[string]string)
	if req.KeyAlias != "" {
		keyDef, ok := cfg.Keys[req.KeyAlias]
		if !ok {
			return "", "", nil, fmt.Errorf("unknown key alias %q", req.KeyAlias)
		}
		vaultDef, ok := cfg.Vaults[keyDef.Vault]
		if !ok {
			return "", "", nil, fmt.Errorf("key %q references unknown vault %q", req.KeyAlias, keyDef.Vault)
		}
		if vaultDef.Store != "" {
			opts["store"] = vaultDef.Store
		}
		if vaultDef.Service != "" {
			opts["service"] = vaultDef.Service
		}
		return vaultDef.Type, keyDef.Path, opts, nil
	}
	if req.VaultName == "" || req.Path == "" {
		return "", "", nil, fmt.Errorf("either key_alias or both vault_name and path are required")
	}
	if vaultDef, ok := cfg.Vaults[req.VaultName]; ok {
		if vaultDef.Store != "" {
			opts["store"] = vaultDef.Store
		}
		if vaultDef.Service != "" {
			opts["service"] = vaultDef.Service
		}
		return vaultDef.Type, req.Path, opts, nil
	}
	return req.VaultName, req.Path, opts, nil
}

// parseTTL returns the requested duration, falling back to defaultTTL when requested is empty.
func parseTTL(requested, defaultTTL string) (time.Duration, error) {
	ttlStr := requested
	if ttlStr == "" {
		ttlStr = defaultTTL
	}
	d, err := time.ParseDuration(ttlStr)
	if err != nil {
		return 0, fmt.Errorf("parsing duration %q: %w", ttlStr, err)
	}
	return d, nil
}
