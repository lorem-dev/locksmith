package daemon_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/lorem-dev/locksmith/internal/config"
	"github.com/lorem-dev/locksmith/internal/daemon"
	"github.com/lorem-dev/locksmith/internal/log"
	"github.com/lorem-dev/locksmith/internal/session"
	locksmithv1 "github.com/lorem-dev/locksmith/gen/proto/locksmith/v1"
)

func TestMain(m *testing.M) {
	log.Init(log.Config{Level: "error", Format: "text"})
	os.Exit(m.Run())
}

func newTestServer() *daemon.Server {
	cfg := &config.Config{
		Defaults: config.Defaults{SessionTTL: "1h"},
		Vaults:   map[string]config.Vault{"keychain": {Type: "keychain"}},
		Keys:     map[string]config.Key{"test-key": {Vault: "keychain", Path: "test-path"}},
	}
	return daemon.NewServer(cfg, session.NewStore(), nil)
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
	_, err := srv.SessionEnd(context.Background(), &locksmithv1.SessionEndRequest{SessionId: startResp.SessionId})
	if err != nil {
		t.Fatalf("SessionEnd() error: %v", err)
	}
	listResp, _ := srv.SessionList(context.Background(), &locksmithv1.SessionListRequest{})
	if len(listResp.Sessions) != 0 {
		t.Errorf("sessions after end = %d, want 0", len(listResp.Sessions))
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
	d := daemon.New(cfg)
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
