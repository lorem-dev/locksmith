package cli_test

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"google.golang.org/grpc"

	locksmithv1 "github.com/lorem-dev/locksmith/gen/proto/locksmith/v1"
	"github.com/lorem-dev/locksmith/internal/cli"
)

type vaultMigrateTestServer struct {
	locksmithv1.UnimplementedLocksmithServiceServer

	mu       sync.Mutex
	getResps map[string][]byte
	getErrs  map[string]error
	setCalls []*locksmithv1.SetSecretRequest
}

func (s *vaultMigrateTestServer) SessionStart(
	_ context.Context,
	_ *locksmithv1.SessionStartRequest,
) (*locksmithv1.SessionStartResponse, error) {
	return &locksmithv1.SessionStartResponse{SessionId: "ls_test", ExpiresAt: "2099-01-01T00:00:00Z"}, nil
}

func (s *vaultMigrateTestServer) GetSecret(
	_ context.Context,
	req *locksmithv1.GetSecretRequest,
) (*locksmithv1.GetSecretResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err, ok := s.getErrs[req.KeyAlias]; ok {
		return nil, err
	}
	val, ok := s.getResps[req.KeyAlias]
	if !ok {
		return nil, fmt.Errorf("no canned response for %s", req.KeyAlias)
	}
	return &locksmithv1.GetSecretResponse{Secret: val, ContentType: "text/plain"}, nil
}

func (s *vaultMigrateTestServer) SetSecret(
	_ context.Context,
	req *locksmithv1.SetSecretRequest,
) (*locksmithv1.SetSecretResponse, error) {
	s.mu.Lock()
	s.setCalls = append(s.setCalls, req)
	s.mu.Unlock()
	return &locksmithv1.SetSecretResponse{}, nil
}

func startMigrateDaemon(t *testing.T, srv locksmithv1.LocksmithServiceServer) string {
	t.Helper()
	dir, err := os.MkdirTemp("/tmp", "lks")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	socketPath := filepath.Join(dir, "m.sock")
	l, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	grpcSrv := grpc.NewServer()
	locksmithv1.RegisterLocksmithServiceServer(grpcSrv, srv)
	go func() { _ = grpcSrv.Serve(l) }()
	t.Cleanup(func() { grpcSrv.Stop() })
	return socketPath
}

func writeMigrateConfig(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(migrateConfig), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}

const migrateConfig = `
defaults:
  session_ttl: 1h
vaults:
  keychain-main:
    type: keychain
    service: com.test
  gopass-main:
    type: gopass
keys:
  github-token:
    vault: keychain-main
    path: github-token
  slack-token:
    vault: keychain-main
    path: slack-token
  gopass-key:
    vault: gopass-main
    path: gopass/key
`

func TestVaultMigrate_Single(t *testing.T) {
	srv := &vaultMigrateTestServer{
		getResps: map[string][]byte{"github-token": []byte("ghp_xxx")},
	}
	socketPath := startMigrateDaemon(t, srv)
	t.Setenv("LOCKSMITH_SOCKET", socketPath)
	t.Setenv("LOCKSMITH_SESSION", "ls_test")
	cfgPath := writeMigrateConfig(t)

	root := cli.NewRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetArgs([]string{"--config", cfgPath, "vault", "migrate", "github-token"})

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	srv.mu.Lock()
	defer srv.mu.Unlock()
	if len(srv.setCalls) != 1 {
		t.Fatalf("SetSecret called %d times, want 1", len(srv.setCalls))
	}
	if string(srv.setCalls[0].Secret) != "ghp_xxx" {
		t.Errorf("daemon got secret %q, want ghp_xxx", srv.setCalls[0].Secret)
	}
	if !srv.setCalls[0].Force {
		t.Error("migrate should set Force=true")
	}
	if !strings.Contains(out.String(), "migrated github-token") {
		t.Errorf("output = %q", out.String())
	}
}

