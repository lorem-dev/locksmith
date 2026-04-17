package initflow_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lorem-dev/locksmith/internal/initflow"
)

// mockPinentryPrompter implements ConfigPinentryPrompter for tests.
type mockPinentryPrompter struct {
	accept bool
	err    error
}

func (m *mockPinentryPrompter) GPGPinentry(_ string) (bool, error) {
	return m.accept, m.err
}

// fakePinentryBin creates a fake locksmith-pinentry executable in a temp dir,
// prepends that dir to PATH, and returns the binary path.
func fakePinentryBin(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	bin := filepath.Join(dir, "locksmith-pinentry")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatalf("creating fake locksmith-pinentry: %v", err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return bin
}

func TestRunConfigPinentry_NotFound(t *testing.T) {
	// Empty PATH - locksmith-pinentry cannot be found.
	t.Setenv("PATH", t.TempDir())
	home := t.TempDir()
	t.Setenv("HOME", home)

	_, err := initflow.RunConfigPinentry(initflow.ConfigPinentryOptions{Auto: true})
	if err == nil {
		t.Fatal("expected error when locksmith-pinentry not found")
	}
	if !strings.Contains(err.Error(), "locksmith-pinentry not found") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestRunConfigPinentry_Auto_Configures(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	binPath := fakePinentryBin(t)

	result, err := initflow.RunConfigPinentry(initflow.ConfigPinentryOptions{Auto: true})
	if err != nil {
		t.Fatalf("RunConfigPinentry() error: %v", err)
	}
	if !result.Configured {
		t.Error("expected Configured = true in auto mode")
	}
	if result.PinentryPath != binPath {
		t.Errorf("PinentryPath = %q, want %q", result.PinentryPath, binPath)
	}
	// Verify gpg-agent.conf was written.
	confPath := filepath.Join(home, ".gnupg", "gpg-agent.conf")
	data, readErr := os.ReadFile(confPath)
	if readErr != nil {
		t.Fatalf("gpg-agent.conf not created: %v", readErr)
	}
	if !strings.Contains(string(data), "pinentry-program "+binPath) {
		t.Errorf("gpg-agent.conf does not contain expected line:\n%s", data)
	}
}

func TestRunConfigPinentry_Interactive_Accepted(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	fakePinentryBin(t)

	mp := &mockPinentryPrompter{accept: true}
	result, err := initflow.RunConfigPinentry(initflow.ConfigPinentryOptions{Prompter: mp})
	if err != nil {
		t.Fatalf("RunConfigPinentry() error: %v", err)
	}
	if !result.Configured {
		t.Error("expected Configured = true when user accepts")
	}
}

func TestRunConfigPinentry_Interactive_Declined(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	fakePinentryBin(t)

	mp := &mockPinentryPrompter{accept: false}
	result, err := initflow.RunConfigPinentry(initflow.ConfigPinentryOptions{Prompter: mp})
	if err != nil {
		t.Fatalf("RunConfigPinentry() error: %v", err)
	}
	if result.Configured {
		t.Error("expected Configured = false when user declines")
	}
	// gpg-agent.conf must not be created.
	confPath := filepath.Join(home, ".gnupg", "gpg-agent.conf")
	if _, statErr := os.Stat(confPath); statErr == nil {
		t.Error("gpg-agent.conf should not be created when user declines")
	}
}

func TestRunConfigPinentry_ReplacesExisting(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	binPath := fakePinentryBin(t)

	// Pre-populate gpg-agent.conf with an existing pinentry-program.
	gnupgDir := filepath.Join(home, ".gnupg")
	os.MkdirAll(gnupgDir, 0o700)
	os.WriteFile(filepath.Join(gnupgDir, "gpg-agent.conf"),
		[]byte("pinentry-program /opt/homebrew/bin/pinentry-mac\n"), 0o600)

	mp := &mockPinentryPrompter{accept: true}
	result, err := initflow.RunConfigPinentry(initflow.ConfigPinentryOptions{Prompter: mp})
	if err != nil {
		t.Fatalf("RunConfigPinentry() error: %v", err)
	}
	if result.Replaced != "/opt/homebrew/bin/pinentry-mac" {
		t.Errorf("Replaced = %q, want %q", result.Replaced, "/opt/homebrew/bin/pinentry-mac")
	}
	data, _ := os.ReadFile(filepath.Join(gnupgDir, "gpg-agent.conf"))
	content := string(data)
	if !strings.Contains(content, "#pinentry-program /opt/homebrew/bin/pinentry-mac") {
		t.Errorf("old line should be commented out:\n%s", content)
	}
	if !strings.Contains(content, "pinentry-program "+binPath) {
		t.Errorf("new line missing:\n%s", content)
	}
}

func TestRunConfigPinentry_PromptError(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	fakePinentryBin(t)

	wantErr := errors.New("prompt failed")
	mp := &mockPinentryPrompter{err: wantErr}
	_, err := initflow.RunConfigPinentry(initflow.ConfigPinentryOptions{Prompter: mp})
	if !errors.Is(err, wantErr) {
		t.Errorf("error = %v, want %v", err, wantErr)
	}
}
