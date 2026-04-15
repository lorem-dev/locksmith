package cli_test

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"testing"

	"google.golang.org/grpc"

	locksmithv1 "github.com/lorem-dev/locksmith/gen/proto/locksmith/v1"
	"github.com/lorem-dev/locksmith/internal/cli"
)

// mockServer implements LocksmithServiceServer for testing.
type mockServer struct {
	locksmithv1.UnimplementedLocksmithServiceServer

	getSecretResp    *locksmithv1.GetSecretResponse
	getSecretErr     error
	sessionStartResp *locksmithv1.SessionStartResponse
	sessionStartErr  error
	sessionEndErr    error
	sessionListResp  *locksmithv1.SessionListResponse
	sessionListErr   error
	vaultListResp    *locksmithv1.VaultListResponse
	vaultListErr     error
	vaultHealthResp  *locksmithv1.VaultHealthResponse
	vaultHealthErr   error
}

func (m *mockServer) GetSecret(_ context.Context, _ *locksmithv1.GetSecretRequest) (*locksmithv1.GetSecretResponse, error) {
	if m.getSecretErr != nil {
		return nil, m.getSecretErr
	}
	if m.getSecretResp != nil {
		return m.getSecretResp, nil
	}
	return &locksmithv1.GetSecretResponse{Secret: []byte("mysecret")}, nil
}

func (m *mockServer) SessionStart(_ context.Context, _ *locksmithv1.SessionStartRequest) (*locksmithv1.SessionStartResponse, error) {
	if m.sessionStartErr != nil {
		return nil, m.sessionStartErr
	}
	if m.sessionStartResp != nil {
		return m.sessionStartResp, nil
	}
	return &locksmithv1.SessionStartResponse{
		SessionId: "test-session-id",
		ExpiresAt: "2099-01-01T00:00:00Z",
	}, nil
}

func (m *mockServer) SessionEnd(_ context.Context, _ *locksmithv1.SessionEndRequest) (*locksmithv1.SessionEndResponse, error) {
	if m.sessionEndErr != nil {
		return nil, m.sessionEndErr
	}
	return &locksmithv1.SessionEndResponse{}, nil
}

func (m *mockServer) SessionList(_ context.Context, _ *locksmithv1.SessionListRequest) (*locksmithv1.SessionListResponse, error) {
	if m.sessionListErr != nil {
		return nil, m.sessionListErr
	}
	if m.sessionListResp != nil {
		return m.sessionListResp, nil
	}
	return &locksmithv1.SessionListResponse{}, nil
}

func (m *mockServer) VaultList(_ context.Context, _ *locksmithv1.VaultListRequest) (*locksmithv1.VaultListResponse, error) {
	if m.vaultListErr != nil {
		return nil, m.vaultListErr
	}
	if m.vaultListResp != nil {
		return m.vaultListResp, nil
	}
	return &locksmithv1.VaultListResponse{}, nil
}

func (m *mockServer) VaultHealth(_ context.Context, _ *locksmithv1.VaultHealthRequest) (*locksmithv1.VaultHealthResponse, error) {
	if m.vaultHealthErr != nil {
		return nil, m.vaultHealthErr
	}
	if m.vaultHealthResp != nil {
		return m.vaultHealthResp, nil
	}
	return &locksmithv1.VaultHealthResponse{}, nil
}

// startMockDaemon starts a gRPC server on a temp Unix socket.
// Returns the socket path and a cleanup function.
// Uses os.MkdirTemp with a short base path to stay within macOS's 104-char
// Unix socket path limit.
func startMockDaemon(t *testing.T, srv *mockServer) (socketPath string, cleanup func()) {
	t.Helper()
	// Use /tmp directly to keep paths short (macOS 104-char Unix socket limit).
	dir, err := os.MkdirTemp("/tmp", "lks")
	if err != nil {
		t.Fatalf("mkdirtemp: %v", err)
	}
	socketPath = filepath.Join(dir, "l.sock")

	lis, lisErr := net.Listen("unix", socketPath)
	if lisErr != nil {
		os.RemoveAll(dir)
		t.Fatalf("listen unix %s: %v", socketPath, lisErr)
	}

	grpcSrv := grpc.NewServer()
	locksmithv1.RegisterLocksmithServiceServer(grpcSrv, srv)

	go func() {
		_ = grpcSrv.Serve(lis)
	}()

	return socketPath, func() {
		grpcSrv.Stop()
		os.RemoveAll(dir)
	}
}

// runWithSocket runs a locksmith command with LOCKSMITH_SOCKET pointing to socketPath.
func runWithSocket(t *testing.T, socketPath string, args ...string) error {
	t.Helper()
	t.Setenv("LOCKSMITH_SOCKET", socketPath)
	root := cli.NewRootCmd()
	root.SetArgs(args)
	return root.Execute()
}

