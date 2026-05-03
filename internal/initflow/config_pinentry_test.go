package initflow_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lorem-dev/locksmith/internal/bundled"
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

// fakePinentryAt creates a fake locksmith-pinentry executable at the canonical
// bundled location under home, sets HOME to home, and returns the binary path.
func fakePinentryAt(t *testing.T, home string) string {
	t.Helper()
	binDir := filepath.Join(home, ".config", "locksmith", "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	bin := filepath.Join(binDir, "locksmith-pinentry")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write fake pinentry: %v", err)
	}
	return bin
}

func TestRunConfigPinentry_MissingPinentry_EmptyBundle(t *testing.T) {
	// This test only passes in builds with the placeholder (empty) bundle.
	// If a real bundle is present, the pinentry is extracted successfully and
	// no error is returned - skip in that case.
	if _, openErr := bundled.OpenBundle(); openErr == nil {
		t.Skip("real bundle present; skipping empty-bundle error test")
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	_, err := initflow.RunConfigPinentry(initflow.ConfigPinentryOptions{Auto: true})
	if err == nil || !strings.Contains(err.Error(), "bundle is empty") {
		t.Errorf("err = %v, want 'bundle is empty' error", err)
	}
}

func TestRunConfigPinentry_Auto_Configures(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	binPath := fakePinentryAt(t, home)

	result, err := initflow.RunConfigPinentry(initflow.ConfigPinentryOptions{Auto: true})
	if err != nil {
		t.Fatalf("RunConfigPinentry() error: %v", err)
	}
	if !result.Configured {
		t.Error("expected Configured = true in auto mode")
	}

	wantPath, pathErr := bundled.PinentryPath()
	if pathErr != nil {
		t.Fatalf("bundled.PinentryPath: %v", pathErr)
	}
	if result.PinentryPath != wantPath {
		t.Errorf("PinentryPath = %q, want %q", result.PinentryPath, wantPath)
	}
	_ = binPath

	// Verify gpg-agent.conf was written.
	confPath := filepath.Join(home, ".gnupg", "gpg-agent.conf")
	data, readErr := os.ReadFile(confPath)
	if readErr != nil {
		t.Fatalf("gpg-agent.conf not created: %v", readErr)
	}
	if !strings.Contains(string(data), "pinentry-program "+wantPath) {
		t.Errorf("gpg-agent.conf does not contain expected line:\n%s", data)
	}
}

func TestRunConfigPinentry_Interactive_Accepted(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	fakePinentryAt(t, home)

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
	fakePinentryAt(t, home)

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
	fakePinentryAt(t, home)

	wantPath, pathErr := bundled.PinentryPath()
	if pathErr != nil {
		t.Fatalf("bundled.PinentryPath: %v", pathErr)
	}

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
	if !strings.Contains(content, "pinentry-program "+wantPath) {
		t.Errorf("new line missing:\n%s", content)
	}
}

func TestRunConfigPinentry_PromptError(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	fakePinentryAt(t, home)

	wantErr := errors.New("prompt failed")
	mp := &mockPinentryPrompter{err: wantErr}
	_, err := initflow.RunConfigPinentry(initflow.ConfigPinentryOptions{Prompter: mp})
	if !errors.Is(err, wantErr) {
		t.Errorf("error = %v, want %v", err, wantErr)
	}
}
