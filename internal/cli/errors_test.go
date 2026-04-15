package cli_test

import (
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	locksmithv1 "github.com/lorem-dev/locksmith/gen/proto/locksmith/v1"
	"github.com/lorem-dev/locksmith/internal/cli"
)

// Tests for the error paths in command RunE functions.

func TestServeCmd_ConfigError(t *testing.T) {
	root := cli.NewRootCmd()
	root.SetArgs([]string{"--config", "/nonexistent/path/config.yaml", "serve"})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for missing config")
	}
}

func TestSessionStartCmd_DaemonError(t *testing.T) {
	srv := &mockServer{
		sessionStartErr: status.Error(codes.Internal, "internal error"),
	}
	socketPath, cleanup := startMockDaemon(t, srv)
	defer cleanup()

	err := runWithSocket(t, socketPath, "session", "start")
	if err == nil {
		t.Fatal("expected error when daemon returns error")
	}
}

func TestSessionEndCmd_DaemonError(t *testing.T) {
	srv := &mockServer{
		sessionEndErr: status.Error(codes.Internal, "internal error"),
	}
	socketPath, cleanup := startMockDaemon(t, srv)
	defer cleanup()

	t.Setenv("LOCKSMITH_SESSION", "test-session")
	err := runWithSocket(t, socketPath, "session", "end")
	if err == nil {
		t.Fatal("expected error when daemon returns error")
	}
}

func TestSessionListCmd_DaemonError(t *testing.T) {
	srv := &mockServer{
		sessionListErr: status.Error(codes.Internal, "internal error"),
	}
	socketPath, cleanup := startMockDaemon(t, srv)
	defer cleanup()

	err := runWithSocket(t, socketPath, "session", "list")
	if err == nil {
		t.Fatal("expected error when daemon returns error")
	}
}

func TestVaultListCmd_DaemonError(t *testing.T) {
	srv := &mockServer{
		vaultListErr: status.Error(codes.Internal, "internal error"),
	}
	socketPath, cleanup := startMockDaemon(t, srv)
	defer cleanup()

	err := runWithSocket(t, socketPath, "vault", "list")
	if err == nil {
		t.Fatal("expected error when daemon returns error")
	}
}

func TestVaultHealthCmd_DaemonError(t *testing.T) {
	srv := &mockServer{
		vaultHealthErr: status.Error(codes.Internal, "internal error"),
	}
	socketPath, cleanup := startMockDaemon(t, srv)
	defer cleanup()

	err := runWithSocket(t, socketPath, "vault", "health")
	if err == nil {
		t.Fatal("expected error when daemon returns error")
	}
}

func TestGetCmd_DaemonRPCError(t *testing.T) {
	srv := &mockServer{
		getSecretErr: status.Error(codes.NotFound, "secret not found"),
	}
	socketPath, cleanup := startMockDaemon(t, srv)
	defer cleanup()

	t.Setenv("LOCKSMITH_SESSION", "test-session-123")
	err := runWithSocket(t, socketPath, "get", "--key", "mykey")
	if err == nil {
		t.Fatal("expected error when daemon returns error")
	}
}

func TestGetCmd_VaultAndPath(t *testing.T) {
	srv := &mockServer{
		getSecretResp: &locksmithv1.GetSecretResponse{Secret: []byte("val")},
	}
	socketPath, cleanup := startMockDaemon(t, srv)
	defer cleanup()

	t.Setenv("LOCKSMITH_SESSION", "test-session-123")
	if err := runWithSocket(t, socketPath, "get", "--vault", "keychain", "--path", "/my/path"); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}
