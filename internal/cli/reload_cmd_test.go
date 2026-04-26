package cli_test

import (
	"bytes"
	"context"
	"net"
	"path/filepath"
	"strings"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	locksmithv1 "github.com/lorem-dev/locksmith/gen/proto/locksmith/v1"
	"github.com/lorem-dev/locksmith/internal/cli"
)

// reloadTestServer is a minimal gRPC server stub for reload tests.
type reloadTestServer struct {
	locksmithv1.UnimplementedLocksmithServiceServer

	err error
}

func (r *reloadTestServer) ReloadConfig(
	_ context.Context,
	_ *locksmithv1.ReloadConfigRequest,
) (*locksmithv1.ReloadConfigResponse, error) {
	if r.err != nil {
		return nil, r.err
	}
	return &locksmithv1.ReloadConfigResponse{Message: "config reloaded"}, nil
}

func startReloadTestDaemon(t *testing.T, srv locksmithv1.LocksmithServiceServer) string {
	t.Helper()
	socketPath := filepath.Join(t.TempDir(), "test.sock")
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

func TestReloadCmd_Success(t *testing.T) {
	socketPath := startReloadTestDaemon(t, &reloadTestServer{})
	t.Setenv("LOCKSMITH_SOCKET", socketPath)

	root := cli.NewRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetArgs([]string{"reload"})

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute() error: %v", err)
	}
	if !strings.Contains(buf.String(), "config reloaded") {
		t.Errorf("output = %q, want to contain 'config reloaded'", buf.String())
	}
}

func TestReloadCmd_DaemonError(t *testing.T) {
	reloadErr := status.Error(codes.Internal, "bad config")
	socketPath := startReloadTestDaemon(t, &reloadTestServer{err: reloadErr})
	t.Setenv("LOCKSMITH_SOCKET", socketPath)

	root := cli.NewRootCmd()
	root.SetArgs([]string{"reload"})

	err := root.Execute()
	if err == nil {
		t.Fatal("Execute() expected error when daemon returns Internal")
	}
}