func TestVaultMigrate_All(t *testing.T) {
	srv := &vaultMigrateTestServer{
		getResps: map[string][]byte{
			"github-token": []byte("v1"),
			"slack-token":  []byte("v2"),
		},
	}
	socketPath := startMigrateDaemon(t, srv)
	t.Setenv("LOCKSMITH_SOCKET", socketPath)
	t.Setenv("LOCKSMITH_SESSION", "ls_test")
	cfgPath := writeMigrateConfig(t)

	root := cli.NewRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetArgs([]string{"--config", cfgPath, "vault", "migrate", "--all"})

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	srv.mu.Lock()
	defer srv.mu.Unlock()
	if len(srv.setCalls) != 2 {
		t.Errorf("SetSecret called %d times, want 2 (keychain aliases only)", len(srv.setCalls))
	}
	if !strings.Contains(out.String(), "migrated 2/2") {
		t.Errorf("output = %q", out.String())
	}
}

func TestVaultMigrate_DryRun(t *testing.T) {
	srv := &vaultMigrateTestServer{}
	socketPath := startMigrateDaemon(t, srv)
	t.Setenv("LOCKSMITH_SOCKET", socketPath)
	t.Setenv("LOCKSMITH_SESSION", "ls_test")
	cfgPath := writeMigrateConfig(t)

	root := cli.NewRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetArgs([]string{"--config", cfgPath, "vault", "migrate", "--all", "--dry-run"})

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	srv.mu.Lock()
	defer srv.mu.Unlock()
	if len(srv.setCalls) != 0 {
		t.Error("dry-run should not call SetSecret")
	}
	if !strings.Contains(out.String(), "would migrate") {
		t.Errorf("output = %q", out.String())
	}
}

func TestVaultMigrate_NonKeychainAlias(t *testing.T) {
	srv := &vaultMigrateTestServer{}
	socketPath := startMigrateDaemon(t, srv)
	t.Setenv("LOCKSMITH_SOCKET", socketPath)
	t.Setenv("LOCKSMITH_SESSION", "ls_test")
	cfgPath := writeMigrateConfig(t)

	root := cli.NewRootCmd()
	root.SetArgs([]string{"--config", cfgPath, "vault", "migrate", "gopass-key"})

	err := root.Execute()
	if err == nil || !strings.Contains(err.Error(), "only relevant for keychain") {
		t.Errorf("err = %v, want error mentioning keychain", err)
	}
}

func TestVaultMigrate_UnknownAlias(t *testing.T) {
	srv := &vaultMigrateTestServer{}
	socketPath := startMigrateDaemon(t, srv)
	t.Setenv("LOCKSMITH_SOCKET", socketPath)
	t.Setenv("LOCKSMITH_SESSION", "ls_test")
	cfgPath := writeMigrateConfig(t)

	root := cli.NewRootCmd()
	root.SetArgs([]string{"--config", cfgPath, "vault", "migrate", "no-such-alias"})

	err := root.Execute()
	if err == nil || !strings.Contains(err.Error(), "unknown alias") {
		t.Errorf("err = %v, want unknown alias", err)
	}
}

func TestVaultMigrate_PartialFailure(t *testing.T) {
	srv := &vaultMigrateTestServer{
		getResps: map[string][]byte{"github-token": []byte("v1")},
		getErrs:  map[string]error{"slack-token": fmt.Errorf("user cancelled")},
	}
	socketPath := startMigrateDaemon(t, srv)
	t.Setenv("LOCKSMITH_SOCKET", socketPath)
	t.Setenv("LOCKSMITH_SESSION", "ls_test")
	cfgPath := writeMigrateConfig(t)

	root := cli.NewRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetArgs([]string{"--config", cfgPath, "vault", "migrate", "--all"})

	// Partial failure: returns nil because at least one succeeded.
	_ = root.Execute()
	if !strings.Contains(out.String(), "migrated 1/2") {
		t.Errorf("output = %q, want migrated 1/2", out.String())
	}
	if !strings.Contains(out.String(), "slack-token") {
		t.Errorf("output should name failed alias slack-token: %q", out.String())
	}
}

func TestVaultMigrate_NoArgsNoFlag(t *testing.T) {
	cfgPath := writeMigrateConfig(t)

	root := cli.NewRootCmd()
	root.SetArgs([]string{"--config", cfgPath, "vault", "migrate"})

	err := root.Execute()
	if err == nil || !strings.Contains(err.Error(), "alias or --all") {
		t.Errorf("err = %v, want error about missing alias", err)
	}
}