func TestDialDaemon_ValidSocket(t *testing.T) {
	srv := &mockServer{}
	socketPath, cleanup := startMockDaemon(t, srv)
	defer cleanup()

	if err := runWithSocket(t, socketPath, "session", "list"); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestGetCmd_WithSession(t *testing.T) {
	srv := &mockServer{
		getSecretResp: &locksmithv1.GetSecretResponse{Secret: []byte("mypassword")},
	}
	socketPath, cleanup := startMockDaemon(t, srv)
	defer cleanup()

	t.Setenv("LOCKSMITH_SESSION", "test-session-123")
	if err := runWithSocket(t, socketPath, "get", "--key", "mykey"); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestSessionStartCmd_Success(t *testing.T) {
	srv := &mockServer{
		sessionStartResp: &locksmithv1.SessionStartResponse{
			SessionId: "sess-abc",
			ExpiresAt: "2099-12-31T00:00:00Z",
		},
	}
	socketPath, cleanup := startMockDaemon(t, srv)
	defer cleanup()

	if err := runWithSocket(t, socketPath, "session", "start", "--ttl", "1h"); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestSessionStartCmd_WithKeys(t *testing.T) {
	srv := &mockServer{}
	socketPath, cleanup := startMockDaemon(t, srv)
	defer cleanup()

	if err := runWithSocket(t, socketPath, "session", "start", "--keys", "key1,key2"); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestSessionEndCmd_WithFlag(t *testing.T) {
	srv := &mockServer{}
	socketPath, cleanup := startMockDaemon(t, srv)
	defer cleanup()

	t.Setenv("LOCKSMITH_SESSION", "")
	if err := runWithSocket(t, socketPath, "session", "end", "--session", "explicit-session-id"); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestSessionEndCmd_WithEnv(t *testing.T) {
	srv := &mockServer{}
	socketPath, cleanup := startMockDaemon(t, srv)
	defer cleanup()

	t.Setenv("LOCKSMITH_SESSION", "env-session-id")
	if err := runWithSocket(t, socketPath, "session", "end"); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestSessionListCmd_Empty(t *testing.T) {
	srv := &mockServer{
		sessionListResp: &locksmithv1.SessionListResponse{},
	}
	socketPath, cleanup := startMockDaemon(t, srv)
	defer cleanup()

	if err := runWithSocket(t, socketPath, "session", "list"); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestSessionListCmd_WithSessions(t *testing.T) {
	srv := &mockServer{
		sessionListResp: &locksmithv1.SessionListResponse{
			Sessions: []*locksmithv1.SessionInfo{
				{SessionId: "s1", ExpiresAt: "2099-01-01T00:00:00Z"},
			},
		},
	}
	socketPath, cleanup := startMockDaemon(t, srv)
	defer cleanup()

	if err := runWithSocket(t, socketPath, "session", "list"); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestVaultListCmd_Empty(t *testing.T) {
	srv := &mockServer{
		vaultListResp: &locksmithv1.VaultListResponse{},
	}
	socketPath, cleanup := startMockDaemon(t, srv)
	defer cleanup()

	if err := runWithSocket(t, socketPath, "vault", "list"); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestVaultListCmd_WithVaults(t *testing.T) {
	srv := &mockServer{
		vaultListResp: &locksmithv1.VaultListResponse{
			Vaults: []*locksmithv1.VaultInfo{
				{Name: "keychain", Type: "keychain"},
			},
		},
	}
	socketPath, cleanup := startMockDaemon(t, srv)
	defer cleanup()

	if err := runWithSocket(t, socketPath, "vault", "list"); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestVaultHealthCmd_WithResults(t *testing.T) {
	srv := &mockServer{
		vaultHealthResp: &locksmithv1.VaultHealthResponse{
			Vaults: []*locksmithv1.VaultHealthInfo{
				{Name: "keychain", Available: true, Message: ""},
				{Name: "gopass", Available: false, Message: "not installed"},
			},
		},
	}
	socketPath, cleanup := startMockDaemon(t, srv)
	defer cleanup()

	if err := runWithSocket(t, socketPath, "vault", "health"); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestVaultHealthCmd_Empty(t *testing.T) {
	srv := &mockServer{
		vaultHealthResp: &locksmithv1.VaultHealthResponse{},
	}
	socketPath, cleanup := startMockDaemon(t, srv)
	defer cleanup()

	if err := runWithSocket(t, socketPath, "vault", "health"); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestConfigCheck_ValidConfig(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	cfgContent := `defaults:
  session_ttl: 1h
vaults:
  keychain:
    type: keychain
keys:
  mykey:
    vault: keychain
    path: /mypath
`
	if err := os.WriteFile(cfgPath, []byte(cfgContent), 0600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	root := cli.NewRootCmd()
	root.SetArgs([]string{"--config", cfgPath, "config", "check"})
	if err := root.Execute(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}
