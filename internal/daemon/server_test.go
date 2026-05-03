package daemon_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	locksmithv1 "github.com/lorem-dev/locksmith/gen/proto/locksmith/v1"
	vaultv1 "github.com/lorem-dev/locksmith/gen/proto/vault/v1"
	"github.com/lorem-dev/locksmith/internal/config"
	"github.com/lorem-dev/locksmith/internal/daemon"
	"github.com/lorem-dev/locksmith/internal/log"
	pluginpkg "github.com/lorem-dev/locksmith/internal/plugin"
	"github.com/lorem-dev/locksmith/internal/session"
	sdklog "github.com/lorem-dev/locksmith/sdk/log"
	"github.com/lorem-dev/locksmith/sdk/vault"
)

func TestMain(m *testing.M) {
	log.Init(io.Discard, "error", "text")
	os.Exit(m.Run())
}

func newTestServer() *daemon.Server {
	cfg := &config.Config{
		Defaults: config.Defaults{SessionTTL: "1h"},
		Vaults:   map[string]config.Vault{"keychain": {Type: "keychain"}},
		Keys:     map[string]config.Key{"test-key": {Vault: "keychain", Path: "test-path"}},
	}
	return daemon.NewServer(func() *config.Config { return cfg }, session.NewStore(), nil, nil)
}

func TestSessionStart(t *testing.T) {
	srv := newTestServer()
	resp, err := srv.SessionStart(context.Background(), &locksmithv1.SessionStartRequest{Ttl: "2h"})
	if err != nil {
		t.Fatalf("SessionStart() error: %v", err)
	}
	if resp.SessionId == "" {
		t.Error("SessionId is empty")
	}
	if resp.ExpiresAt == "" {
		t.Error("ExpiresAt is empty")
	}
}

func TestSessionStart_DefaultTTL(t *testing.T) {
	srv := newTestServer()
	resp, err := srv.SessionStart(context.Background(), &locksmithv1.SessionStartRequest{})
	if err != nil {
		t.Fatalf("SessionStart() error: %v", err)
	}
	parsed, _ := time.Parse(time.RFC3339, resp.ExpiresAt)
	if time.Until(parsed) < 59*time.Minute {
		t.Error("default TTL not applied correctly")
	}
}

func TestSessionEnd(t *testing.T) {
	srv := newTestServer()
	startResp, _ := srv.SessionStart(context.Background(), &locksmithv1.SessionStartRequest{})
	endResp, err := srv.SessionEnd(
		context.Background(),
		&locksmithv1.SessionEndRequest{SessionIdPrefix: startResp.SessionId},
	)
	if err != nil {
		t.Fatalf("SessionEnd() error: %v", err)
	}
	if endResp.SessionId != startResp.SessionId {
		t.Errorf("session ID mismatch: got %s, want %s", endResp.SessionId, startResp.SessionId)
	}
	listResp, _ := srv.SessionList(context.Background(), &locksmithv1.SessionListRequest{})
	if len(listResp.Sessions) != 0 {
		t.Errorf("sessions after end = %d, want 0", len(listResp.Sessions))
	}
}

func TestSessionEnd_Prefix(t *testing.T) {
	srv := newTestServer()
	startResp, _ := srv.SessionStart(context.Background(), &locksmithv1.SessionStartRequest{})
	endResp, err := srv.SessionEnd(
		context.Background(),
		&locksmithv1.SessionEndRequest{SessionIdPrefix: startResp.SessionId[:5]},
	)
	if err != nil {
		t.Fatalf("SessionEnd() error: %v", err)
	}
	if endResp.SessionId != startResp.SessionId {
		t.Errorf("session ID mismatch: got %s, want %s", endResp.SessionId, startResp.SessionId)
	}
}

func TestSessionEnd_NotFound(t *testing.T) {
	srv := newTestServer()
	_, err := srv.SessionEnd(context.Background(), &locksmithv1.SessionEndRequest{SessionIdPrefix: "ls_nonexistent"})
	want := `deleting session: session for prefix "ls_nonexistent" not found`
	if err == nil || err.Error() != want {
		t.Fatalf("SessionEnd() error = %v, want '%s'", err, want)
	}
}

