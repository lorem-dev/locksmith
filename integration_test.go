//go:build integration

package locksmith_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/lorem-dev/locksmith/internal/config"
	"github.com/lorem-dev/locksmith/internal/daemon"
	locksmithv1 "github.com/lorem-dev/locksmith/gen/proto/locksmith/v1"
	ilog "github.com/lorem-dev/locksmith/internal/log"
)

func TestMain(m *testing.M) {
	// Initialize the global logger before any test runs so daemon code that
	// calls log.Info/Warn/etc. does not panic with a nil logger.
	ilog.Init(ilog.Config{Level: "error", Format: "text"})
	os.Exit(m.Run())
}

func TestFullSessionLifecycle(t *testing.T) {
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "test.sock")

	os.WriteFile(filepath.Join(tmpDir, "config.yaml"), []byte(`
defaults:
  session_ttl: 1h
  socket_path: `+socketPath+`
logging:
  level: error
  format: text
vaults:
  keychain:
    type: keychain
keys:
  test-key:
    vault: keychain
    path: test-secret
`), 0o644)

	cfg, err := config.Load(filepath.Join(tmpDir, "config.yaml"))
	if err != nil {
		t.Fatalf("Load config: %v", err)
	}

	d := daemon.New(cfg)
	go func() { d.Start() }()
	defer d.Stop()

	// Wait for socket to appear
	for i := 0; i < 50; i++ {
		if _, err := os.Stat(socketPath); err == nil {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	conn, err := grpc.NewClient("unix://"+socketPath,
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("dial daemon: %v", err)
	}
	defer conn.Close()

	client := locksmithv1.NewLocksmithServiceClient(conn)
	ctx := context.Background()

	// Start session
	start, err := client.SessionStart(ctx, &locksmithv1.SessionStartRequest{Ttl: "30m"})
	if err != nil {
		t.Fatalf("SessionStart: %v", err)
	}
	if start.SessionId == "" {
		t.Fatal("empty session ID")
	}

	// List — expect 1
	list, _ := client.SessionList(ctx, &locksmithv1.SessionListRequest{})
	if len(list.Sessions) != 1 {
		t.Fatalf("sessions = %d, want 1", len(list.Sessions))
	}

	// End session
	if _, err := client.SessionEnd(ctx, &locksmithv1.SessionEndRequest{SessionId: start.SessionId}); err != nil {
		t.Fatalf("SessionEnd: %v", err)
	}

	// List — expect 0
	list, _ = client.SessionList(ctx, &locksmithv1.SessionListRequest{})
	if len(list.Sessions) != 0 {
		t.Fatalf("sessions after end = %d, want 0", len(list.Sessions))
	}
}
