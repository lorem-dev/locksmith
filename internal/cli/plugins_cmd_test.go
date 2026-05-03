package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lorem-dev/locksmith/internal/bundled"
)

func TestPluginsUpdate_NoVaultsConfigured(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	cfgDir := filepath.Join(home, ".config", "locksmith")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfgDir, "config.yaml"), []byte("vaults: {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	stdout := captureStdout(t, func() {
		if err := runPluginsUpdate(false, false); err != nil {
			t.Fatalf("runPluginsUpdate: %v", err)
		}
	})
	if !strings.Contains(stdout, "No vaults configured") {
		t.Errorf("stdout = %q, want hint about configuring vaults", stdout)
	}
}

func TestPluginsUpdate_EmptyBundle(t *testing.T) {
	// This test only passes in builds with the placeholder (empty) bundle.
	// If a real bundle is present, the bundle opens successfully and the
	// "no bundled plugins" path is never reached - skip in that case.
	if _, openErr := bundled.OpenBundle(); openErr == nil {
		t.Skip("real bundle present; skipping empty-bundle path test")
	}

	home := t.TempDir()
	t.Setenv("HOME", home)
	cfgDir := filepath.Join(home, ".config", "locksmith")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfgDir, "config.yaml"),
		[]byte("vaults:\n  myvault:\n    type: gopass\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	stdout := captureStdout(t, func() {
		if err := runPluginsUpdate(false, false); err != nil {
			t.Fatalf("runPluginsUpdate: %v", err)
		}
	})
	if !strings.Contains(stdout, "no bundled plugins") {
		t.Errorf("stdout = %q, want 'no bundled plugins'", stdout)
	}
}

// captureStdout returns whatever was written to os.Stdout during fn.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	defer func() { os.Stdout = old }()
	done := make(chan struct{})
	var buf bytes.Buffer
	go func() {
		_, _ = buf.ReadFrom(r)
		close(done)
	}()
	fn()
	w.Close()
	<-done
	return buf.String()
}
