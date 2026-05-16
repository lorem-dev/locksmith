package cli_test

import (
	"bytes"
	"context"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	locksmithv1 "github.com/lorem-dev/locksmith/gen/proto/locksmith/v1"
	"github.com/lorem-dev/locksmith/internal/cli"
)

// vaultSetTestServer records SetSecret calls and answers KeyExists.
type vaultSetTestServer struct {
	locksmithv1.UnimplementedLocksmithServiceServer

	mu             sync.Mutex
	lastSet        *locksmithv1.SetSecretRequest
	keyExistsResp  bool
	keyExistsErr   error
	setSecretErr   error
	sessionStartID string
}

func (s *vaultSetTestServer) SessionStart(
	_ context.Context,
	_ *locksmithv1.SessionStartRequest,
) (*locksmithv1.SessionStartResponse, error) {
	id := s.sessionStartID
	if id == "" {
		id = "ls_test"
	}
	return &locksmithv1.SessionStartResponse{SessionId: id, ExpiresAt: "2099-01-01T00:00:00Z"}, nil
}

func (s *vaultSetTestServer) KeyExists(
	_ context.Context,
	_ *locksmithv1.KeyExistsRequest,
) (*locksmithv1.KeyExistsResponse, error) {
	if s.keyExistsErr != nil {
		return nil, s.keyExistsErr
	}
	return &locksmithv1.KeyExistsResponse{Exists: s.keyExistsResp}, nil
}

func (s *vaultSetTestServer) SetSecret(
	_ context.Context,
	req *locksmithv1.SetSecretRequest,
) (*locksmithv1.SetSecretResponse, error) {
	if s.setSecretErr != nil {
		return nil, s.setSecretErr
	}
	s.mu.Lock()
	s.lastSet = req
	s.mu.Unlock()
	return &locksmithv1.SetSecretResponse{}, nil
}

