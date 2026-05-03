// internal/bundled/paths_test.go
package bundled

import (
	"path/filepath"
	"testing"
)

func TestPluginsDir(t *testing.T) {
	t.Setenv("HOME", "/fake/home")
	got, err := PluginsDir()
	if err != nil {
		t.Fatalf("PluginsDir: %v", err)
	}
	want := filepath.Join("/fake/home", ".config", "locksmith", "plugins")
	if got != want {
		t.Errorf("PluginsDir() = %q, want %q", got, want)
	}
}

func TestBinDir(t *testing.T) {
	t.Setenv("HOME", "/fake/home")
	got, err := BinDir()
	if err != nil {
		t.Fatalf("BinDir: %v", err)
	}
	want := filepath.Join("/fake/home", ".config", "locksmith", "bin")
	if got != want {
		t.Errorf("BinDir() = %q, want %q", got, want)
	}
}

func TestPinentryPath(t *testing.T) {
	t.Setenv("HOME", "/fake/home")
	got, err := PinentryPath()
	if err != nil {
		t.Fatalf("PinentryPath: %v", err)
	}
	want := filepath.Join("/fake/home", ".config", "locksmith", "bin", "locksmith-pinentry")
	if got != want {
		t.Errorf("PinentryPath() = %q, want %q", got, want)
	}
}
