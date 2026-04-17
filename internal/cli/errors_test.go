package cli_test

import (
	"errors"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	locksmithv1 "github.com/lorem-dev/locksmith/gen/proto/locksmith/v1"
	"github.com/lorem-dev/locksmith/internal/cli"
)

func TestFormatError_GRPCNotFound(t *testing.T) {
	err := status.Error(codes.NotFound, "keychain: item not found")
	msg, hint := cli.FormatErrorParts(err)
	if msg != "keychain: item not found" {
		t.Errorf("msg = %q, want %q", msg, "keychain: item not found")
	}
	if hint == "" {
		t.Error("expected non-empty hint for NotFound")
	}
}

func TestFormatError_NonGRPC(t *testing.T) {
	err := errors.New("something unexpected")
	msg, hint := cli.FormatErrorParts(err)
	if msg != "something unexpected" {
		t.Errorf("msg = %q, want %q", msg, "something unexpected")
	}
	if hint != "" {
		t.Errorf("hint should be empty for non-gRPC error, got %q", hint)
	}
}

func TestFormatError_GRPCWithHints(t *testing.T) {
	tests := []struct {
		code     codes.Code
		wantHint bool
	}{
		{codes.PermissionDenied, true},
		{codes.Unauthenticated, true},
		{codes.Unavailable, true},
		{codes.InvalidArgument, true},
		{codes.DeadlineExceeded, true},
		{codes.Unimplemented, true},
		{codes.Internal, true},
		{codes.Unknown, true},
	}
	for _, tt := range tests {
		err := status.Error(tt.code, "test error")
		msg, hint := cli.FormatErrorParts(err)
		if msg != "test error" {
			t.Errorf("code %v: msg = %q, want %q", tt.code, msg, "test error")
		}
		if tt.wantHint && hint == "" {
			t.Errorf("code %v: expected non-empty hint", tt.code)
		}
	}
}

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
