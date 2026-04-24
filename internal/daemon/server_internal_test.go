package daemon

import (
	"context"
	"fmt"
	"testing"

	locksmithv1 "github.com/lorem-dev/locksmith/gen/proto/locksmith/v1"
	vaultv1 "github.com/lorem-dev/locksmith/gen/proto/vault/v1"
	"github.com/lorem-dev/locksmith/internal/config"
	"github.com/lorem-dev/locksmith/internal/session"
	sdkerrors "github.com/lorem-dev/locksmith/sdk/errors"
	"github.com/lorem-dev/locksmith/sdk/vault"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// mockRegistry implements pluginRegistry for testing.
type mockRegistry struct {
	providers map[string]vault.Provider
}

func (m *mockRegistry) Get(vaultType string) (vault.Provider, error) {
	p, ok := m.providers[vaultType]
	if !ok {
		return nil, fmt.Errorf("no plugin for %q", vaultType)
	}
	return p, nil
}

func (m *mockRegistry) Types() []string {
	types := make([]string, 0, len(m.providers))
	for t := range m.providers {
		types = append(types, t)
	}
	return types
}

// mockProvider implements vault.Provider for testing.
type mockProvider struct {
	secret      []byte
	contentType string
	available   bool
	version     string
	platforms   []string
	getErr      error
	healthErr   error
	// lastReq is set to the most recent GetSecret request for opts inspection.
	lastReq *vaultv1.GetSecretRequest
}

func (p *mockProvider) GetSecret(_ context.Context, req *vaultv1.GetSecretRequest) (*vaultv1.GetSecretResponse, error) {
	p.lastReq = req
	if p.getErr != nil {
		return nil, p.getErr
	}
	return &vaultv1.GetSecretResponse{Secret: p.secret, ContentType: p.contentType}, nil
}

func (p *mockProvider) HealthCheck(_ context.Context, _ *vaultv1.HealthCheckRequest) (*vaultv1.HealthCheckResponse, error) {
	if p.healthErr != nil {
		return nil, p.healthErr
	}
	return &vaultv1.HealthCheckResponse{Available: p.available, Message: "ok"}, nil
}

func (p *mockProvider) Info(_ context.Context, _ *vaultv1.InfoRequest) (*vaultv1.InfoResponse, error) {
	return &vaultv1.InfoResponse{Version: p.version, Platforms: p.platforms}, nil
}

func newServerWithMock(providers map[string]vault.Provider) *Server {
	cfg := &config.Config{
		Defaults: config.Defaults{SessionTTL: "1h"},
		Vaults:   map[string]config.Vault{"keychain": {Type: "keychain"}},
		Keys:     map[string]config.Key{"test-key": {Vault: "keychain", Path: "test-path"}},
	}
	reg := &mockRegistry{providers: providers}
	return &Server{cfg: cfg, store: session.NewStore(), plugins: reg}
}

func TestVaultList_WithPlugins(t *testing.T) {
	srv := newServerWithMock(map[string]vault.Provider{
		"keychain": &mockProvider{version: "1.0.0", platforms: []string{"darwin"}},
	})
	resp, err := srv.VaultList(context.Background(), &locksmithv1.VaultListRequest{})
	if err != nil {
		t.Fatalf("VaultList() error: %v", err)
	}
	if len(resp.Vaults) != 1 {
		t.Fatalf("vaults = %d, want 1", len(resp.Vaults))
	}
	if resp.Vaults[0].Type != "keychain" {
		t.Errorf("vault type = %q, want keychain", resp.Vaults[0].Type)
	}
	if resp.Vaults[0].Version != "1.0.0" {
		t.Errorf("version = %q, want 1.0.0", resp.Vaults[0].Version)
	}
}

func TestVaultHealth_WithPlugins(t *testing.T) {
	srv := newServerWithMock(map[string]vault.Provider{
		"keychain": &mockProvider{available: true},
	})
	resp, err := srv.VaultHealth(context.Background(), &locksmithv1.VaultHealthRequest{})
	if err != nil {
		t.Fatalf("VaultHealth() error: %v", err)
	}
	if len(resp.Vaults) != 1 {
		t.Fatalf("vaults = %d, want 1", len(resp.Vaults))
	}
	if !resp.Vaults[0].Available {
		t.Error("vault not available, want available")
	}
}

func TestVaultHealth_PluginError(t *testing.T) {
	srv := newServerWithMock(map[string]vault.Provider{
		"keychain": &mockProvider{healthErr: fmt.Errorf("vault unavailable")},
	})
	resp, err := srv.VaultHealth(context.Background(), &locksmithv1.VaultHealthRequest{})
	if err != nil {
		t.Fatalf("VaultHealth() error: %v", err)
	}
	if len(resp.Vaults) != 1 {
		t.Fatalf("vaults = %d, want 1", len(resp.Vaults))
	}
	if resp.Vaults[0].Available {
		t.Error("vault available, want unavailable")
	}
}

func TestGetSecret_FromVault(t *testing.T) {
	srv := newServerWithMock(map[string]vault.Provider{
		"keychain": &mockProvider{secret: []byte("s3cr3t"), contentType: "text/plain"},
	})
	startResp, _ := srv.SessionStart(context.Background(), &locksmithv1.SessionStartRequest{})
	resp, err := srv.GetSecret(context.Background(), &locksmithv1.GetSecretRequest{
		SessionId: startResp.SessionId,
		KeyAlias:  "test-key",
	})
	if err != nil {
		t.Fatalf("GetSecret() error: %v", err)
	}
	if string(resp.Secret) != "s3cr3t" {
		t.Errorf("secret = %q, want s3cr3t", resp.Secret)
	}
}

func TestGetSecret_FromCache(t *testing.T) {
	provider := &mockProvider{secret: []byte("s3cr3t"), contentType: "text/plain"}
	srv := newServerWithMock(map[string]vault.Provider{"keychain": provider})
	startResp, _ := srv.SessionStart(context.Background(), &locksmithv1.SessionStartRequest{})

	// First call populates cache.
	_, err := srv.GetSecret(context.Background(), &locksmithv1.GetSecretRequest{
		SessionId: startResp.SessionId,
		KeyAlias:  "test-key",
	})
	if err != nil {
		t.Fatalf("first GetSecret() error: %v", err)
	}

	// Second call should hit cache; change provider to return error to verify.
	provider.getErr = fmt.Errorf("should not be called")
	resp, err := srv.GetSecret(context.Background(), &locksmithv1.GetSecretRequest{
		SessionId: startResp.SessionId,
		KeyAlias:  "test-key",
	})
	if err != nil {
		t.Fatalf("second GetSecret() (from cache) error: %v", err)
	}
	if string(resp.Secret) != "s3cr3t" {
		t.Errorf("cached secret = %q, want s3cr3t", resp.Secret)
	}
}

func TestGetSecret_VaultError(t *testing.T) {
	srv := newServerWithMock(map[string]vault.Provider{
		"keychain": &mockProvider{getErr: fmt.Errorf("vault failure")},
	})
	startResp, _ := srv.SessionStart(context.Background(), &locksmithv1.SessionStartRequest{})
	_, err := srv.GetSecret(context.Background(), &locksmithv1.GetSecretRequest{
		SessionId: startResp.SessionId,
		KeyAlias:  "test-key",
	})
	if err == nil {
		t.Fatal("GetSecret() expected error on vault failure")
	}
}

func TestGetSecret_UnknownVaultPlugin(t *testing.T) {
	// Registry has no "keychain" plugin loaded despite config referencing it.
	srv := newServerWithMock(map[string]vault.Provider{})
	startResp, _ := srv.SessionStart(context.Background(), &locksmithv1.SessionStartRequest{})
	_, err := srv.GetSecret(context.Background(), &locksmithv1.GetSecretRequest{
		SessionId: startResp.SessionId,
		KeyAlias:  "test-key",
	})
	if err == nil {
		t.Fatal("GetSecret() expected error for unloaded vault plugin")
	}
}

func TestSessionStart_InvalidTTL(t *testing.T) {
	cfg := &config.Config{
		Defaults: config.Defaults{SessionTTL: "invalid"},
		Vaults:   map[string]config.Vault{},
		Keys:     map[string]config.Key{},
	}
	srv := &Server{cfg: cfg, store: session.NewStore()}
	_, err := srv.SessionStart(context.Background(), &locksmithv1.SessionStartRequest{})
	if err == nil {
		t.Fatal("SessionStart() expected error for invalid default TTL")
	}
}

func TestSessionEnd_NotFound(t *testing.T) {
	cfg := &config.Config{
		Defaults: config.Defaults{SessionTTL: "1h"},
		Vaults:   map[string]config.Vault{},
		Keys:     map[string]config.Key{},
	}
	srv := &Server{cfg: cfg, store: session.NewStore()}
	_, err := srv.SessionEnd(context.Background(), &locksmithv1.SessionEndRequest{SessionIdPrefix: "ls_notfound"})
	if err == nil {
		t.Fatal("SessionEnd() expected error for unknown session")
	}
}

func TestResolveKey_UnknownVault(t *testing.T) {
	// Key references a vault that is not configured.
	cfg := &config.Config{
		Defaults: config.Defaults{SessionTTL: "1h"},
		Vaults:   map[string]config.Vault{},
		Keys:     map[string]config.Key{"orphan-key": {Vault: "missing-vault", Path: "p"}},
	}
	store := session.NewStore()
	srvForSession := &Server{cfg: &config.Config{Defaults: config.Defaults{SessionTTL: "1h"}, Vaults: map[string]config.Vault{}, Keys: map[string]config.Key{}}, store: store}
	startResp, _ := srvForSession.SessionStart(context.Background(), &locksmithv1.SessionStartRequest{})

	srv := &Server{cfg: cfg, store: store}
	_, err := srv.GetSecret(context.Background(), &locksmithv1.GetSecretRequest{
		SessionId: startResp.SessionId,
		KeyAlias:  "orphan-key",
	})
	if err == nil {
		t.Fatal("GetSecret() expected error for key with unknown vault")
	}
}

func TestGetSecret_WithStoreInOpts(t *testing.T) {
	// Vault has a Store field - opts["store"] path.
	cfg := &config.Config{
		Defaults: config.Defaults{SessionTTL: "1h"},
		Vaults:   map[string]config.Vault{"myvault": {Type: "keychain", Store: "mystore"}},
		Keys:     map[string]config.Key{"mykey": {Vault: "myvault", Path: "some/path"}},
	}
	provider := &mockProvider{secret: []byte("value"), contentType: "text/plain"}
	reg := &mockRegistry{providers: map[string]vault.Provider{"keychain": provider}}
	srv := &Server{cfg: cfg, store: session.NewStore(), plugins: reg}

	startResp, _ := srv.SessionStart(context.Background(), &locksmithv1.SessionStartRequest{})
	resp, err := srv.GetSecret(context.Background(), &locksmithv1.GetSecretRequest{
		SessionId: startResp.SessionId,
		KeyAlias:  "mykey",
	})
	if err != nil {
		t.Fatalf("GetSecret() with store opts error: %v", err)
	}
	if string(resp.Secret) != "value" {
		t.Errorf("secret = %q, want value", resp.Secret)
	}
}

func TestGetSecret_DirectVaultWithStore(t *testing.T) {
	// Direct vault+path where vault has a Store.
	cfg := &config.Config{
		Defaults: config.Defaults{SessionTTL: "1h"},
		Vaults:   map[string]config.Vault{"myvault": {Type: "keychain", Store: "mystore"}},
		Keys:     map[string]config.Key{},
	}
	provider := &mockProvider{secret: []byte("direct"), contentType: "text/plain"}
	reg := &mockRegistry{providers: map[string]vault.Provider{"keychain": provider}}
	srv := &Server{cfg: cfg, store: session.NewStore(), plugins: reg}

	startResp, _ := srv.SessionStart(context.Background(), &locksmithv1.SessionStartRequest{})
	resp, err := srv.GetSecret(context.Background(), &locksmithv1.GetSecretRequest{
		SessionId: startResp.SessionId,
		VaultName: "myvault",
		Path:      "direct/path",
	})
	if err != nil {
		t.Fatalf("GetSecret() direct vault with store error: %v", err)
	}
	if string(resp.Secret) != "direct" {
		t.Errorf("secret = %q, want direct", resp.Secret)
	}
}

func TestGetSecret_DirectVaultNotInConfig(t *testing.T) {
	// Direct vault+path where VaultName is not in cfg.Vaults - fallback path.
	cfg := &config.Config{
		Defaults: config.Defaults{SessionTTL: "1h"},
		Vaults:   map[string]config.Vault{},
		Keys:     map[string]config.Key{},
	}
	provider := &mockProvider{secret: []byte("raw"), contentType: "text/plain"}
	reg := &mockRegistry{providers: map[string]vault.Provider{"rawvault": provider}}
	srv := &Server{cfg: cfg, store: session.NewStore(), plugins: reg}

	startResp, _ := srv.SessionStart(context.Background(), &locksmithv1.SessionStartRequest{})
	resp, err := srv.GetSecret(context.Background(), &locksmithv1.GetSecretRequest{
		SessionId: startResp.SessionId,
		VaultName: "rawvault",
		Path:      "raw/path",
	})
	if err != nil {
		t.Fatalf("GetSecret() direct vault not in config error: %v", err)
	}
	if string(resp.Secret) != "raw" {
		t.Errorf("secret = %q, want raw", resp.Secret)
	}
}

// failingMockRegistry implements pluginRegistry with Types() returning an entry
// but Get() failing - exercises the VaultHealth manager.Get error branch.
type failingMockRegistry struct{}

func (f *failingMockRegistry) Get(_ string) (vault.Provider, error) {
	return nil, fmt.Errorf("get failed")
}

func (f *failingMockRegistry) Types() []string {
	return []string{"broken"}
}

func TestVaultHealth_GetError(t *testing.T) {
	cfg := &config.Config{
		Defaults: config.Defaults{SessionTTL: "1h"},
		Vaults:   map[string]config.Vault{},
		Keys:     map[string]config.Key{},
	}
	srv := &Server{cfg: cfg, store: session.NewStore(), plugins: &failingMockRegistry{}}
	resp, err := srv.VaultHealth(context.Background(), &locksmithv1.VaultHealthRequest{})
	if err != nil {
		t.Fatalf("VaultHealth() error: %v", err)
	}
	if len(resp.Vaults) != 1 {
		t.Fatalf("vaults = %d, want 1", len(resp.Vaults))
	}
	if resp.Vaults[0].Message == "" {
		t.Error("expected error message in vault health info")
	}
}

func TestGetSecret_PreservesVaultErrorCode(t *testing.T) {
	cfg := &config.Config{
		Defaults: config.Defaults{SessionTTL: "1h"},
		Vaults:   map[string]config.Vault{"keychain": {Type: "keychain"}},
		Keys:     map[string]config.Key{"notion": {Vault: "keychain", Path: "notion"}},
	}
	provider := &mockProvider{getErr: sdkerrors.NotFoundError("keychain: item not found")}
	reg := &mockRegistry{providers: map[string]vault.Provider{"keychain": provider}}
	srv := NewServerWithRegistry(cfg, session.NewStore(), reg)

	startResp, _ := srv.SessionStart(context.Background(), &locksmithv1.SessionStartRequest{})
	_, err := srv.GetSecret(context.Background(), &locksmithv1.GetSecretRequest{
		SessionId: startResp.SessionId,
		KeyAlias:  "notion",
	})
	if err == nil {
		t.Fatal("expected error")
	}
	s, ok := status.FromError(err)
	if !ok {
		t.Fatalf("error is not gRPC status: %T %v", err, err)
	}
	if s.Code() != codes.NotFound {
		t.Errorf("Code() = %v, want NotFound", s.Code())
	}
}

func TestResolveKey_PassesServiceOpt(t *testing.T) {
	cfg := &config.Config{
		Defaults: config.Defaults{SessionTTL: "1h"},
		Vaults:   map[string]config.Vault{"keychain": {Type: "keychain", Service: "com.example.app"}},
		Keys:     map[string]config.Key{"mykey": {Vault: "keychain", Path: "myaccount"}},
	}
	provider := &mockProvider{secret: []byte("secret"), contentType: "text/plain"}
	reg := &mockRegistry{providers: map[string]vault.Provider{"keychain": provider}}
	srv := NewServerWithRegistry(cfg, session.NewStore(), reg)

	startResp, _ := srv.SessionStart(context.Background(), &locksmithv1.SessionStartRequest{})
	_, err := srv.GetSecret(context.Background(), &locksmithv1.GetSecretRequest{
		SessionId: startResp.SessionId,
		KeyAlias:  "mykey",
	})
	if err != nil {
		t.Fatalf("GetSecret() error: %v", err)
	}
	if provider.lastReq == nil {
		t.Fatal("provider was not called")
	}
	if provider.lastReq.Opts["service"] != "com.example.app" {
		t.Errorf("opts[service] = %q, want %q", provider.lastReq.Opts["service"], "com.example.app")
	}
}

func TestResolveKey_DirectVault_PassesServiceOpt(t *testing.T) {
	cfg := &config.Config{
		Defaults: config.Defaults{SessionTTL: "1h"},
		Vaults:   map[string]config.Vault{"keychain": {Type: "keychain", Service: "com.example.direct"}},
		Keys:     map[string]config.Key{},
	}
	provider := &mockProvider{secret: []byte("secret"), contentType: "text/plain"}
	reg := &mockRegistry{providers: map[string]vault.Provider{"keychain": provider}}
	srv := NewServerWithRegistry(cfg, session.NewStore(), reg)

	startResp, _ := srv.SessionStart(context.Background(), &locksmithv1.SessionStartRequest{})
	_, err := srv.GetSecret(context.Background(), &locksmithv1.GetSecretRequest{
		SessionId: startResp.SessionId,
		VaultName: "keychain",
		Path:      "someaccount",
	})
	if err != nil {
		t.Fatalf("GetSecret() error: %v", err)
	}
	if provider.lastReq.Opts["service"] != "com.example.direct" {
		t.Errorf("opts[service] = %q, want %q", provider.lastReq.Opts["service"], "com.example.direct")
	}
}
