package sdk_test

import (
	"os"
	"path/filepath"
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
