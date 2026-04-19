package sdk_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lorem-dev/locksmith/sdk"
)

func TestNewLogWriter_Stdout(t *testing.T) {
	w, err := sdk.NewLogWriter(sdk.LogConfig{Level: "info", Format: "text"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if w != os.Stdout {
		t.Fatalf("expected os.Stdout when File is empty, got %T", w)
	}
}

func TestNewLogWriter_SetsDebugFalse(t *testing.T) {
	_, _ = sdk.NewLogWriter(sdk.LogConfig{Level: "info"})
	if sdk.IsDebug() {
		t.Error("IsDebug should be false after NewLogWriter with level=info")
	}
}

func TestNewLogWriter_SetsDebugTrue(t *testing.T) {
	_, _ = sdk.NewLogWriter(sdk.LogConfig{Level: "debug"})
	if !sdk.IsDebug() {
		t.Error("IsDebug should be true after NewLogWriter with level=debug")
	}
	// Reset
	_, _ = sdk.NewLogWriter(sdk.LogConfig{Level: "info"})
}

func TestNewLogWriter_CreatesFileWriter(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "logs", "daemon.log")

	w, err := sdk.NewLogWriter(sdk.LogConfig{Level: "info", File: logPath})
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

	_, err := sdk.NewLogWriter(sdk.LogConfig{Level: "info", File: logPath})
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
	// Build a path relative to the real home dir so the tilde expands correctly.
	rel, err := filepath.Rel(home, filepath.Join(dir, "daemon.log"))
	if err != nil {
		t.Fatalf("failed to build relative path: %v", err)
	}
	tilded := "~/" + rel

	w, err := sdk.NewLogWriter(sdk.LogConfig{Level: "info", File: tilded})
	if err != nil {
		t.Fatalf("unexpected error with tilde path: %v", err)
	}
	if w == os.Stdout {
		t.Fatal("expected file writer, got os.Stdout")
	}
}

func TestExpandTilde_NoPrefix(t *testing.T) {
	path := "/absolute/path/file.log"
	got := sdk.ExpandTilde(path, "/home/user")
	if got != path {
		t.Errorf("expected %q unchanged, got %q", path, got)
	}
}

func TestExpandTilde_EmptyHome(t *testing.T) {
	path := "~/logs/daemon.log"
	got := sdk.ExpandTilde(path, "")
	if got != path {
		t.Errorf("expected path returned unchanged when home is empty, got %q", got)
	}
}

func TestExpandTilde_HappyPath(t *testing.T) {
	got := sdk.ExpandTilde("~/logs/daemon.log", "/home/user")
	if !strings.HasPrefix(got, "/home/user") {
		t.Errorf("expected path under /home/user, got %q", got)
	}
}