func TestSessionEnd_Multiple(t *testing.T) {
	srv := newTestServer()
	srv.SessionStart(context.Background(), &locksmithv1.SessionStartRequest{})
	srv.SessionStart(context.Background(), &locksmithv1.SessionStartRequest{})
	_, err := srv.SessionEnd(context.Background(), &locksmithv1.SessionEndRequest{SessionIdPrefix: "ls_"})
	want := `deleting session: multiple sessions found for prefix "ls_"`
	if err == nil || err.Error() != want {
		t.Fatalf("SessionEnd() error = %v, want '%s'", err, want)
	}
}

func TestSessionList(t *testing.T) {
	srv := newTestServer()
	srv.SessionStart(context.Background(), &locksmithv1.SessionStartRequest{})
	srv.SessionStart(context.Background(), &locksmithv1.SessionStartRequest{})
	resp, err := srv.SessionList(context.Background(), &locksmithv1.SessionListRequest{})
	if err != nil {
		t.Fatalf("SessionList() error: %v", err)
	}
	if len(resp.Sessions) != 2 {
		t.Errorf("sessions = %d, want 2", len(resp.Sessions))
	}
}

func TestGetSecret_NoSession(t *testing.T) {
	srv := newTestServer()
	_, err := srv.GetSecret(context.Background(), &locksmithv1.GetSecretRequest{
		SessionId: "ls_nonexistent",
		KeyAlias:  "test-key",
	})
	if err == nil {
		t.Fatal("GetSecret() expected error with invalid session")
	}
}

func TestGetSecret_UnknownAlias(t *testing.T) {
	srv := newTestServer()
	startResp, _ := srv.SessionStart(context.Background(), &locksmithv1.SessionStartRequest{})
	_, err := srv.GetSecret(context.Background(), &locksmithv1.GetSecretRequest{
		SessionId: startResp.SessionId,
		KeyAlias:  "nonexistent-alias",
	})
	if err == nil {
		t.Fatal("GetSecret() expected error for unknown key alias")
	}
}

func TestNewDaemon(t *testing.T) {
	cfg := &config.Config{
		Defaults: config.Defaults{SessionTTL: "1h", SocketPath: "/tmp/test.sock"},
		Vaults:   map[string]config.Vault{},
		Keys:     map[string]config.Key{},
	}
	d := daemon.New(cfg, "")
	if d == nil {
		t.Fatal("New() returned nil")
	}
}

func TestVaultList_NoPlugins(t *testing.T) {
	srv := newTestServer()
	resp, err := srv.VaultList(context.Background(), &locksmithv1.VaultListRequest{})
	if err != nil {
		t.Fatalf("VaultList() error: %v", err)
	}
	if len(resp.Vaults) != 0 {
		t.Errorf("vaults = %d, want 0", len(resp.Vaults))
	}
}

func TestVaultHealth_NoPlugins(t *testing.T) {
	srv := newTestServer()
	resp, err := srv.VaultHealth(context.Background(), &locksmithv1.VaultHealthRequest{})
	if err != nil {
		t.Fatalf("VaultHealth() error: %v", err)
	}
	if len(resp.Vaults) != 0 {
		t.Errorf("vaults = %d, want 0", len(resp.Vaults))
	}
}

func TestGetSecret_NoPluginManager(t *testing.T) {
	srv := newTestServer()
	startResp, _ := srv.SessionStart(context.Background(), &locksmithv1.SessionStartRequest{})
	_, err := srv.GetSecret(context.Background(), &locksmithv1.GetSecretRequest{
		SessionId: startResp.SessionId,
		KeyAlias:  "test-key",
	})
	if err == nil {
		t.Fatal("GetSecret() expected error when plugin manager is nil")
	}
}

func TestResolveKey_DirectVaultPath(t *testing.T) {
	srv := newTestServer()
	startResp, _ := srv.SessionStart(context.Background(), &locksmithv1.SessionStartRequest{})
	// Should fail because plugins == nil, but should not fail on key resolution
	_, err := srv.GetSecret(context.Background(), &locksmithv1.GetSecretRequest{
		SessionId: startResp.SessionId,
		VaultName: "keychain",
		Path:      "some/path",
	})
	// Expect error about no plugin manager, not about resolution
	if err == nil {
		t.Fatal("GetSecret() expected error (no plugin manager)")
	}
	if err.Error() == "either key_alias or both vault_name and path are required" {
		t.Fatal("GetSecret() failed at key resolution, not plugin manager")
	}
}

func TestResolveKey_MissingBothAliasAndPath(t *testing.T) {
	srv := newTestServer()
	startResp, _ := srv.SessionStart(context.Background(), &locksmithv1.SessionStartRequest{})
	_, err := srv.GetSecret(context.Background(), &locksmithv1.GetSecretRequest{
		SessionId: startResp.SessionId,
		// No KeyAlias, no VaultName, no Path
	})
	if err == nil {
		t.Fatal("GetSecret() expected error for missing alias and path")
	}
}

