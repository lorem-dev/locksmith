package log_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	sdklog "github.com/lorem-dev/locksmith/sdk/log"
)

func TestNewLogWriter_Stdout(t *testing.T) {
	w, err := sdklog.NewLogWriter(sdklog.LogConfig{Level: "info", Format: "text"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if w != os.Stdout {
		t.Fatalf("expected os.Stdout when File is empty, got %T", w)
	}
}

func TestNewLogWriter_SetsDebugFalse(t *testing.T) {
	_, _ = sdklog.NewLogWriter(sdklog.LogConfig{Level: "info"})
	if sdklog.IsDebug() {
		t.Error("IsDebug should be false after NewLogWriter with level=info")
	}
}

func TestNewLogWriter_SetsDebugTrue(t *testing.T) {
	_, _ = sdklog.NewLogWriter(sdklog.LogConfig{Level: "debug"})
	if !sdklog.IsDebug() {
		t.Error("IsDebug should be true after NewLogWriter with level=debug")
	}
	// Reset
	_, _ = sdklog.NewLogWriter(sdklog.LogConfig{Level: "info"})
}

func TestNewLogWriter_CreatesFileWriter(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "logs", "daemon.log")

	w, err := sdklog.NewLogWriter(sdklog.LogConfig{Level: "info", File: logPath})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if w == os.Stdout {
		t.Fatal("expected lumberjack writer, got os.Stdout")
	}
	if _, err := os.Stat(filepath.Dir(logPath)); err != nil {
		t.Fatalf("log directory was not created: %v", err)
	}
}

func TestNewLogWriter_CreatesParentDirectory(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "a", "b", "c", "daemon.log")

	_, err := sdklog.NewLogWriter(sdklog.LogConfig{Level: "info", File: logPath})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := os.Stat(filepath.Dir(logPath)); err != nil {
		t.Fatalf("nested log directory was not created: %v", err)
	}
}

func TestNewLogWriter_ExpandsTilde(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("no home directory available")
	}
	dir := t.TempDir()
	rel, err := filepath.Rel(home, filepath.Join(dir, "daemon.log"))
	if err != nil {
		t.Fatalf("failed to build relative path: %v", err)
	}
	tilded := "~/" + rel

	w, err := sdklog.NewLogWriter(sdklog.LogConfig{Level: "info", File: tilded})
	if err != nil {
		t.Fatalf("unexpected error with tilde path: %v", err)
	}
	if w == os.Stdout {
		t.Fatal("expected file writer, got os.Stdout")
	}
}

func TestExpandTilde_NoPrefix(t *testing.T) {
	path := "/absolute/path/file.log"
	got := sdklog.ExpandTilde(path, "/home/user")
	if got != path {
		t.Errorf("expected %q unchanged, got %q", path, got)
	}
}

func TestExpandTilde_EmptyHome(t *testing.T) {
	path := "~/logs/daemon.log"
	got := sdklog.ExpandTilde(path, "")
	if got != path {
		t.Errorf("expected path returned unchanged when home is empty, got %q", got)
	}
}

func TestExpandTilde_HappyPath(t *testing.T) {
	got := sdklog.ExpandTilde("~/logs/daemon.log", "/home/user")
	if !strings.HasPrefix(got, "/home/user") {
		t.Errorf("expected path under /home/user, got %q", got)
	}
}