func startVaultSetDaemon(t *testing.T, srv locksmithv1.LocksmithServiceServer) string {
	t.Helper()
	dir, err := os.MkdirTemp("/tmp", "lks")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	socketPath := filepath.Join(dir, "vs.sock")
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

func TestVaultSet_FromStdin(t *testing.T) {
	srv := &vaultSetTestServer{}
	socketPath := startVaultSetDaemon(t, srv)
	t.Setenv("LOCKSMITH_SOCKET", socketPath)
	t.Setenv("LOCKSMITH_SESSION", "ls_test")

	root := cli.NewRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetIn(strings.NewReader("ghp_xxx\n"))
	root.SetArgs([]string{"vault", "set", "github-token"})

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	srv.mu.Lock()
	got := srv.lastSet
	srv.mu.Unlock()
	if got == nil {
		t.Fatal("SetSecret not called")
		return
	}
	if string(got.Secret) != "ghp_xxx" {
		t.Errorf("secret = %q, want ghp_xxx (trailing newline trimmed)", got.Secret)
	}
	if got.KeyAlias != "github-token" {
		t.Errorf("alias = %q", got.KeyAlias)
	}
	if !strings.Contains(out.String(), "stored github-token") {
		t.Errorf("output = %q", out.String())
	}
}

func TestVaultSet_FromFile(t *testing.T) {
	srv := &vaultSetTestServer{}
	socketPath := startVaultSetDaemon(t, srv)
	t.Setenv("LOCKSMITH_SOCKET", socketPath)
	t.Setenv("LOCKSMITH_SESSION", "ls_test")

	tmp := filepath.Join(t.TempDir(), "secret")
	if err := os.WriteFile(tmp, []byte("from-file\n"), 0o600); err != nil {
		t.Fatalf("write tmp: %v", err)
	}

	root := cli.NewRootCmd()
	root.SetArgs([]string{"vault", "set", "github-token", "--from-file", tmp})

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	srv.mu.Lock()
	got := srv.lastSet
	srv.mu.Unlock()
	if got == nil || string(got.Secret) != "from-file" {
		t.Errorf("secret = %v, want from-file", got)
	}
}

func TestVaultSet_ExistingItem_NoForce(t *testing.T) {
	srv := &vaultSetTestServer{keyExistsResp: true}
	socketPath := startVaultSetDaemon(t, srv)
	t.Setenv("LOCKSMITH_SOCKET", socketPath)
	t.Setenv("LOCKSMITH_SESSION", "ls_test")

	root := cli.NewRootCmd()
	root.SetArgs([]string{"vault", "set", "github-token"})
	root.SetIn(strings.NewReader("v\n"))

	err := root.Execute()
	if err == nil || !strings.Contains(err.Error(), "--force") {
		t.Errorf("err = %v, want error mentioning --force", err)
	}
	srv.mu.Lock()
	defer srv.mu.Unlock()
	if srv.lastSet != nil {
		t.Error("SetSecret should not have been called")
	}
}

func TestVaultSet_ExistingItem_WithForce(t *testing.T) {
	srv := &vaultSetTestServer{keyExistsResp: true}
	socketPath := startVaultSetDaemon(t, srv)
	t.Setenv("LOCKSMITH_SOCKET", socketPath)
	t.Setenv("LOCKSMITH_SESSION", "ls_test")

	root := cli.NewRootCmd()
	root.SetArgs([]string{"vault", "set", "github-token", "--force"})
	root.SetIn(strings.NewReader("v\n"))

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	srv.mu.Lock()
	got := srv.lastSet
	srv.mu.Unlock()
	if got == nil || !got.Force {
		t.Errorf("Force flag not propagated, got %v", got)
	}
}

func TestVaultSet_UnsupportedVaultType(t *testing.T) {
	srv := &vaultSetTestServer{setSecretErr: status.Errorf(codes.Unimplemented, "read-only")}
	socketPath := startVaultSetDaemon(t, srv)
	t.Setenv("LOCKSMITH_SOCKET", socketPath)
	t.Setenv("LOCKSMITH_SESSION", "ls_test")

	root := cli.NewRootCmd()
	root.SetArgs([]string{"vault", "set", "github-token"})
	root.SetIn(strings.NewReader("v\n"))

	err := root.Execute()
	if err == nil || !strings.Contains(err.Error(), "does not support") {
		t.Errorf("err = %v, want friendly message about unsupported write", err)
	}
}

func TestVaultSet_EmptySecret(t *testing.T) {
	srv := &vaultSetTestServer{}
	socketPath := startVaultSetDaemon(t, srv)
	t.Setenv("LOCKSMITH_SOCKET", socketPath)
	t.Setenv("LOCKSMITH_SESSION", "ls_test")

	root := cli.NewRootCmd()
	root.SetArgs([]string{"vault", "set", "github-token"})
	root.SetIn(strings.NewReader(""))

	if err := root.Execute(); err == nil {
		t.Fatal("expected error for empty secret")
	}
}

func TestVaultSet_KeyExistsUnimplemented_FallsThrough(t *testing.T) {
	// If KeyExists is Unimplemented (e.g. a third-party plugin without
	// the probe), the strict check is skipped; SetSecret still runs.
	srv := &vaultSetTestServer{keyExistsErr: status.Errorf(codes.Unimplemented, "no probe")}
	socketPath := startVaultSetDaemon(t, srv)
	t.Setenv("LOCKSMITH_SOCKET", socketPath)
	t.Setenv("LOCKSMITH_SESSION", "ls_test")

	root := cli.NewRootCmd()
	root.SetArgs([]string{"vault", "set", "github-token"})
	root.SetIn(strings.NewReader("v\n"))

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	srv.mu.Lock()
	defer srv.mu.Unlock()
	if srv.lastSet == nil {
		t.Error("SetSecret should have been called despite KeyExists Unimplemented")
	}
}