func TestSessionStart_LogsMaskedSessionId(t *testing.T) {
	var buf bytes.Buffer
	_, _ = sdklog.NewLogWriter(sdklog.LogConfig{Level: "info"})
	log.Init(&buf, "info", "json")
	defer log.Init(io.Discard, "error", "text")

	srv := newTestServer()
	_, err := srv.SessionStart(context.Background(), &locksmithv1.SessionStartRequest{Ttl: "1h"})
	if err != nil {
		t.Fatalf("SessionStart() error: %v", err)
	}

	var entry map[string]interface{}
	if err := json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &entry); err != nil {
		t.Fatalf("log output is not valid JSON: %v — output: %q", err, buf.String())
	}
	sessionID, ok := entry["session_id"].(string)
	if !ok {
		t.Fatalf("log entry missing session_id field, got: %v", entry)
	}
	if !strings.Contains(sessionID, "****") {
		t.Errorf("session_id in log is not masked at info level: %q", sessionID)
	}
}

func TestSessionStart_LogsFullSessionIdInDebug(t *testing.T) {
	var buf bytes.Buffer
	_, _ = sdklog.NewLogWriter(sdklog.LogConfig{Level: "debug"})
	log.Init(&buf, "debug", "json")
	defer func() {
		_, _ = sdklog.NewLogWriter(sdklog.LogConfig{Level: "info"})
		log.Init(io.Discard, "error", "text")
	}()

	srv := newTestServer()
	resp, err := srv.SessionStart(context.Background(), &locksmithv1.SessionStartRequest{Ttl: "1h"})
	if err != nil {
		t.Fatalf("SessionStart() error: %v", err)
	}

	var entry map[string]interface{}
	for _, line := range strings.Split(strings.TrimSpace(buf.String()), "\n") {
		if line == "" {
			continue
		}
		var e map[string]interface{}
		if json.Unmarshal([]byte(line), &e) == nil {
			if e["message"] == "session started" {
				entry = e
				break
			}
		}
	}
	if entry == nil {
		t.Fatalf("no 'session started' log entry found in output: %q", buf.String())
	}
	sessionID, ok := entry["session_id"].(string)
	if !ok {
		t.Fatalf("log entry missing session_id field: %v", entry)
	}
	if sessionID != resp.SessionId {
		t.Errorf("session_id in debug log should be full ID\ngot:  %q\nwant: %q", sessionID, resp.SessionId)
	}
}

// stubReloader records Reload() calls for testing.
type stubReloader struct {
	mu     sync.Mutex
	calls  int
	retErr error
}

func (r *stubReloader) Reload() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls++
	return r.retErr
}

func TestReloadConfig_RPC_Success(t *testing.T) {
	cfg := &config.Config{
		Defaults: config.Defaults{SessionTTL: "1h"},
		Vaults:   map[string]config.Vault{},
		Keys:     map[string]config.Key{},
	}
	r := &stubReloader{}
	srv := daemon.NewServerWithRegistry(
		func() *config.Config { return cfg },
		session.NewStore(),
		nil,
		r,
	)
	resp, err := srv.ReloadConfig(context.Background(), &locksmithv1.ReloadConfigRequest{})
	if err != nil {
		t.Fatalf("ReloadConfig() error: %v", err)
	}
	if resp.Message == "" {
		t.Error("expected non-empty message")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.calls != 1 {
		t.Errorf("Reload() called %d times, want 1", r.calls)
	}
}

func TestReloadConfig_RPC_Error(t *testing.T) {
	cfg := &config.Config{
		Defaults: config.Defaults{SessionTTL: "1h"},
		Vaults:   map[string]config.Vault{},
		Keys:     map[string]config.Key{},
	}
	r := &stubReloader{retErr: errors.New("bad config")}
	srv := daemon.NewServerWithRegistry(
		func() *config.Config { return cfg },
		session.NewStore(),
		nil,
		r,
	)
	_, err := srv.ReloadConfig(context.Background(), &locksmithv1.ReloadConfigRequest{})
	if err == nil {
		t.Fatal("expected error from ReloadConfig when Reload fails")
	}
	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.Internal {
		t.Errorf("expected codes.Internal, got %v", err)
	}
}

func TestReloadConfig_RPC_NilReloader(t *testing.T) {
	cfg := &config.Config{
		Defaults: config.Defaults{SessionTTL: "1h"},
		Vaults:   map[string]config.Vault{},
		Keys:     map[string]config.Key{},
	}
	srv := daemon.NewServer(func() *config.Config { return cfg }, session.NewStore(), nil, nil)
	_, err := srv.ReloadConfig(context.Background(), &locksmithv1.ReloadConfigRequest{})
	if err == nil {
		t.Fatal("expected error when reloader is nil")
	}
	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.Unimplemented {
		t.Errorf("expected codes.Unimplemented, got %v", err)
	}
}

// fakeProvider is a minimal vault.Provider for use in fakeRegistry.
type fakeProvider struct {
	vault.Provider

	healthAvailable bool
	healthMessage   string
}

func (f *fakeProvider) HealthCheck(
	_ context.Context,
	_ *vaultv1.HealthCheckRequest,
) (*vaultv1.HealthCheckResponse, error) {
	return &vaultv1.HealthCheckResponse{
		Available: f.healthAvailable,
		Message:   f.healthMessage,
	}, nil
}

func (f *fakeProvider) Info(
	_ context.Context,
	_ *vaultv1.InfoRequest,
) (*vaultv1.InfoResponse, error) {
	return &vaultv1.InfoResponse{}, nil
}

// fakeRegistry implements daemon's pluginRegistry interface for tests.
type fakeRegistry struct {
	types      []string
	provider   vault.Provider
	warnings   []pluginpkg.CompatWarning
	cachedInfo *vaultv1.InfoResponse
}

func (f *fakeRegistry) Get(_ string) (vault.Provider, error) {
	if f.provider == nil {
		return nil, errors.New("no provider")
	}
	return f.provider, nil
}

func (f *fakeRegistry) Types() []string { return f.types }

func (f *fakeRegistry) Warnings(_ string) []pluginpkg.CompatWarning { return f.warnings }

func (f *fakeRegistry) CachedInfo(_ string) *vaultv1.InfoResponse { return f.cachedInfo }

func newRegistryServer(reg *fakeRegistry) *daemon.Server {
	cfg := &config.Config{
		Defaults: config.Defaults{SessionTTL: "1h"},
		Vaults:   map[string]config.Vault{},
		Keys:     map[string]config.Key{},
	}
	return daemon.NewServerWithRegistry(func() *config.Config { return cfg }, session.NewStore(), reg, nil)
}

func TestVaultHealth_IncludesCompatWarnings(t *testing.T) {
	reg := &fakeRegistry{
		types: []string{"x"},
		provider: &fakeProvider{
			healthAvailable: true,
			healthMessage:   "ok",
		},
		warnings: []pluginpkg.CompatWarning{
			{
				Kind:    pluginpkg.WarnPlatformMismatch,
				Message: "supports [darwin] but on linux",
			},
		},
	}
	srv := newRegistryServer(reg)
	resp, err := srv.VaultHealth(context.Background(), &locksmithv1.VaultHealthRequest{})
	if err != nil {
		t.Fatalf("VaultHealth() error: %v", err)
	}
	if len(resp.Vaults) != 1 {
		t.Fatalf("expected 1 vault, got %d", len(resp.Vaults))
	}
	v := resp.Vaults[0]
	if len(v.CompatWarnings) != 1 {
		t.Fatalf("expected 1 compat warning, got %d", len(v.CompatWarnings))
	}
	want := "platform_mismatch: supports [darwin] but on linux"
	if v.CompatWarnings[0] != want {
		t.Errorf("compat warning = %q, want %q", v.CompatWarnings[0], want)
	}
}

func TestVaultList_UsesCachedInfo(t *testing.T) {
	reg := &fakeRegistry{
		types: []string{"x"},
		cachedInfo: &vaultv1.InfoResponse{
			Name:      "x",
			Version:   "9.9.9",
			Platforms: []string{"linux"},
		},
	}
	srv := newRegistryServer(reg)
	resp, err := srv.VaultList(context.Background(), &locksmithv1.VaultListRequest{})
	if err != nil {
		t.Fatalf("VaultList() error: %v", err)
	}
	if len(resp.Vaults) != 1 {
		t.Fatalf("expected 1 vault, got %d", len(resp.Vaults))
	}
	if resp.Vaults[0].Version != "9.9.9" {
		t.Errorf("Version = %q, want %q", resp.Vaults[0].Version, "9.9.9")
	}
}
